// Package sqlserver implements GoEF's execution SPI with go-mssqldb.
package sqlserver

import (
	"database/sql"
	"errors"
	"strconv"

	"github.com/igorpooh1978/GoEF/goeferr"
	"github.com/igorpooh1978/GoEF/provider/sqladapter"
	mssql "github.com/microsoft/go-mssqldb"
)

const providerID = "sqlserver"

// Backend is the GoEF SQL Server execution adapter.
type Backend = sqladapter.Backend

// Open creates an application-owned SQL Server pool. It does not connect to
// SQL Server. Close the pool returned by Backend.DB when finished.
func Open(dsn string) (*Backend, error) {
	connector, err := mssql.NewConnector(dsn)
	if err != nil {
		return nil, goeferr.NewProviderFailure(goeferr.ErrConnectionUnavailable, providerID, goeferr.PhaseAcquire, "")
	}
	return New(sql.OpenDB(connector))
}

// New binds a caller-owned SQL Server pool built with go-mssqldb.
func New(db *sql.DB) (*Backend, error) {
	if db == nil {
		return nil, goeferr.NewInvalidState()
	}
	if _, ok := db.Driver().(*mssql.Driver); !ok {
		return nil, goeferr.NewInvalidState()
	}
	return sqladapter.New(db, sqladapter.Config{ID: providerID, Classify: classify})
}

func classify(err error) (error, string) {
	var value mssql.Error
	if !errors.As(err, &value) {
		return nil, ""
	}
	number := value.Number
	kind := classifyNumber(number)
	if len(value.All) > 0 {
		// 3621 reports that the statement has been terminated. Ignore it when
		// deciding which of the preceding errors caused the failure.
		number, kind = 0, nil
		for _, item := range value.All {
			if item.Number == 3621 {
				continue
			}
			current := classifyNumber(item.Number)
			if number != 0 && (current != kind || current == nil && item.Number != number) {
				return nil, ""
			}
			number, kind = item.Number, current
		}
		if number == 0 {
			number = value.Number
		}
	}
	return kind, strconv.FormatInt(int64(number), 10)
}

func classifyNumber(number int32) error {
	switch number {
	case 2601, 2627:
		return goeferr.ErrUniqueViolation
	case 1205:
		return goeferr.ErrDeadlock
	default:
		// 547 may mean FOREIGN KEY or CHECK. The frozen transport has no
		// verified schema context that can disambiguate the two.
		return nil
	}
}
