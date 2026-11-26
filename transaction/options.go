package transaction

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"

	"github.com/igorpooh1978/GoEF/goeferr"
)

// BeginWithOptions starts an owned transaction using the selected engine and
// options. It accepts Default, Read Committed, Repeatable Read and Serializable
// on PostgreSQL, SQL Server and MySQL. PostgreSQL and MySQL also accept ReadOnly.
// SQLite accepts Default and Serializable with read_uncommitted off.
// An unsupported request returns goeferr.ErrUnsupportedFeature. Explicit SQL
// Server isolation discards the physical session after transaction completion
// because a pool reset may leave the isolation setting on the session.
func BeginWithOptions(ctx context.Context, db *sql.DB, dialect Dialect, options sql.TxOptions) (*Tx, error) {
	if ctx == nil || db == nil {
		return nil, goeferr.NewInvalidState()
	}
	if !supportsOptions(dialect, options) {
		return nil, goeferr.NewUnsupportedFeature()
	}
	return begin(ctx, db, dialect, options)
}

func supportsOptions(dialect Dialect, options sql.TxOptions) bool {
	switch dialect {
	case DialectPostgreSQL, DialectMySQL:
	case DialectSQLServer:
		if options.ReadOnly {
			return false
		}
	case DialectSQLite:
		return !options.ReadOnly && (options.Isolation == sql.LevelDefault || options.Isolation == sql.LevelSerializable)
	default:
		return false
	}
	return options.Isolation == sql.LevelDefault || options.Isolation == sql.LevelReadCommitted ||
		options.Isolation == sql.LevelRepeatableRead || options.Isolation == sql.LevelSerializable
}

// Scan reads one row on the transaction's physical connection. It consumes
// the row before releasing the operation gate and returns sql.ErrNoRows for no
// match. Use parameterised SQL and pass arguments separately from destinations.
func (tx *Tx) Scan(ctx context.Context, query string, args []any, dest ...any) error {
	return tx.withCommand(ctx, func(command context.Context, owned *ownedTx) error {
		err := owned.native.QueryRowContext(command, query, args...).Scan(dest...)
		if errors.Is(err, sql.ErrNoRows) {
			return sql.ErrNoRows
		}
		if errors.Is(err, driver.ErrBadConn) {
			return errors.Join(operationError(err), owned.rollback(true))
		}
		return operationError(commandError(ctx, command, err))
	})
}
