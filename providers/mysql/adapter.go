// Package mysql implements GoEF's execution SPI for MySQL/InnoDB.
package mysql

import (
	"database/sql"
	"errors"
	"strconv"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/igorpooh1978/GoEF/goeferr"
	"github.com/igorpooh1978/GoEF/provider/sqladapter"
)

const providerID = "mysql"

// Backend is the GoEF MySQL execution adapter.
type Backend = sqladapter.Backend

// Open creates an application-owned MySQL pool with temporal decoding enabled.
// It does not connect to MySQL. Close the pool returned by Backend.DB when finished.
func Open(dsn string) (*Backend, error) {
	config, err := mysqlDriver.ParseDSN(dsn)
	if err != nil {
		return nil, connectionFailure()
	}
	config.ParseTime = true
	connector, err := mysqlDriver.NewConnector(config)
	if err != nil {
		return nil, connectionFailure()
	}
	return New(sql.OpenDB(connector))
}

func connectionFailure() error {
	return goeferr.NewProviderFailure(goeferr.ErrConnectionUnavailable, providerID, goeferr.PhaseAcquire, "")
}

// New binds a caller-owned Go MySQL Driver pool. A pool used for timestamp
// transport must set ParseTime in the driver configuration.
func New(db *sql.DB) (*Backend, error) {
	if db == nil {
		return nil, goeferr.NewInvalidState()
	}
	switch db.Driver().(type) {
	case *mysqlDriver.MySQLDriver, mysqlDriver.MySQLDriver:
	default:
		return nil, goeferr.NewInvalidState()
	}
	return sqladapter.New(db, sqladapter.Config{ID: providerID, Classify: classify, IntegerBool: true})
}

func classify(err error) (error, string) {
	var value *mysqlDriver.MySQLError
	if !errors.As(err, &value) || value == nil {
		return nil, ""
	}
	var kind error
	switch value.Number {
	case 1062:
		kind = goeferr.ErrUniqueViolation
	case 1451, 1452:
		kind = goeferr.ErrForeignKeyViolation
	case 1213:
		kind = goeferr.ErrDeadlock
	case 1205:
		kind = goeferr.ErrTimeout
	}
	return kind, strconv.FormatUint(uint64(value.Number), 10)
}
