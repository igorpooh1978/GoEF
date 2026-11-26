// Package sqlite implements GoEF's execution SPI with the pure-Go SQLite driver.
package sqlite

import (
	"database/sql"
	"errors"
	"strconv"

	"github.com/igorpooh1978/GoEF/goeferr"
	"github.com/igorpooh1978/GoEF/provider/sqladapter"
	sqliteDriver "modernc.org/sqlite"
	"modernc.org/sqlite/lib"
)

const providerID = "sqlite"

// Backend is the GoEF SQLite execution adapter.
type Backend = sqladapter.Backend

// Open creates an application-owned SQLite pool for a file path or driver URI.
// The driver opens the database on first acquisition. Configure required
// connection PRAGMAs through the URI. Close Backend.DB when finished.
func Open(dsn string) (*Backend, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, goeferr.NewProviderFailure(goeferr.ErrConnectionUnavailable, providerID, goeferr.PhaseAcquire, "")
	}
	return New(db)
}

// New binds a caller-owned pool using modernc.org/sqlite.
func New(db *sql.DB) (*Backend, error) {
	if db == nil {
		return nil, goeferr.NewInvalidState()
	}
	if _, ok := db.Driver().(*sqliteDriver.Driver); !ok {
		return nil, goeferr.NewInvalidState()
	}
	return sqladapter.New(db, sqladapter.Config{ID: providerID, Classify: classify, IntegerBool: true})
}

func classify(err error) (error, string) {
	var value *sqliteDriver.Error
	if !errors.As(err, &value) || value == nil {
		return nil, ""
	}
	var kind error
	code := value.Code()
	switch code {
	case sqlite3.SQLITE_CONSTRAINT_UNIQUE, sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY:
		kind = goeferr.ErrUniqueViolation
	case sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY:
		kind = goeferr.ErrForeignKeyViolation
	case sqlite3.SQLITE_CONSTRAINT_NOTNULL:
		kind = goeferr.ErrNotNullViolation
	case sqlite3.SQLITE_CONSTRAINT_CHECK:
		kind = goeferr.ErrCheckViolation
	case sqlite3.SQLITE_BUSY_SNAPSHOT:
		kind = goeferr.ErrSerializationFailure
	case sqlite3.SQLITE_BUSY, sqlite3.SQLITE_LOCKED:
		kind = goeferr.ErrLockUnavailable
	}
	return kind, strconv.Itoa(code)
}
