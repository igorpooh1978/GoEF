// Package transaction manages transactions on a single leased SQL connection.
package transaction

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"sync"
	"sync/atomic"

	"github.com/igorpooh1978/GoEF/goeferr"
)

// State is the last confirmed status of an owned transaction.
type State string

const (
	StateActive         State = "active"
	StateCommitted      State = "committed"
	StateRolledBack     State = "rolled_back"
	StateFailed         State = "failed"
	StateOutcomeUnknown State = "outcome_unknown"
)

// Tx owns a transaction and its connection lease. The application owns the pool.
// Only Begin creates an active Tx. Copies of a handle share the same lifecycle.
type Tx struct {
	owned *ownedTx
}

type ownedTx struct {
	mu           sync.Mutex
	state        atomic.Value // State, initialized before publishing the handle
	lifetime     context.Context
	conn         *sql.Conn
	native       *sql.Tx
	stopRollback func() bool
	cleanupErr   error
}

// Begin acquires one connection and starts a transaction with the driver defaults.
// Canceling ctx cancels active commands and rolls the transaction back. Driver
// transaction control calls may run until the driver returns. If cancellation
// happens during native begin, cleanup finishes before Begin returns an error.
func Begin(ctx context.Context, db *sql.DB) (*Tx, error) {
	if ctx == nil || db == nil {
		return nil, goeferr.NewInvalidState()
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, operationError(err)
	}
	if err := ctx.Err(); err != nil {
		release(conn, false)
		return nil, operationError(err)
	}

	// Own rollback and release in one place. A cancellable native Tx context
	// would let database/sql roll back independently and obscure its result.
	native, err := conn.BeginTx(context.WithoutCancel(ctx), nil)
	if err != nil {
		release(conn, true)
		return nil, operationError(err)
	}
	if err := ctx.Err(); err != nil {
		rollbackErr := native.Rollback()
		release(conn, rollbackErr != nil)
		return nil, errors.Join(operationError(err), rollbackError(rollbackErr))
	}

	owned := &ownedTx{lifetime: ctx, conn: conn, native: native}
	owned.state.Store(StateActive)
	tx := &Tx{owned: owned}
	owned.mu.Lock()
	owned.stopRollback = context.AfterFunc(ctx, func() { _ = tx.Close() })
	owned.mu.Unlock()
	return tx, nil
}

// State reports the status without waiting for an active command. The zero
// handle is failed. Terminal states are visible after the lease is released.
func (tx *Tx) State() State {
	if tx == nil || tx.owned == nil {
		return StateFailed
	}
	return tx.owned.state.Load().(State)
}

// Exec runs one command on the transaction's connection. Use it for statements
// that do not independently end the transaction, such as parameterized DML.
// Execution observes both ctx and the transaction lifetime. After a command
// error, finish the transaction explicitly. A bad connection ends it.
func (tx *Tx) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if ctx == nil {
		return nil, goeferr.NewInvalidState()
	}
	owned, err := tx.lockActive()
	if err != nil {
		return nil, err
	}
	defer owned.mu.Unlock()

	lifetime := owned.lifetime
	command, cancel := context.WithCancelCause(ctx)
	stop := context.AfterFunc(lifetime, func() { cancel(lifetime.Err()) })
	defer func() {
		stop()
		cancel(nil)
	}()
	if err := lifetime.Err(); err != nil {
		return nil, errors.Join(operationError(err), owned.rollback(false))
	}

	result, err := owned.native.ExecContext(command, query, args...)
	if err != nil {
		if errors.Is(err, driver.ErrBadConn) {
			return nil, errors.Join(operationError(err), owned.rollback(true))
		}
		if errors.Is(err, context.Canceled) && ctx.Err() == nil &&
			context.Cause(command) == context.DeadlineExceeded {
			err = context.DeadlineExceeded
		}
		return nil, operationError(err)
	}
	return safeResult{underlying: result}, nil
}

// Commit commits and releases the lease. A native commit error leaves the
// outcome unknown and discards the connection. Reconcile before considering
// retry. If lifetime cancellation is already known, Commit rolls back first.
func (tx *Tx) Commit() error {
	owned, err := tx.lockActive()
	if err != nil {
		return err
	}
	defer owned.mu.Unlock()

	if err := owned.lifetime.Err(); err != nil {
		return errors.Join(operationError(err), owned.rollback(false))
	}
	if err := owned.native.Commit(); err != nil {
		owned.finish(StateOutcomeUnknown, true)
		return errors.Join(goeferr.NewCommitOutcomeUnknown(), contextCause(err))
	}
	owned.finish(StateCommitted, false)
	return nil
}

// Rollback aborts the transaction and releases the lease. A failed rollback
// discards the connection. Repeated completion returns ErrInvalidState.
func (tx *Tx) Rollback() error {
	owned, err := tx.lockActive()
	if err != nil {
		return err
	}
	defer owned.mu.Unlock()
	return owned.rollback(false)
}

// Close waits for any active command and rolls back if the handle is still
// active. Close is safe to repeat and reports any recorded rollback failure.
func (tx *Tx) Close() error {
	if tx == nil || tx.owned == nil {
		return nil
	}
	owned := tx.owned
	owned.mu.Lock()
	defer owned.mu.Unlock()
	if tx.State() != StateActive {
		return owned.cleanupErr
	}
	return owned.rollback(false)
}

func (tx *Tx) lockActive() (*ownedTx, error) {
	if tx.State() != StateActive {
		return nil, goeferr.NewInvalidState()
	}
	owned := tx.owned
	if !owned.mu.TryLock() {
		return nil, goeferr.NewContextBusy()
	}
	if tx.State() != StateActive {
		owned.mu.Unlock()
		return nil, goeferr.NewInvalidState()
	}
	return owned, nil
}

// The lock is held until the connection has returned or been discarded.
func (o *ownedTx) rollback(badConnection bool) error {
	err := o.native.Rollback()
	o.cleanupErr = rollbackError(err)
	if err != nil || badConnection {
		o.finish(StateFailed, true)
	} else {
		o.finish(StateRolledBack, false)
	}
	return o.cleanupErr
}

func (o *ownedTx) finish(state State, discard bool) {
	o.stopRollback()
	release(o.conn, discard)
	o.native, o.conn, o.lifetime, o.stopRollback = nil, nil, nil, nil
	o.state.Store(state)
}

func release(conn *sql.Conn, discard bool) {
	if discard {
		// database/sql discards the connection when Raw returns ErrBadConn.
		// A driver can also discard it during native completion.
		_ = conn.Raw(func(any) error { return driver.ErrBadConn })
	}
	// Close returns nil or ErrConnDone. The latter means it is already released.
	_ = conn.Close()
}

func contextCause(err error) error {
	switch {
	case errors.Is(err, context.Canceled):
		return context.Canceled
	case errors.Is(err, context.DeadlineExceeded):
		return context.DeadlineExceeded
	default:
		return nil
	}
}

func operationError(err error) error {
	if err == nil {
		return nil
	}
	switch contextCause(err) {
	case context.Canceled:
		return errors.Join(goeferr.NewCanceled(), context.Canceled)
	case context.DeadlineExceeded:
		return errors.Join(goeferr.NewDeadlineExceeded(), context.DeadlineExceeded)
	default:
		return goeferr.NewUnknownDriver()
	}
}

func rollbackError(err error) error {
	if err == nil {
		return nil
	}
	return errors.Join(goeferr.NewRollbackFailed(), contextCause(err))
}

type safeResult struct {
	underlying sql.Result
}

func (r safeResult) LastInsertId() (int64, error) {
	value, err := r.underlying.LastInsertId()
	return value, operationError(err)
}

func (r safeResult) RowsAffected() (int64, error) {
	value, err := r.underlying.RowsAffected()
	return value, operationError(err)
}
