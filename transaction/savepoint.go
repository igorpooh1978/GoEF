package transaction

import (
	"context"
	"crypto/rand"
	"database/sql/driver"
	"encoding/base32"
	"errors"
	"slices"
	"sync/atomic"

	"github.com/igorpooh1978/GoEF/goeferr"
)

// Dialect selects the savepoint syntax for the transaction's engine.
type Dialect string

const (
	// DialectPostgreSQL selects PostgreSQL savepoint syntax.
	DialectPostgreSQL Dialect = "postgres"
	// DialectSQLite selects SQLite savepoint syntax.
	DialectSQLite Dialect = "sqlite"
	// DialectSQLServer selects Transact-SQL savepoint syntax.
	DialectSQLServer Dialect = "sqlserver"
	// DialectMySQL selects MySQL savepoint syntax.
	DialectMySQL Dialect = "mysql"
)

// Savepoint identifies a point within one transaction. Copies refer to the
// same point and share its lifetime.
type Savepoint struct {
	scope savepointScope
	name  string
}

type savepointScope interface {
	withSavepoints(context.Context, func(*savepointStack, func(string) error) error) error
}

type savepointStack struct {
	dialect   Dialect
	live      []savepointMarker
	receipts  []*atomic.Bool
	uncertain bool
}

type savepointMarker struct {
	name  string
	saves int
}

// Savepoint creates a managed point on tx's physical connection using dialect.
// The caller supplies the engine dialect on the first creation and subsequent
// calls use that same dialect. Use only managed savepoint operations within the
// scope so that the stack reflects the native transaction.
func (tx *Tx) Savepoint(ctx context.Context, dialect Dialect) (*Savepoint, error) {
	return createSavepoint(ctx, tx, dialect)
}

func createSavepoint(ctx context.Context, scope savepointScope, dialect Dialect) (*Savepoint, error) {
	var point *Savepoint
	err := scope.withSavepoints(ctx, func(stack *savepointStack, exec func(string) error) error {
		if err := stack.validateDialect(dialect); err != nil {
			return err
		}

		create, _, _ := savepointSQL(dialect)
		name := newSavepointName()
		if err := stack.execute(exec, create+name); err != nil {
			return err
		}
		stack.dialect = dialect
		stack.live = append(stack.live, savepointMarker{name: name, saves: len(stack.receipts)})
		point = &Savepoint{scope: scope, name: name}
		return nil
	})
	return point, err
}

// Rollback rolls back to the point, preserving the point and invalidating all
// younger points. It leaves the transaction open.
func (s *Savepoint) Rollback(ctx context.Context) error {
	return s.apply(ctx, false)
}

// Release retires the innermost active point and keeps its writes. SQL Server
// has no native release operation, so release retires only the GoEF handle.
func (s *Savepoint) Release(ctx context.Context) error {
	return s.apply(ctx, true)
}

func (s *Savepoint) apply(ctx context.Context, release bool) error {
	if s == nil || s.scope == nil {
		return goeferr.NewInvalidState()
	}
	return s.scope.withSavepoints(ctx, func(stack *savepointStack, exec func(string) error) error {
		index := slices.IndexFunc(stack.live, func(marker savepointMarker) bool { return marker.name == s.name })
		if index < 0 || (release && index != len(stack.live)-1) {
			return goeferr.NewInvalidState()
		}

		create, prefix, releasePrefix := savepointSQL(stack.dialect)
		keep := index + 1
		if release {
			prefix = releasePrefix
			keep = index
		}
		if prefix != "" {
			if err := stack.execute(exec, prefix+s.name); err != nil {
				return err
			}
		}
		if !release && stack.dialect == DialectSQLServer {
			// Rolling back can consume the native point on SQL Server.
			// Recreate it at the restored position so the same GoEF
			// handle can be used for another rollback.
			if err := exec(create + s.name); err != nil {
				// Rollback already dispatched. A canceled re-arm also
				// leaves the managed stack without a confirmed target.
				stack.invalidate()
				if errors.Is(err, goeferr.ErrSavepointFailed) {
					return err
				}
				return errors.Join(goeferr.NewSavepointFailed(), err)
			}
		}
		if !release {
			stack.invalidateReceipts(stack.live[index].saves)
		}
		clear(stack.live[keep:])
		stack.live = stack.live[:keep]
		return nil
	})
}

func (s *savepointStack) execute(exec func(string) error, statement string) error {
	err := exec(statement)
	if errors.Is(err, goeferr.ErrSavepointFailed) {
		s.invalidate()
	}
	return err
}

func (s *savepointStack) invalidate() {
	s.invalidateReceipts(0)
	s.live = nil
	s.uncertain = true
}

func (s *savepointStack) invalidateReceipts(keep int) {
	for _, valid := range s.receipts[keep:] {
		valid.Store(false)
	}
	clear(s.receipts[keep:])
	s.receipts = s.receipts[:keep]
}

func (s *savepointStack) validateDialect(dialect Dialect) error {
	if s.uncertain {
		return goeferr.NewInvalidState()
	}
	switch dialect {
	case DialectPostgreSQL, DialectSQLite, DialectSQLServer, DialectMySQL:
	default:
		return goeferr.NewUnsupportedFeature()
	}
	if s.dialect != "" && s.dialect != dialect {
		return goeferr.NewUnsupportedFeature()
	}
	return nil
}

func savepointSQL(dialect Dialect) (create, rewind, release string) {
	if dialect == DialectSQLServer {
		return "SAVE TRANSACTION ", "ROLLBACK TRANSACTION ", ""
	}
	return "SAVEPOINT ", "ROLLBACK TO SAVEPOINT ", "RELEASE SAVEPOINT "
}

func newSavepointName() string {
	// The fixed-width ASCII name contains 128 random bits and fits SQL
	// Server's 32-character limit. It can be written as an identifier.
	var random [16]byte
	rand.Read(random[:])
	return "goef_" + base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(random[:])
}

func (tx *Tx) withSavepoints(ctx context.Context, run func(*savepointStack, func(string) error) error) error {
	return tx.withCommand(ctx, func(command context.Context, owned *ownedTx) error {
		return run(&owned.savepoints, func(statement string) error {
			if err := command.Err(); err != nil {
				return operationError(commandError(ctx, command, err))
			}
			_, err := owned.native.ExecContext(command, statement)
			if errors.Is(err, driver.ErrBadConn) {
				return errors.Join(savepointError(err), owned.rollback(true))
			}
			return savepointError(commandError(ctx, command, err))
		})
	})
}

func savepointError(err error) error {
	if err == nil {
		return nil
	}
	return errors.Join(goeferr.NewSavepointFailed(), contextCause(err))
}
