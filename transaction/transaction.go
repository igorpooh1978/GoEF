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
// Begin and BeginWithOptions create active transactions. Copies of a handle
// share the same lifecycle.
type Tx struct {
	owned *ownedTx
}

type ownedTx struct {
	mu           sync.Mutex
	state        atomic.Value // State, initialized before publishing the handle
	lifetime     context.Context
	native       NativeTransaction
	stopRollback func() bool
	cleanupErr   error
	savepoints   savepointStack
}

// Begin acquires one connection and starts a transaction with the driver defaults.
// Canceling ctx cancels active commands and rolls the transaction back. Driver
// transaction control calls may run until the driver returns. If cancellation
// happens during native begin, cleanup finishes before Begin returns an error.
func Begin(ctx context.Context, db *sql.DB) (*Tx, error) {
	return begin(ctx, db, "", sql.TxOptions{})
}

func begin(ctx context.Context, db *sql.DB, dialect Dialect, options sql.TxOptions) (*Tx, error) {
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
	if dialect == DialectSQLite && options.Isolation == sql.LevelSerializable {
		var readUncommitted int
		err := conn.QueryRowContext(ctx, "PRAGMA read_uncommitted").Scan(&readUncommitted)
		if err != nil {
			release(conn, errors.Is(err, driver.ErrBadConn))
			return nil, operationError(err)
		}
		if readUncommitted != 0 {
			release(conn, false)
			return nil, goeferr.NewUnsupportedFeature()
		}
		if err := ctx.Err(); err != nil {
			release(conn, false)
			return nil, operationError(err)
		}
		// The driver leaves isolation at SQLite's native setting.
		options.Isolation = sql.LevelDefault
	}
	discardAfterFinish := dialect == DialectSQLServer && options.Isolation != sql.LevelDefault

	// Own rollback and release in one place. A cancellable native Tx context
	// would let database/sql roll back independently and obscure its result.
	native, err := conn.BeginTx(context.WithoutCancel(ctx), &options)
	if err != nil {
		release(conn, true)
		return nil, operationError(err)
	}
	if err := ctx.Err(); err != nil {
		rollbackErr := native.Rollback()
		release(conn, rollbackErr != nil || discardAfterFinish)
		return nil, errors.Join(operationError(err), rollbackError(rollbackErr))
	}

	return own(ctx, dialect, &sqlNativeTransaction{
		sqlCommands: sqlCommands{native},
		conn:        conn, discardAfterFinish: discardAfterFinish,
	}), nil
}

func own(ctx context.Context, dialect Dialect, native NativeTransaction) *Tx {
	owned := &ownedTx{
		lifetime: ctx, native: native,
		savepoints: savepointStack{dialect: dialect},
	}
	owned.state.Store(StateActive)
	tx := &Tx{owned: owned}
	owned.mu.Lock()
	owned.stopRollback = context.AfterFunc(ctx, func() { _ = tx.Close() })
	owned.mu.Unlock()
	return tx
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
	var result sql.Result
	err := tx.withCommand(ctx, func(command context.Context, owned *ownedTx) error {
		nativeResult, err := owned.native.ExecContext(command, query, args...)
		if errors.Is(err, driver.ErrBadConn) {
			return errors.Join(operationError(commandError(ctx, command, err)), owned.rollback(true))
		}
		if err != nil {
			return operationError(commandError(ctx, command, err))
		}
		result = safeResult{underlying: nativeResult}
		return nil
	})
	return result, err
}

// withCommand serializes SQL commands with lifetime cleanup and completion.
func (tx *Tx) withCommand(ctx context.Context, run func(context.Context, *ownedTx) error) error {
	if ctx == nil {
		return goeferr.NewInvalidState()
	}
	owned, err := tx.lockState(StateActive)
	if err != nil {
		return err
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
		return errors.Join(operationError(err), owned.rollback(false))
	}
	if err := command.Err(); err != nil {
		return operationError(commandError(ctx, command, err))
	}
	return run(command, owned)
}

func commandError(ctx, command context.Context, err error) error {
	if errors.Is(err, context.Canceled) && ctx.Err() == nil &&
		context.Cause(command) == context.DeadlineExceeded {
		return context.DeadlineExceeded
	}
	return err
}

// Commit ends the transaction and releases the lease. An explicitly confirmed
// rollback returns goeferr.ErrCommitRolledBack. An unconfirmed commit error
// discards the connection and requires [Tx.ReconcileCommit] before considering
// retry. If lifetime cancellation is already known, Commit rolls back first.
func (tx *Tx) Commit() error {
	owned, err := tx.lockState(StateActive)
	if err != nil {
		return err
	}
	defer owned.mu.Unlock()

	if err := owned.lifetime.Err(); err != nil {
		return errors.Join(operationError(err), owned.rollback(false))
	}
	if err := owned.native.Commit(owned.lifetime); err != nil {
		if errors.Is(err, goeferr.ErrCommitRolledBack) {
			owned.finish(StateRolledBack, false)
			return goeferr.NewCommitRolledBack()
		}
		owned.finish(StateOutcomeUnknown, true)
		return errors.Join(goeferr.NewCommitOutcomeUnknown(), contextCause(err))
	}
	owned.finish(StateCommitted, false)
	return nil
}

// Rollback aborts the transaction and releases the lease. A failed rollback
// discards the connection. Repeated completion returns ErrInvalidState.
func (tx *Tx) Rollback() error {
	owned, err := tx.lockState(StateActive)
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
	// Terminal state publishes the immutable cleanup result. A verifier
	// can run after that publication without delaying resource cleanup.
	if tx.State() != StateActive {
		return owned.cleanupErr
	}
	owned.mu.Lock()
	defer owned.mu.Unlock()
	if tx.State() != StateActive {
		return owned.cleanupErr
	}
	return owned.rollback(false)
}

func (tx *Tx) lockState(expected State) (*ownedTx, error) {
	if tx.State() != expected {
		return nil, goeferr.NewInvalidState()
	}
	owned := tx.owned
	if !owned.mu.TryLock() {
		return nil, goeferr.NewContextBusy()
	}
	if tx.State() != expected {
		owned.mu.Unlock()
		return nil, goeferr.NewInvalidState()
	}
	return owned, nil
}

// The lock is held until the connection has returned or been discarded.
func (o *ownedTx) rollback(badConnection bool) error {
	cleanup, cancel := cleanupContext(o.lifetime)
	defer cancel()
	err := o.native.Rollback(cleanup)
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
	cleanup, cancel := cleanupContext(o.lifetime)
	defer cancel()
	o.native.Release(cleanup, discard)
	o.native, o.lifetime, o.stopRollback = nil, nil, nil
	o.savepoints = savepointStack{}
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
