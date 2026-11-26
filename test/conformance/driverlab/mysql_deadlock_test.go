package driverlab

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
)

const mysqlDeadlockResultPrefix = "GOEF_MYSQL_DEADLOCK_RESULT="

func TestMySQLDeadlockRecovery(t *testing.T) {
	if os.Getenv("GOEF_DRIVERLAB_MYSQL_DEADLOCK") != "1" {
		t.Skip("run the isolated MySQL deadlock lab")
	}
	profile := os.Getenv("GOEF_DRIVERLAB_PROFILE")
	if (profile != "mysql-8.0" && profile != "mysql-8.4") || os.Getenv("GOEF_DRIVERLAB_PROVIDER") != "mysql" {
		t.Fatal("the MySQL deadlock lab requires an exact MySQL fixture profile")
	}
	s := &scenario{t: t, provider: "mysql"}
	t.Cleanup(func() {
		if t.Failed() {
			return
		}
		record, err := json.Marshal(map[string]string{
			"profile":             profile,
			"server_version":      s.facts["server_version"],
			"status":              "PASS",
			"native_code":         "1213/40001",
			"wait_observed":       "requester_and_blocker",
			"victim_rollback":     "verified_before_explicit_rollback",
			"survivor_commit":     "verified",
			"fresh_victim_commit": "verified",
		})
		if err != nil {
			t.Fatal("could not encode MySQL deadlock evidence")
		}
		fmt.Printf("%s%s\n", mysqlDeadlockResultPrefix, record)
	})
	s.setup()
	s.checkMySQLDeadlock()
}

type mysqlDeadlockAttempt struct {
	result sql.Result
	err    error
}

func (a mysqlDeadlockAttempt) isVictim() bool {
	var native *mysql.MySQLError
	return errors.As(a.err, &native) && native.Number == 1213 && string(native.SQLState[:]) == "40001"
}

func (s *scenario) checkMySQLDeadlock() {
	s.t.Helper()

	seeds := []string{"first", "second", "first-sentinel", "second-sentinel"}
	for i, value := range seeds {
		if _, err := s.writer.ExecContext(s.ctx, s.insertParentSQL(), i+1, value); err != nil {
			s.t.Fatal("could not seed the MySQL deadlock fixture")
		}
	}
	writerID := s.mysqlLockIdentity(s.writer.QueryRowContext)
	observerID := s.mysqlLockIdentity(s.observer.QueryRowContext)
	if writerID.connection == observerID.connection || writerID.thread == observerID.thread {
		s.t.Fatal("MySQL deadlock fixture requires distinct physical sessions")
	}
	for _, conn := range []*sql.Conn{s.writer, s.observer} {
		var detectsDeadlocks bool
		var waitSeconds int
		if err := conn.QueryRowContext(s.ctx, "SELECT @@global.innodb_deadlock_detect, @@session.innodb_lock_wait_timeout").
			Scan(&detectsDeadlocks, &waitSeconds); err != nil || !detectsDeadlocks || waitSeconds < 20 {
			s.t.Fatal("MySQL deadlock fixture requires detection and a longer lock timeout")
		}
	}
	writerTx, err := s.writer.BeginTx(s.ctx, nil)
	if err != nil {
		s.t.Fatal("could not start the MySQL deadlock writer transaction")
	}
	defer s.rollbackMySQLLockTx(writerTx)
	observerTx, err := s.observer.BeginTx(s.ctx, nil)
	if err != nil {
		s.t.Fatal("could not start the MySQL deadlock observer transaction")
	}
	defer s.rollbackMySQLLockTx(observerTx)

	s.updateMySQLLockRow(writerTx, 3, "writer-sentinel")
	s.updateMySQLLockRow(writerTx, 1, "writer-held")
	s.updateMySQLLockRow(observerTx, 4, "observer-sentinel")
	s.updateMySQLLockRow(observerTx, 2, "observer-held")

	ctx, cancel := context.WithTimeout(s.ctx, 8*time.Second)
	done := make(chan struct{})
	var writer mysqlDeadlockAttempt
	go func() {
		defer close(done)
		writer.result, writer.err = writerTx.ExecContext(ctx, mysqlContendedUpdate, "writer-finished", 2)
	}()
	defer func() {
		cancel()
		<-done
	}()
	s.awaitMySQLDeadlockEdge(ctx, observerTx, writerID.thread, observerID.thread, done)
	var observer mysqlDeadlockAttempt
	observer.result, observer.err = observerTx.ExecContext(ctx, mysqlContendedUpdate, "observer-finished", 1)
	<-done
	if ctx.Err() != nil || writer.isVictim() == observer.isVictim() {
		s.t.Fatal("MySQL deadlock did not return exactly one 1213/40001 victim with live callers")
	}

	victimTx, survivorTx := observerTx, writerTx
	victimConn, survivorConn := s.observer, s.writer
	victimID := observerID
	victimSentinel, survivorSentinel := 4, 3
	winner := writer
	committed := []string{"writer-held", "writer-finished", "writer-sentinel", seeds[3]}
	if writer.isVictim() {
		victimTx, survivorTx = writerTx, observerTx
		victimConn, survivorConn = s.writer, s.observer
		victimID = writerID
		victimSentinel, survivorSentinel = 3, 4
		winner = observer
		committed = []string{"observer-finished", "observer-held", seeds[2], "observer-sentinel"}
	}
	if winner.err != nil || winner.result == nil {
		s.t.Fatal("MySQL deadlock survivor did not complete its conflicting update")
	}
	if changed, err := winner.result.RowsAffected(); err != nil || changed != 1 {
		s.t.Fatal("MySQL deadlock survivor did not update exactly one row")
	}

	// The sentinel is independent of the row the survivor acquired. Read it
	// through the victim handle before the test explicitly issues ROLLBACK.
	s.checkMySQLLockRow(victimTx.QueryRowContext, victimSentinel, seeds[victimSentinel-1])
	s.checkMySQLLockRow(survivorTx.QueryRowContext, survivorSentinel, committed[survivorSentinel-1])
	if err := victimTx.Rollback(); err != nil {
		s.t.Fatal("could not finalize the MySQL deadlock victim handle")
	}
	if err := survivorTx.Commit(); err != nil {
		s.t.Fatal("could not commit the MySQL deadlock survivor")
	}
	s.checkMySQLLockRow(victimConn.QueryRowContext, survivorSentinel, committed[survivorSentinel-1])
	s.checkParents(committed...)
	if s.mysqlLockIdentity(victimConn.QueryRowContext) != victimID {
		s.t.Fatal("the MySQL deadlock victim session was replaced")
	}

	recovery, err := victimConn.BeginTx(s.ctx, nil)
	if err != nil {
		s.t.Fatal("could not begin a fresh transaction on the MySQL deadlock victim session")
	}
	defer s.rollbackMySQLLockTx(recovery)
	s.updateMySQLLockRow(recovery, victimSentinel, "recovered")
	if err := recovery.Commit(); err != nil {
		s.t.Fatal("could not commit the fresh MySQL deadlock recovery transaction")
	}
	s.checkMySQLLockRow(survivorConn.QueryRowContext, victimSentinel, "recovered")
	committed[victimSentinel-1] = "recovered"
	s.checkParents(committed...)
}

func (s *scenario) awaitMySQLDeadlockEdge(ctx context.Context, blocker *sql.Tx, requester, owner uint64, done <-chan struct{}) {
	s.t.Helper()

	probe := time.NewTicker(10 * time.Millisecond)
	defer probe.Stop()
	for {
		var observed bool
		err := blocker.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM performance_schema.data_lock_waits
				WHERE ENGINE = 'INNODB' AND REQUESTING_THREAD_ID = ? AND BLOCKING_THREAD_ID = ?
			)`, requester, owner).Scan(&observed)
		if err != nil {
			s.t.Fatal("could not inspect the MySQL deadlock wait edge")
		}
		if observed && ctx.Err() == nil {
			return
		}
		select {
		case <-done:
			s.t.Fatal("the first MySQL deadlock request ended before its wait edge was observed")
		case <-ctx.Done():
			s.t.Fatal("could not observe the MySQL deadlock wait edge before the deadline")
		case <-probe.C:
		}
	}
}
