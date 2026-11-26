package transaction_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"testing"
	"time"

	"github.com/igorpooh1978/GoEF/goeferr"
	"github.com/igorpooh1978/GoEF/transaction"
)

func TestBeginRejectsInvalidOrCanceledInputs(t *testing.T) {
	p := new(probe)
	db := openProbe(t, p)
	for _, tc := range []struct {
		ctx  context.Context
		pool *sql.DB
	}{
		{nil, db},
		{t.Context(), nil},
	} {
		if tx, err := transaction.Begin(tc.ctx, tc.pool); tx != nil || !errors.Is(err, goeferr.ErrInvalidState) {
			t.Fatalf("invalid Begin = %v, %v", tx, err)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if tx, err := transaction.Begin(ctx, db); tx != nil || !errors.Is(err, context.Canceled) ||
		!errors.Is(err, goeferr.ErrCanceled) || p.open.Load() != 0 {
		t.Fatalf("canceled acquisition = %v, %v", tx, err)
	}
	for _, tx := range []*transaction.Tx{nil, {}} {
		if tx.State() != transaction.StateFailed || tx.Close() != nil {
			t.Fatal("uninitialized handle must be safe to inspect and close")
		}
		_, err := tx.Exec(t.Context(), "")
		if !errors.Is(err, goeferr.ErrInvalidState) {
			t.Fatal(err)
		}
	}
	owned := begin(t, t.Context(), db)
	if _, err := owned.Exec(nil, "synthetic write"); !errors.Is(err, goeferr.ErrInvalidState) {
		t.Fatalf("nil command context = %v", err)
	}
}

func TestBeginReleasesCanceledAcquisition(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	p := &probe{connected: cancel}
	db := openProbe(t, p)
	if tx, err := transaction.Begin(ctx, db); tx != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled acquisition = %v, %v", tx, err)
	}
	if db.Stats().InUse != 0 || db.Stats().Idle != 1 || p.close.Load() != 0 {
		t.Fatal("cancellation before native begin did not return the unused lease")
	}
}

func TestNativeBeginFailureDiscardsTheLease(t *testing.T) {
	p := &probe{beginErr: privateError}
	db := openProbe(t, p)
	tx, err := transaction.Begin(t.Context(), db)
	if tx != nil || !errors.Is(err, goeferr.ErrUnknownDriver) || errors.Is(err, privateError) {
		t.Fatalf("begin failure = %v, %v", tx, err)
	}
	if db.Stats().InUse != 0 || p.close.Load() != 1 {
		t.Fatal("failed Begin leaked or reused the native lease")
	}
}

func TestCancelDuringNativeBeginPreservesCleanupOutcome(t *testing.T) {
	for _, tc := range []struct {
		name        string
		rollbackErr error
		discarded   int32
	}{
		{"acknowledged rollback", nil, 0},
		{"unconfirmed rollback", privateError, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &probe{beginStarted: make(chan struct{}), beginContinue: make(chan struct{}), rollbackErr: tc.rollbackErr}
			db := openProbe(t, p)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			done := make(chan error, 1)
			go func() {
				tx, err := transaction.Begin(ctx, db)
				if tx != nil {
					_ = tx.Close()
					err = errors.New("canceled Begin exposed an active handle")
				}
				done <- err
			}()
			select {
			case <-p.beginStarted:
				cancel()
				close(p.beginContinue)
			case <-time.After(2 * time.Second):
				close(p.beginContinue)
				t.Fatal("native begin did not start")
			}
			select {
			case err := <-done:
				if !errors.Is(err, goeferr.ErrCanceled) || !errors.Is(err, context.Canceled) ||
					errors.Is(err, privateError) || errors.Is(err, goeferr.ErrRollbackFailed) != (tc.rollbackErr != nil) {
					t.Fatalf("canceled Begin error = %v", err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("canceled Begin did not finish")
			}
			if db.Stats().InUse != 0 || p.rollback.Load() != 1 || p.close.Load() != tc.discarded {
				t.Fatal("canceled Begin lost its cleanup disposition")
			}
		})
	}
}

func TestRollbackFailureIsRecordedForClose(t *testing.T) {
	p := &probe{rollbackErr: privateError}
	db := openProbe(t, p)
	tx := begin(t, t.Context(), db)
	for _, err := range []error{tx.Rollback(), tx.Close(), tx.Close()} {
		var detail *goeferr.ErrorDetail
		if !errors.Is(err, goeferr.ErrRollbackFailed) || errors.Is(err, privateError) ||
			!errors.As(err, &detail) || detail.Outcome() != "rollback_unconfirmed" {
			t.Fatalf("rollback error = %v", err)
		}
	}
	checkState(t, tx, db, transaction.StateFailed, 0)
	if p.close.Load() != 1 || p.rollback.Load() != 1 {
		t.Fatal("unconfirmed rollback was reused or retried")
	}
}

func TestBadConnectionDuringExecFinishesAndDiscards(t *testing.T) {
	p := &probe{execErr: driver.ErrBadConn, rollbackErr: privateError}
	db := openProbe(t, p)
	tx := begin(t, t.Context(), db)
	_, err := tx.Exec(t.Context(), "synthetic write")
	if !errors.Is(err, goeferr.ErrUnknownDriver) || !errors.Is(err, goeferr.ErrRollbackFailed) ||
		errors.Is(err, driver.ErrBadConn) || errors.Is(err, privateError) {
		t.Fatalf("bad-connection error = %v", err)
	}
	checkState(t, tx, db, transaction.StateFailed, 0)
	if p.close.Load() != 1 || p.rollback.Load() != 1 || p.exec.Load() != 1 {
		t.Fatal("bad connection was reused or replayed")
	}
}

func TestCommandDeadlineRemainsDistinctFromLifetime(t *testing.T) {
	p := new(probe)
	db := openProbe(t, p)
	tx := begin(t, t.Context(), db)
	ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
	defer cancel()
	if _, err := tx.Exec(ctx, "synthetic write"); !errors.Is(err, goeferr.ErrDeadlineExceeded) ||
		!errors.Is(err, context.DeadlineExceeded) || p.exec.Load() != 0 {
		t.Fatalf("expired command error = %v", err)
	}
	checkState(t, tx, db, transaction.StateActive, 1)
	if _, err := tx.Exec(t.Context(), "synthetic retry chosen by caller"); err != nil {
		t.Fatal(err)
	}
}

type cancelAsDeadline struct{ context.Context }

func (c cancelAsDeadline) Err() error {
	if c.Context.Err() != nil {
		return context.DeadlineExceeded
	}
	return nil
}

func TestLifetimeDeadlineReachesExecutingCommand(t *testing.T) {
	p := &probe{execStarted: make(chan struct{})}
	db := openProbe(t, p)
	lifetime, cancel := context.WithCancel(t.Context())
	defer cancel()
	tx := begin(t, cancelAsDeadline{lifetime}, db)
	done := make(chan error, 1)
	go func() {
		_, err := tx.Exec(t.Context(), "synthetic long-running write")
		done <- err
	}()
	select {
	case <-p.execStarted:
		cancel()
	case <-time.After(2 * time.Second):
		t.Fatal("command did not start")
	}
	select {
	case err := <-done:
		if !errors.Is(err, goeferr.ErrDeadlineExceeded) || !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expired transaction lifetime error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("command did not stop")
	}
	if err := tx.Close(); err != nil {
		t.Fatal(err)
	}
	checkState(t, tx, db, transaction.StateRolledBack, 0)
}

type privateResult struct{}

func (privateResult) LastInsertId() (int64, error) { return 0, privateError }
func (privateResult) RowsAffected() (int64, error) { return 0, privateError }

func TestResultDoesNotExposeDriverErrors(t *testing.T) {
	db := openProbe(t, &probe{result: privateResult{}})
	tx := begin(t, t.Context(), db)
	result, err := tx.Exec(t.Context(), "synthetic write")
	if err != nil {
		t.Fatal(err)
	}
	_, idErr := result.LastInsertId()
	_, countErr := result.RowsAffected()
	for _, err := range []error{idErr, countErr} {
		if !errors.Is(err, goeferr.ErrUnknownDriver) || errors.Is(err, privateError) {
			t.Fatalf("unsafe result error = %v", err)
		}
	}
}
