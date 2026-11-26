// Package postgres implements GoEF's execution SPI with pgx's database/sql driver.
package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/igorpooh1978/GoEF/goeferr"
	"github.com/igorpooh1978/GoEF/provider/sqladapter"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/stdlib"
)

const providerID = "postgres"

// Backend is the GoEF PostgreSQL execution adapter.
type Backend = sqladapter.Backend

// Open creates an application-owned pgx pool in Exec query mode. It does not
// connect to PostgreSQL. Close the pool returned by Backend.DB when finished.
func Open(dsn string) (*Backend, error) {
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, goeferr.NewProviderFailure(goeferr.ErrConnectionUnavailable, providerID, goeferr.PhaseAcquire, "")
	}
	config.DefaultQueryExecMode = pgx.QueryExecModeExec
	return New(stdlib.OpenDB(*config))
}

// New binds a caller-owned database/sql pool built with the pgx stdlib driver.
// Existing driver settings remain in effect.
func New(db *sql.DB) (*Backend, error) {
	if db == nil {
		return nil, goeferr.NewInvalidState()
	}
	if _, ok := db.Driver().(*stdlib.Driver); !ok {
		return nil, goeferr.NewInvalidState()
	}
	return sqladapter.New(db, sqladapter.Config{
		ID: providerID, Classify: classify, ClosePhysical: closePhysical,
	})
}

func closePhysical(ctx context.Context, native any) error {
	// pgx's database/sql driver may return the connection to a separate
	// pgxpool.Pool on Close. End the native session before that can happen.
	conn, ok := native.(*stdlib.Conn)
	if !ok {
		return goeferr.ErrInvalidState
	}
	return conn.Conn().Close(ctx)
}

func classify(err error) (error, string) {
	var value *pgconn.PgError
	if !errors.As(err, &value) || value == nil {
		return nil, ""
	}
	var kind error
	switch value.Code {
	case "23505":
		kind = goeferr.ErrUniqueViolation
	case "23503":
		kind = goeferr.ErrForeignKeyViolation
	case "23502":
		kind = goeferr.ErrNotNullViolation
	case "23514":
		kind = goeferr.ErrCheckViolation
	case "40P01":
		kind = goeferr.ErrDeadlock
	case "40001":
		kind = goeferr.ErrSerializationFailure
	}
	return kind, value.Code
}
