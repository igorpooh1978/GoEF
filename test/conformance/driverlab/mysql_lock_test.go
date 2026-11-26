package driverlab

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"time"

	"github.com/go-sql-driver/mysql"
)

const mysqlContendedUpdate = "UPDATE goef_driver_parent SET code = ? WHERE id = ?"

type mysqlLockSession struct {
	connection uint64
	thread     uint64
}

func (s *scenario) checkMySQLLockWait() {
	s.t.Helper()

	for i, code := range []string{"first", "second"} {
		if _, err := s.writer.ExecContext(s.ctx, s.insertParentSQL(), i+1, code); err != nil {
			s.t.Fatal("could not seed MySQL lock fixture")
		}
	}
	var rollbackOnTimeout bool
	if err := s.observer.QueryRowContext(s.ctx, "SELECT @@global.innodb_rollback_on_timeout").
		Scan(&rollbackOnTimeout); err != nil || rollbackOnTimeout {
		s.t.Fatal("MySQL lock fixture requires statement rollback on timeout")
	}
	holder := s.mysqlLockIdentity(s.writer.QueryRowContext)
	waiter := s.mysqlLockIdentity(s.observer.QueryRowContext)
	if holder.connection == waiter.connection || holder.thread == waiter.thread {
		s.t.Fatal("MySQL lock fixture requires distinct physical sessions")
	}

	lock, err := s.writer.BeginTx(s.ctx, nil)
	if err != nil {
		s.t.Fatal("could not start MySQL holder transaction")
	}
	defer s.rollbackMySQLLockTx(lock)
	s.updateMySQLLockRow(lock, 1, "held")

	s.setMySQLLockWait(3)
	pending, err := s.observer.BeginTx(s.ctx, nil)
	if err != nil {
		s.t.Fatal("could not start MySQL waiter transaction")
	}
	defer s.rollbackMySQLLockTx(pending)
	s.updateMySQLLockRow(pending, 2, "pending")

	nativeErr := s.observeMySQLLockWait(s.ctx, lock, pending.ExecContext, holder, waiter)
	var native *mysql.MySQLError
	if s.ctx.Err() != nil || !errors.As(nativeErr, &native) || native.Number != 1205 ||
		string(native.SQLState[:]) != "HY000" {
		s.t.Fatal("MySQL lock wait did not return 1205/HY000 with live context")
	}
	if s.mysqlLockIdentity(pending.QueryRowContext) != waiter {
		s.t.Fatal("MySQL lock timeout changed the waiter session")
	}
	s.checkMySQLLockRow(pending.QueryRowContext, 2, "pending")
	if err := pending.Rollback(); err != nil {
		s.t.Fatal("could not roll back MySQL waiter transaction")
	}
	s.checkMySQLLockRow(s.observer.QueryRowContext, 2, "second")

	s.setMySQLLockWait(12)
	// A canceled client can lose its socket before the server stops waiting.
	// Keep this write in an uncommitted transaction so it cannot persist.
	expired, err := s.observer.BeginTx(s.ctx, nil)
	if err != nil {
		s.t.Fatal("could not start MySQL deadline transaction")
	}
	defer s.rollbackMySQLLockTx(expired)
	deadline, cancel := context.WithTimeout(s.ctx, 4*time.Second)
	defer cancel()
	deadlineErr := s.observeMySQLLockWait(deadline, lock, expired.ExecContext, holder, waiter)
	if err := expired.Rollback(); err != nil && !errors.Is(err, context.DeadlineExceeded) &&
		!errors.Is(err, driver.ErrBadConn) {
		s.t.Fatal("could not finalize MySQL deadline transaction")
	}
	if !errors.Is(deadline.Err(), context.DeadlineExceeded) ||
		!errors.Is(deadlineErr, context.DeadlineExceeded) || errors.As(deadlineErr, &native) {
		s.t.Fatal("MySQL lock wait did not preserve the caller deadline cause")
	}
	if err := s.observer.Raw(func(any) error { return driver.ErrBadConn }); !errors.Is(err, driver.ErrBadConn) &&
		!errors.Is(err, sql.ErrConnDone) {
		s.t.Fatal("could not discard MySQL waiter after caller deadline")
	}
	s.observer = s.lease()
	replacement := s.mysqlLockIdentity(s.observer.QueryRowContext)
	if replacement.connection == waiter.connection || replacement.thread == waiter.thread ||
		replacement.connection == holder.connection || replacement.thread == holder.thread {
		s.t.Fatal("MySQL deadline did not result in a fresh waiter session")
	}
	s.verifyMarker(s.ctx, s.observer.QueryContext)

	if err := lock.Rollback(); err != nil {
		s.t.Fatal("could not roll back MySQL holder transaction")
	}
	s.checkMySQLLockRow(s.observer.QueryRowContext, 1, "first")
	s.checkMySQLLockRow(s.observer.QueryRowContext, 2, "second")
	s.facts["lock_deadline_cause"] = "context.DeadlineExceeded"
	s.facts["lock_timeout_native"] = "1205/HY000"
	s.facts["lock_wait_observed"] = "both"
}

func (s *scenario) mysqlLockIdentity(query func(context.Context, string, ...any) *sql.Row) mysqlLockSession {
	s.t.Helper()

	var session mysqlLockSession
	if err := query(s.ctx, "SELECT CONNECTION_ID(), PS_CURRENT_THREAD_ID()").
		Scan(&session.connection, &session.thread); err != nil || session.connection == 0 || session.thread == 0 {
		s.t.Fatal("could not read MySQL lock session identity")
	}
	return session
}

func (s *scenario) setMySQLLockWait(seconds int) {
	s.t.Helper()

	if _, err := s.observer.ExecContext(s.ctx, "SET SESSION innodb_lock_wait_timeout = ?", seconds); err != nil {
		s.t.Fatal("could not set MySQL row lock wait timeout")
	}
	var actual int
	if err := s.observer.QueryRowContext(s.ctx, "SELECT @@session.innodb_lock_wait_timeout").
		Scan(&actual); err != nil || actual != seconds {
		s.t.Fatal("MySQL row lock timeout did not match the fixture setting")
	}
}

func (s *scenario) observeMySQLLockWait(ctx context.Context, holderTx *sql.Tx,
	exec func(context.Context, string, ...any) (sql.Result, error), holder, waiter mysqlLockSession,
) error {
	s.t.Helper()

	waitCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	completed := make(chan error, 1)
	go func() {
		_, err := exec(waitCtx, mysqlContendedUpdate, "blocked", 1)
		completed <- err
	}()
	probe := time.NewTicker(10 * time.Millisecond)
	defer probe.Stop()
	for {
		var observed bool
		err := holderTx.QueryRowContext(s.ctx, `
			SELECT EXISTS (
				SELECT 1 FROM performance_schema.data_lock_waits
				WHERE ENGINE = 'INNODB' AND REQUESTING_THREAD_ID = ? AND BLOCKING_THREAD_ID = ?
			)`, waiter.thread, holder.thread).Scan(&observed)
		if err != nil {
			cancel()
			<-completed
			s.t.Fatal("could not observe the MySQL InnoDB data lock wait")
		}
		if observed && waitCtx.Err() == nil {
			return <-completed
		}
		select {
		case <-completed:
			s.t.Fatal("MySQL waiter ended before its InnoDB data lock wait was observed")
		case <-probe.C:
		}
	}
}

func (s *scenario) updateMySQLLockRow(tx *sql.Tx, id int, code string) {
	s.t.Helper()

	result, err := tx.ExecContext(s.ctx, mysqlContendedUpdate, code, id)
	if err != nil {
		s.t.Fatal("could not update the MySQL lock fixture row")
	}
	if changed, err := result.RowsAffected(); err != nil || changed != 1 {
		s.t.Fatal("MySQL lock fixture update did not change one row")
	}
}

func (s *scenario) checkMySQLLockRow(query func(context.Context, string, ...any) *sql.Row, id int, want string) {
	s.t.Helper()

	var actual string
	if err := query(s.ctx, "SELECT code FROM goef_driver_parent WHERE id = ?", id).
		Scan(&actual); err != nil || actual != want {
		s.t.Fatal("MySQL lock fixture row did not match the expected state")
	}
}

func (s *scenario) rollbackMySQLLockTx(tx *sql.Tx) {
	s.t.Helper()

	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		s.t.Error("could not clean up the MySQL lock fixture transaction")
	}
}
