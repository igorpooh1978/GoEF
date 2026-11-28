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

type beginProbe struct {
	transaction.NativeCommands
	rollbackErr  error
	rollback     context.Context
	rollbackDone error
	release      context.Context
	releaseDone  error
	discarded    bool
}

func (*beginProbe) Commit(context.Context) error {
	panic("BeginNative unexpectedly called commit")
}

func (p *beginProbe) Rollback(ctx context.Context) error {
	p.rollback = ctx
	p.rollbackDone = ctx.Err()
	return p.rollbackErr
}

func (p *beginProbe) Release(ctx context.Context, discard bool) {
	p.release = ctx
	p.releaseDone = ctx.Err()
	p.discarded = discard
}

func TestNativeBridgeRejectsInvalidInputs(t *testing.T) {
	var typedNil *beginProbe
	for _, commands := range []transaction.NativeCommands{nil, typedNil} {
		external, err := transaction.NewExternalCommands(transaction.DialectPostgreSQL, commands)
		if external != nil || !errors.Is(err, goeferr.ErrInvalidState) {
			t.Fatalf("NewExternalCommands(nil) = %v, %v", external, err)
		}
	}
	if external, err := transaction.NewExternalCommands("unknown", &beginProbe{}); external != nil ||
		!errors.Is(err, goeferr.ErrUnsupportedFeature) {
		t.Fatalf("NewExternalCommands(unknown) = %v, %v", external, err)
	}
	if tx, err := transaction.BeginNative(t.Context(), transaction.DialectPostgreSQL, nil); tx != nil || !errors.Is(err, goeferr.ErrInvalidState) {
		t.Fatalf("BeginNative(nil open) = %v, %v", tx, err)
	}
	open := func(context.Context) (transaction.NativeTransaction, error) { return typedNil, nil }
	if tx, err := transaction.BeginNative(t.Context(), transaction.DialectPostgreSQL, open); tx != nil || !errors.Is(err, goeferr.ErrInvalidState) {
		t.Fatalf("BeginNative(nil transaction) = %v, %v", tx, err)
	}
	if tx, err := transaction.BeginNative(t.Context(), "unknown", open); tx != nil || !errors.Is(err, goeferr.ErrUnsupportedFeature) {
		t.Fatalf("BeginNative(unknown) = %v, %v", tx, err)
	}
}

func TestNativeBridgeCleansUpAfterCanceledOpen(t *testing.T) {
	for _, tc := range []struct {
		name        string
		rollbackErr error
	}{
		{"rollback confirmed", nil},
		{"rollback failed", errors.New("native private rollback error")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lifetime, cancel := context.WithCancel(t.Context())
			defer cancel()
			probe := &beginProbe{rollbackErr: tc.rollbackErr}
			tx, err := transaction.BeginNative(lifetime, transaction.DialectPostgreSQL, func(ctx context.Context) (transaction.NativeTransaction, error) {
				if ctx != lifetime {
					t.Fatal("open lost the caller's lifetime")
				}
				cancel()
				return probe, nil
			})
			if tx != nil || !errors.Is(err, context.Canceled) || !errors.Is(err, goeferr.ErrCanceled) {
				t.Fatalf("BeginNative exposed an active transaction or lost cancellation: %v, %v", tx, err)
			}
			if errors.Is(err, goeferr.ErrRollbackFailed) != (tc.rollbackErr != nil) ||
				(tc.rollbackErr != nil && errors.Is(err, tc.rollbackErr)) {
				t.Fatalf("BeginNative did not report a redacted rollback result: %v", err)
			}
			if probe.rollbackDone != nil || probe.releaseDone != nil {
				t.Fatal("native cleanup inherited cancellation")
			}
			for _, cleanup := range []context.Context{probe.rollback, probe.release} {
				if cleanup == nil {
					t.Fatal("BeginNative skipped native cleanup")
				}
				deadline, hasDeadline := cleanup.Deadline()
				if !hasDeadline || time.Until(deadline) > 5*time.Second || !errors.Is(cleanup.Err(), context.Canceled) {
					t.Fatal("BeginNative did not use and finish a bounded cleanup context")
				}
			}
			if probe.discarded != (tc.rollbackErr != nil) {
				t.Fatal("BeginNative selected the wrong disposition after rollback")
			}
		})
	}
}

type deadlineProbe struct {
	beginProbe
	expire context.CancelFunc
}

func (p *deadlineProbe) fail(ctx context.Context) error {
	p.expire()
	<-ctx.Done()
	return errors.Join(driver.ErrBadConn, ctx.Err())
}

func (p *deadlineProbe) ExecContext(ctx context.Context, _ string, _ ...any) (sql.Result, error) {
	return nil, p.fail(ctx)
}

func (p *deadlineProbe) Scan(ctx context.Context, _ string, _ []any, _ ...any) error {
	return p.fail(ctx)
}

func TestNativeLifetimeDeadlineAndBadConnection(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  func(context.Context, *transaction.Tx) error
	}{
		{"exec", func(ctx context.Context, tx *transaction.Tx) error {
			_, err := tx.Exec(ctx, "UPDATE fixture")
			return err
		}},
		{"scan", func(ctx context.Context, tx *transaction.Tx) error {
			return tx.Scan(ctx, "SELECT fixture", nil, new(int))
		}},
		{"savepoint", func(ctx context.Context, tx *transaction.Tx) error {
			_, err := tx.Savepoint(ctx, transaction.DialectPostgreSQL)
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, stop := context.WithTimeout(t.Context(), 5*time.Second)
			defer stop()
			lifetime, expire := context.WithCancel(ctx)
			defer expire()
			probe := &deadlineProbe{expire: expire}
			tx, err := transaction.BeginNative(cancelAsDeadline{lifetime}, transaction.DialectPostgreSQL, func(context.Context) (transaction.NativeTransaction, error) {
				return probe, nil
			})
			if err != nil {
				t.Fatal(err)
			}
			err = tc.run(ctx, tx)
			if !errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) || errors.Is(err, driver.ErrBadConn) {
				t.Fatalf("the public error lost the lifetime deadline: %v", err)
			}
			if !probe.discarded || probe.rollbackDone != nil || probe.releaseDone != nil || tx.State() != transaction.StateFailed {
				t.Fatal("the interrupted transaction did not discard the failed session with bounded cleanup")
			}
		})
	}
}
