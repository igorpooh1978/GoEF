package postgres

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"reflect"
	"time"

	"github.com/igorpooh1978/GoEF/goeferr"
	"github.com/igorpooh1978/GoEF/transaction"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// BeginTx starts an owned transaction on the caller's pgx pool. Supported
// isolation levels are Default, Read Committed, Repeatable Read and
// Serializable. ReadOnly is optional for each level. ctx controls pool
// acquisition, BEGIN, the transaction lifetime and COMMIT. Rollback and
// connection cleanup can continue with a bounded budget after ctx expires.
// A deferred foreign key violation at COMMIT confirms rollback when PostgreSQL
// returns the connection to an idle state.
//
// Use transaction-preserving statements and avoid session-level effects on
// the borrowed connection. The caller owns the pool and its configuration.
func BeginTx(ctx context.Context, pool *pgxpool.Pool, options sql.TxOptions) (*transaction.Tx, error) {
	if ctx == nil || pool == nil {
		return nil, goeferr.NewInvalidState()
	}
	var nativeOptions pgx.TxOptions
	switch options.Isolation {
	case sql.LevelDefault:
	case sql.LevelReadCommitted:
		nativeOptions.IsoLevel = pgx.ReadCommitted
	case sql.LevelRepeatableRead:
		nativeOptions.IsoLevel = pgx.RepeatableRead
	case sql.LevelSerializable:
		nativeOptions.IsoLevel = pgx.Serializable
	default:
		return nil, goeferr.NewUnsupportedFeature()
	}
	if options.ReadOnly {
		nativeOptions.AccessMode = pgx.ReadOnly
	}
	return transaction.BeginNative(ctx, transaction.DialectPostgreSQL, func(ctx context.Context) (transaction.NativeTransaction, error) {
		conn, err := pool.Acquire(ctx)
		if err != nil {
			return nil, err
		}
		native, err := conn.Conn().BeginTx(ctx, nativeOptions)
		if err != nil {
			cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			discardTransactionConnection(cleanup, conn)
			return nil, err
		}
		return &pgxTransaction{pgxCommands{native}, conn}, nil
	})
}

// NewExternal enlists a caller-owned pgx transaction directly, including one
// opened with pgxpool.Pool.Begin. Use one coordinator and serialize direct
// native calls with it. Closing the coordinator or a participant detaches
// GoEF. The caller remains responsible for completion and pool ownership.
func NewExternal(native pgx.Tx) (*transaction.External, error) {
	value := reflect.ValueOf(native)
	switch value.Kind() {
	case reflect.Invalid:
		return nil, goeferr.NewInvalidState()
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Pointer, reflect.Slice:
		if value.IsNil() {
			return nil, goeferr.NewInvalidState()
		}
	}
	return transaction.NewExternalCommands(transaction.DialectPostgreSQL, pgxCommands{native})
}

type pgxCommands struct {
	native pgx.Tx
}

func (c pgxCommands) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	tag, err := c.native.Exec(ctx, query, args...)
	if err != nil {
		return nil, c.commandError(err)
	}
	return driver.RowsAffected(tag.RowsAffected()), nil
}

func (c pgxCommands) Scan(ctx context.Context, query string, args []any, dest ...any) error {
	return c.commandError(c.native.QueryRow(ctx, query, args...).Scan(dest...))
}

func (c pgxCommands) commandError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, pgx.ErrTxClosed):
		return sql.ErrTxDone
	case c.native.Conn().IsClosed():
		return errors.Join(driver.ErrBadConn, err)
	default:
		return err
	}
}

type pgxTransaction struct {
	pgxCommands
	conn *pgxpool.Conn
}

func (tx *pgxTransaction) Commit(ctx context.Context) error {
	err := tx.native.Commit(ctx)
	if errors.Is(err, pgx.ErrTxCommitRollback) {
		return goeferr.ErrCommitRolledBack
	}
	var rejection *pgconn.PgError
	// PostgreSQL checks deferred foreign keys before it records the commit.
	// An idle transaction status alone does not identify the outcome.
	if errors.As(err, &rejection) && rejection.SeverityUnlocalized == "ERROR" && rejection.Code == "23503" {
		conn := tx.conn.Conn()
		if !conn.IsClosed() && conn.PgConn().TxStatus() == 'I' {
			return goeferr.ErrCommitRolledBack
		}
	}
	return err
}

func (tx *pgxTransaction) Rollback(ctx context.Context) error {
	return tx.native.Rollback(ctx)
}

func (tx *pgxTransaction) Release(ctx context.Context, discard bool) {
	if discard {
		discardTransactionConnection(ctx, tx.conn)
		return
	}
	tx.conn.Release()
}

func discardTransactionConnection(ctx context.Context, conn *pgxpool.Conn) {
	// Hijack removes the connection before Close. pgx always closes the
	// transport, even if it cannot send an orderly termination.
	_ = conn.Hijack().Close(ctx)
}
