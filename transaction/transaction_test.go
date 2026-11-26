package transaction_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/igorpooh1978/GoEF/goeferr"
	"github.com/igorpooh1978/GoEF/transaction"
)

var privateError = errors.New("injected private driver diagnostic")

type probe struct {
	open, close, exec, commit, rollback                   atomic.Int32
	connectErr, beginErr, execErr, commitErr, rollbackErr error
	beginStarted, execStarted                             chan struct{}
	beginContinue                                         chan struct{}
	connected                                             func()
	result                                                driver.Result
}

func (*probe) Driver() driver.Driver { return nil }

func (p *probe) Connect(context.Context) (driver.Conn, error) {
	if p.connectErr != nil {
		return nil, p.connectErr
	}
	p.open.Add(1)
	if p.connected != nil {
		p.connected()
	}
	return &probeConn{p: p}, nil
}

type probeConn struct {
	p      *probe
	active bool
}

func (*probeConn) Prepare(string) (driver.Stmt, error) { return nil, privateError }
func (*probeConn) Begin() (driver.Tx, error)           { return nil, privateError }
func (c *probeConn) Close() error                      { c.p.close.Add(1); return nil }

func (c *probeConn) BeginTx(_ context.Context, _ driver.TxOptions) (driver.Tx, error) {
	if c.p.beginStarted != nil {
		close(c.p.beginStarted)
		<-c.p.beginContinue
	}
	if c.p.beginErr != nil {
		return nil, c.p.beginErr
	}
	c.active = true
	return c, nil
}

func (c *probeConn) ExecContext(ctx context.Context, _ string, _ []driver.NamedValue) (driver.Result, error) {
	c.p.exec.Add(1)
	if !c.active {
		return nil, privateError
	}
	if c.p.execStarted != nil {
		close(c.p.execStarted)
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if c.p.execErr != nil {
		return nil, c.p.execErr
	}
	if c.p.result != nil {
		return c.p.result, nil
	}
	return driver.RowsAffected(1), nil
}

func (c *probeConn) Commit() error {
	c.p.commit.Add(1)
	c.active = false
	return c.p.commitErr
}

func (c *probeConn) Rollback() error {
	c.p.rollback.Add(1)
	c.active = false
	return c.p.rollbackErr
}

func openProbe(t *testing.T, p *probe) *sql.DB {
	t.Helper()
	db := sql.OpenDB(p)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func begin(t *testing.T, ctx context.Context, db *sql.DB) *transaction.Tx {
	t.Helper()
	tx, err := transaction.Begin(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Close() })
	return tx
}

func checkState(t *testing.T, tx *transaction.Tx, db *sql.DB, state transaction.State, inUse int) {
	t.Helper()
	if got := tx.State(); got != state {
		t.Fatalf("state = %v, want %v", got, state)
	}
	if got := db.Stats().InUse; got != inUse {
		t.Fatalf("connections in use = %d, want %d", got, inUse)
	}
}

func TestOwnedTransactionReturnsTheSameLease(t *testing.T) {
	p := new(probe)
	db := openProbe(t, p)
	for _, tc := range []struct {
		name   string
		finish func(*transaction.Tx) error
		state  transaction.State
	}{
		{"commit", (*transaction.Tx).Commit, transaction.StateCommitted},
		{"rollback", (*transaction.Tx).Rollback, transaction.StateRolledBack},
		{"close", (*transaction.Tx).Close, transaction.StateRolledBack},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx := begin(t, t.Context(), db)
			checkState(t, tx, db, transaction.StateActive, 1)
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			result, err := tx.Exec(ctx, "synthetic write")
			if err != nil {
				t.Fatal(err)
			}
			if got, err := result.RowsAffected(); got != 1 || err != nil {
				t.Fatalf("affected rows = %d, error = %v", got, err)
			}
			if err := tc.finish(tx); err != nil {
				t.Fatal(err)
			}
			checkState(t, tx, db, tc.state, 0)
			_, execErr := tx.Exec(t.Context(), "synthetic write")
			for _, err := range []error{execErr, tx.Commit(), tx.Rollback()} {
				if !errors.Is(err, goeferr.ErrInvalidState) {
					t.Fatalf("terminal command error = %v", err)
				}
			}
			if err := tx.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
	if p.open.Load() != 1 || p.close.Load() != 0 || p.exec.Load() != 3 ||
		p.commit.Load() != 1 || p.rollback.Load() != 2 {
		t.Fatal("commands or completion escaped the one pooled connection")
	}
}

func TestCommitFailureReportsUnknownAndDiscards(t *testing.T) {
	p := &probe{commitErr: privateError}
	db := openProbe(t, p)
	tx := begin(t, t.Context(), db)
	err := tx.Commit()
	var detail *goeferr.ErrorDetail
	if !errors.Is(err, goeferr.ErrCommitOutcomeUnknown) || !errors.As(err, &detail) ||
		errors.Is(err, privateError) || detail.ID() != "GOEF-TX-0001" ||
		detail.Phase() != "commit" || detail.Outcome() != "unknown" ||
		detail.RetryClass() != "resolve_outcome" {
		t.Fatalf("unsafe commit result = %v", err)
	}
	checkState(t, tx, db, transaction.StateOutcomeUnknown, 0)
	replacement := begin(t, t.Context(), db)
	if p.close.Load() != 1 || p.open.Load() != 2 || p.commit.Load() != 1 || p.rollback.Load() != 0 {
		t.Fatal("failed commit released an unsafe connection or replayed")
	}
	if err := replacement.Rollback(); err != nil {
		t.Fatal(err)
	}
}

func TestLifetimeCancellationReleasesInFlightCommand(t *testing.T) {
	p := &probe{execStarted: make(chan struct{})}
	db := openProbe(t, p)
	lifetime, stop := context.WithCancel(t.Context())
	defer stop()
	tx := begin(t, lifetime, db)
	done := make(chan error, 1)
	go func() {
		_, err := tx.Exec(t.Context(), "synthetic long-running write")
		done <- err
	}()
	select {
	case <-p.execStarted:
	case <-time.After(time.Second):
		t.Fatal("command did not reach the driver")
	}
	if err := tx.Commit(); !errors.Is(err, goeferr.ErrContextBusy) {
		t.Fatalf("concurrent commit = %v", err)
	}
	stop()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("command cancellation = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("command did not observe lifetime cancellation")
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("lifetime cancellation did not release the pool slot: %v", err)
	}
	_ = conn.Close()
	if err := tx.Close(); err != nil {
		t.Fatal(err)
	}
	checkState(t, tx, db, transaction.StateRolledBack, 0)
	if p.rollback.Load() != 1 || p.commit.Load() != 0 || p.open.Load() != 1 {
		t.Fatal("cancellation completed the transaction incorrectly")
	}
}
