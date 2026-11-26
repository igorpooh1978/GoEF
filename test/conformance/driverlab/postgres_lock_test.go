package driverlab

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

func (s *scenario) checkPostgresLockWait() {
	s.t.Helper()
	s.seedParent()

	s.postgresLockWaitRound(time.Second, 10*time.Second)
	// A caller deadline can retire pgx's physical connection. Acquire a fresh
	// lease before checking state and starting the independent second round.
	if err := s.observer.Close(); err != nil && !errors.Is(err, sql.ErrConnDone) {
		s.t.Fatal("could not release PostgreSQL deadline contender")
	}
	s.observer = s.lease()
	s.checkParent()

	s.postgresLockWaitRound(10*time.Second, time.Second)
	s.checkParent()

	s.facts["lock_deadline_cause"] = "context.DeadlineExceeded"
	s.facts["lock_timeout_native"] = "55P03"
	s.facts["lock_wait_observed"] = "both"
}

func (s *scenario) postgresLockWaitRound(callerTimeout, lockTimeout time.Duration) {
	s.t.Helper()
	deadlineFirst := callerTimeout < lockTimeout

	// Rollback uses the fixture lifetime even after the contender's operation
	// context ends. Keep the holder alive until the contender has returned.
	holder, err := s.writer.BeginTx(s.ctx, nil)
	if err != nil {
		s.t.Fatal("could not start PostgreSQL lock holder")
	}
	defer s.rollbackPostgresLockWait(holder, false)
	contender, err := s.observer.BeginTx(s.ctx, nil)
	if err != nil {
		s.t.Fatal("could not start PostgreSQL lock contender")
	}
	defer s.rollbackPostgresLockWait(contender, deadlineFirst)

	holderPID := s.postgresLockWaitPID(holder)
	contenderPID := s.postgresLockWaitPID(contender)
	if holderPID == contenderPID {
		s.t.Fatal("PostgreSQL lock actors share one backend")
	}
	if _, err := contender.ExecContext(s.ctx, "SELECT set_config('lock_timeout', $1, true)", lockTimeout.String()); err != nil {
		s.t.Fatal("could not set PostgreSQL transaction lock timeout")
	}
	const update = "UPDATE goef_driver_parent SET code = $1 WHERE id = $2"
	result, err := holder.ExecContext(s.ctx, update, "held", 1)
	if err != nil {
		s.t.Fatal("could not acquire PostgreSQL fixture row lock")
	}
	if count, err := result.RowsAffected(); err != nil || count != 1 {
		s.t.Fatal("PostgreSQL holder did not update exactly the seeded row")
	}

	type outcome struct {
		err        error
		contextErr error
	}
	request, cancel := context.WithTimeout(s.ctx, callerTimeout)
	done := make(chan outcome, 1)
	go func() {
		defer close(done)
		_, err := contender.ExecContext(request, update, "waiting", 1)
		done <- outcome{err: err, contextErr: request.Err()}
	}()
	// Join the worker before either transaction rolls back, even on failure.
	defer func() {
		cancel()
		for range done {
		}
	}()

	// The holder can inspect both PIDs while retaining the lock. This keeps
	// the probe on two existing leases and avoids a statistics snapshot.
	const blocked = "SELECT $1::integer = ANY(pg_blocking_pids($2::integer))"
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		var observed bool
		if err := holder.QueryRowContext(s.ctx, blocked, holderPID, contenderPID).Scan(&observed); err != nil {
			s.t.Fatal("could not inspect PostgreSQL lock wait")
		}
		if observed && request.Err() == nil {
			break
		}
		select {
		case <-done:
			s.t.Fatal("PostgreSQL contender ended before a lock wait was observed")
		case <-s.ctx.Done():
			s.t.Fatal("PostgreSQL fixture ended before a lock wait was observed")
		case <-ticker.C:
		}
	}

	select {
	case finished := <-done:
		if deadlineFirst {
			if !errors.Is(finished.err, context.DeadlineExceeded) || finished.contextErr != context.DeadlineExceeded {
				s.t.Fatal("PostgreSQL lock wait did not preserve the caller deadline")
			}
		} else if finished.contextErr != nil || errors.Is(finished.err, context.DeadlineExceeded) || s.code(finished.err) != "55P03" {
			s.t.Fatal("PostgreSQL did not report lock timeout under a live caller context")
		}
	case <-s.ctx.Done():
		s.t.Fatal("PostgreSQL fixture ended before the lock contender returned")
	}
}

func (s *scenario) postgresLockWaitPID(tx *sql.Tx) int {
	s.t.Helper()
	var pid int
	if err := tx.QueryRowContext(s.ctx, "SELECT pg_backend_pid()").Scan(&pid); err != nil || pid <= 0 {
		s.t.Fatal("could not identify PostgreSQL lock actor")
	}
	return pid
}

func (s *scenario) rollbackPostgresLockWait(tx *sql.Tx, mayBeClosed bool) {
	s.t.Helper()
	err := tx.Rollback()
	if err == nil {
		return
	}
	if mayBeClosed && (errors.Is(err, pgconn.ErrConnClosed) || errors.Is(err, driver.ErrBadConn) || errors.Is(err, sql.ErrConnDone)) {
		return
	}
	s.t.Error("could not roll back PostgreSQL lock actor")
}
