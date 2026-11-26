package driverlab

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"
)

type sqliteLockCase struct {
	name           string
	busyTimeout    time.Duration
	callerDeadline time.Duration
}

func (s *scenario) checkSQLiteLockWait() {
	s.t.Helper()
	s.seedParent()

	s.runSQLiteLockCase(sqliteLockCase{"caller deadline", 10 * time.Second, time.Second})
	s.runSQLiteLockCase(sqliteLockCase{"engine busy timeout", time.Second, 10 * time.Second})

	s.facts["lock_deadline_cause"] = "context.DeadlineExceeded"
	s.facts["lock_timeout_native"] = "5"
	s.facts["lock_wait_observed"] = "both"
}

func (s *scenario) runSQLiteLockCase(test sqliteLockCase) {
	s.t.Helper()

	busyMillis := test.busyTimeout.Milliseconds()
	if _, err := s.observer.ExecContext(s.ctx, "PRAGMA busy_timeout="+strconv.FormatInt(busyMillis, 10)); err != nil {
		s.t.Fatal("could not configure the SQLite contender's busy timeout")
	}
	var actualBusyMillis int64
	if err := s.observer.QueryRowContext(s.ctx, "PRAGMA busy_timeout").Scan(&actualBusyMillis); err != nil || actualBusyMillis != busyMillis {
		s.t.Fatal("SQLite contender's busy timeout did not match the expected setting")
	}

	if _, err := s.writer.ExecContext(s.ctx, "BEGIN IMMEDIATE"); err != nil {
		s.t.Fatal("could not acquire the SQLite fixture write lock")
	}
	lockHeld := true
	defer func() {
		if lockHeld && s.rollbackSQLiteHolder() != nil {
			s.t.Error("could not clean up the SQLite write lock")
		}
	}()
	s.updateSQLiteLockRow(s.writer, "held-lock")
	// The competing connection can still see only the committed marker.
	s.checkParent()

	caller, cancel := context.WithTimeout(s.ctx, test.callerDeadline)
	defer cancel()
	start := time.Now()
	err, observed := runObservedSQLiteUpdate(s.t, caller, s.observer, s.sqliteLockUpdate())
	elapsed := time.Since(start)
	callerErr := caller.Err()
	cancel()
	s.t.Logf("SQLite %s: busy_timeout_ms=%d caller_deadline_ms=%d elapsed_ms=%d",
		test.name, busyMillis, test.callerDeadline.Milliseconds(), elapsed.Milliseconds())
	if !observed {
		s.t.Fatal("SQLite busy callback was not observed on the contender before its caller context ended")
	}
	if test.callerDeadline < test.busyTimeout {
		if !errors.Is(err, context.DeadlineExceeded) || !errors.Is(callerErr, context.DeadlineExceeded) {
			s.t.Fatal("SQLite contender did not report the caller deadline")
		}
	} else if s.code(err) != "5" || callerErr != nil { // SQLITE_BUSY
		s.t.Fatal("SQLite contender did not report native SQLITE_BUSY under a live context")
	}
	s.checkParent()

	if s.rollbackSQLiteHolder() != nil {
		s.t.Fatal("could not roll back the SQLite holder")
	}
	lockHeld = false
	s.checkParent()

	// The same contender can now write, and the holder observes its commit.
	s.updateSQLiteLockRow(s.observer, "recovered")
	var recovered string
	if err := s.writer.QueryRowContext(s.ctx, "SELECT code FROM goef_driver_parent WHERE id = ?1", 1).Scan(&recovered); err != nil || recovered != "recovered" {
		s.t.Fatal("SQLite holder did not observe the contender's recovery write")
	}
	s.updateSQLiteLockRow(s.observer, "original")
	s.checkParent()
}

func (s *scenario) sqliteLockUpdate() string {
	return "UPDATE goef_driver_parent SET code = " + s.bind(1) + " WHERE id = " + s.bind(2)
}

func (s *scenario) updateSQLiteLockRow(conn *sql.Conn, code string) {
	s.t.Helper()
	result, err := conn.ExecContext(s.ctx, s.sqliteLockUpdate(), code, 1)
	if err != nil {
		s.t.Fatal("could not update the SQLite lock fixture row")
	}
	if count, err := result.RowsAffected(); err != nil || count != 1 {
		s.t.Fatal("the SQLite lock fixture update did not affect exactly one row")
	}
}

func (s *scenario) rollbackSQLiteHolder() error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(s.ctx), 5*time.Second)
	defer cancel()
	_, err := s.writer.ExecContext(ctx, "ROLLBACK")
	return err
}

func runObservedSQLiteUpdate(t *testing.T, ctx context.Context, contender *sql.Conn, query string) (error, bool) {
	t.Helper()
	switch sqliteVersion {
	case "3.54.1":
		watched := &sqliteBusyContext{Context: ctx, entered: make(chan struct{})}
		err := execSQLiteLockContender(watched, contender, query)
		select {
		case <-watched.entered:
			return err, true
		default:
			return err, false
		}
	case "3.51.3":
		finished := make(chan struct{})
		observed := make(chan bool, 1)
		go func() { observed <- observeLegacySQLiteBusy(ctx, finished) }()
		err := execSQLiteLockContender(ctx, contender, query)
		close(finished)
		return err, <-observed
	default:
		t.Fatal("no busy callback observation is defined for the linked SQLite profile")
		return nil, false
	}
}

// Keep a distinct frame for the legacy observer to locate in the contender's stack.
//
//go:noinline
func execSQLiteLockContender(ctx context.Context, contender *sql.Conn, query string) error {
	_, err := contender.ExecContext(ctx, query, "contending", 1)
	return err
}

// v1.63.0 calls Done from its private Go busy handler on the contender's stack.
// Forward the original context and witness only that exact source-pinned call.
type sqliteBusyContext struct {
	context.Context
	entered chan struct{}
	once    sync.Once
}

func (c *sqliteBusyContext) Done() <-chan struct{} {
	done := c.Context.Done()
	var callers [8]uintptr
	n := runtime.Callers(2, callers[:])
	frames := runtime.CallersFrames(callers[:n])
	for {
		frame, more := frames.Next()
		if frame.Function == "modernc.org/sqlite.busyHandler" && c.Context.Err() == nil {
			c.once.Do(func() { close(c.entered) })
			break
		}
		if !more {
			break
		}
	}
	return done
}

// v1.47.0 still uses the transpiled native busy handler. Observe its frame
// on the dedicated contender's stack in this sequential fixture. A missed
// sample fails the scenario, regardless of the statement's elapsed time.
func observeLegacySQLiteBusy(ctx context.Context, finished <-chan struct{}) bool {
	const contenderFrame = "execSQLiteLockContender("
	const callbackFrame = "modernc.org/sqlite/lib._sqliteDefaultBusyCallback("
	stack := make([]byte, 64*1024)
	poll := time.NewTicker(2 * time.Millisecond)
	defer poll.Stop()
	for {
		select {
		case <-ctx.Done():
			return false
		case <-finished:
			return false
		case <-poll.C:
			n := runtime.Stack(stack, true)
			for _, goroutine := range bytes.Split(stack[:n], []byte("\n\n")) {
				if bytes.Contains(goroutine, []byte(contenderFrame)) && bytes.Contains(goroutine, []byte(callbackFrame)) {
					return ctx.Err() == nil
				}
			}
		}
	}
}
