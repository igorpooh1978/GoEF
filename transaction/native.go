package transaction

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"time"

	"github.com/igorpooh1978/GoEF/goeferr"
)

// NativeCommands binds a provider's commands to one native transaction.
// Implementations consume a scanned row before returning and use
// sql.ErrNoRows and sql.ErrTxDone for those outcomes. Use driver.ErrBadConn
// to signal an unusable connection and retain any standard context cause.
type NativeCommands interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	Scan(ctx context.Context, query string, args []any, dest ...any) error
}

// NativeTransaction adds completion and lease ownership to NativeCommands.
// Release consumes the checkout once. With discard set, it must prevent the
// connection from going back into circulation even if its context expires.
// Commit can return goeferr.ErrCommitRolledBack only on a confirmed rollback
// acknowledged by the server. Other commit errors have an unknown outcome.
type NativeTransaction interface {
	NativeCommands
	Commit(context.Context) error
	Rollback(context.Context) error
	Release(ctx context.Context, discard bool)
}

// BeginNative starts an owned transaction through a provider's native driver.
// open uses the lifetime context to acquire a connection and start the
// transaction. On error, open returns no handle and cleans up the checkout.
// On success, GoEF controls completion and releases or discards the connection.
// Commit uses the lifetime context. Rollback and release use independent,
// bounded cleanup contexts so cancellation cannot skip cleanup.
func BeginNative(ctx context.Context, dialect Dialect, open func(context.Context) (NativeTransaction, error)) (*Tx, error) {
	if ctx == nil || open == nil {
		return nil, goeferr.NewInvalidState()
	}
	if err := new(savepointStack).validateDialect(dialect); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, operationError(err)
	}
	native, err := open(ctx)
	if err != nil {
		return nil, operationError(err)
	}
	if nilCommands(native) {
		return nil, goeferr.NewInvalidState()
	}
	if err := ctx.Err(); err != nil {
		cleanup, cancel := cleanupContext(ctx)
		defer cancel()
		rollbackErr := native.Rollback(cleanup)
		releaseCtx, stopRelease := cleanupContext(ctx)
		defer stopRelease()
		native.Release(releaseCtx, rollbackErr != nil)
		return nil, errors.Join(operationError(err), rollbackError(rollbackErr))
	}
	return own(ctx, dialect, native), nil
}

// NewExternalCommands coordinates participation through a borrowed adapter.
// Use one coordinator per native transaction and serialize any direct native
// use with coordinator operations. The caller retains completion and lease
// ownership. A native sql.ErrTxDone detaches the coordinator.
func NewExternalCommands(dialect Dialect, commands NativeCommands) (*External, error) {
	if nilCommands(commands) {
		return nil, goeferr.NewInvalidState()
	}
	stack := savepointStack{dialect: dialect}
	if err := stack.validateDialect(dialect); err != nil {
		return nil, err
	}
	return &External{shared: &externalTx{native: commands, savepoints: stack}}, nil
}

func nilCommands(commands NativeCommands) bool {
	value := reflect.ValueOf(commands)
	switch value.Kind() {
	case reflect.Invalid:
		return true
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func cleanupContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
}

type sqlCommands struct {
	*sql.Tx
}

func (c sqlCommands) Scan(ctx context.Context, query string, args []any, dest ...any) error {
	return c.QueryRowContext(ctx, query, args...).Scan(dest...)
}

type sqlNativeTransaction struct {
	sqlCommands
	conn               *sql.Conn
	discardAfterFinish bool
}

func (tx *sqlNativeTransaction) Commit(context.Context) error {
	return tx.Tx.Commit()
}

func (tx *sqlNativeTransaction) Rollback(context.Context) error {
	return tx.Tx.Rollback()
}

func (tx *sqlNativeTransaction) Release(_ context.Context, discard bool) {
	release(tx.conn, discard || tx.discardAfterFinish)
}
