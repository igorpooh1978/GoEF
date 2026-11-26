// Package sqlite applies reviewed, transactional migrations to SQLite.
// It uses database/sql without registering a database driver.
package sqlite

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"time"

	"github.com/igorpooh1978/GoEF/goeferr"
	"github.com/igorpooh1978/GoEF/migration"
)

// Queryer reads the migration's current, uncommitted database state.
type Queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// Session executes statements on the migration's exclusive connection.
type Session interface {
	Queryer
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

// Plan identifies a single reviewed artifact at the end of a linear registry.
type Plan struct {
	// ApplicationID must match the nonzero application_id of the caller's
	// main database. Use a dedicated file-backed migration pool.
	ApplicationID int32

	// Lineage includes the target as its last entry. Its digest must identify
	// the reviewed operations and checks. All earlier entries must be applied.
	Lineage []migration.Entry

	// CheckSource and CheckTarget verify the schema, relevant values and
	// dependency inventory expected before and after the reviewed change.
	// A false result leaves history unchanged and requires inspection.
	CheckSource func(context.Context, Queryer) (bool, error)
	CheckTarget func(context.Context, Queryer) (bool, error)

	// Run applies the reviewed change using only the provided session.
	Run func(context.Context, Session) error
}

// Apply takes a SQLite writer lock and applies one reviewed migration to the
// caller's main database. It commits the change and its history row atomically.
// A verified already-applied target returns false, nil. A confirmed new commit
// returns true, including if releasing the connection later fails. An
// unconfirmed commit returns ErrCommitOutcomeUnknown. Apply never retries.
//
// Supply a deadline and configure a bounded SQLite busy timeout for the pool.
// Keep the plan stable during the call. Callbacks are trusted, synchronous and
// must close their rows before returning. They must not retain the session,
// control transactions, change connection settings or write migration history.
// The checks must be read-only. Run must use transactional SQL and avoid
// effects outside this transaction. Apply validates the final foreign keys
// and restores the connection's original foreign-key setting before reuse.
func Apply(ctx context.Context, db *sql.DB, plan Plan) (applied bool, err error) {
	if ctx == nil || db == nil || plan.ApplicationID == 0 || len(plan.Lineage) == 0 ||
		plan.CheckSource == nil || plan.CheckTarget == nil || plan.Run == nil {
		return false, goeferr.NewInvalidState()
	}
	if _, err := migration.Pending(plan.Lineage, nil); err != nil {
		return false, err
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		return false, safeError(err)
	}
	discard := true
	foreignKeys := 0
	defer func() {
		if !discard {
			cleanup, cancel := cleanupContext(ctx)
			defer cancel()
			if restoreErr := setForeignKeys(cleanup, conn, foreignKeys); restoreErr != nil {
				err = errors.Join(err, safeError(restoreErr))
				discard = true
			}
		}
		err = errors.Join(err, release(conn, discard))
	}()

	if err := conn.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		return false, safeError(err)
	}
	if foreignKeys != 0 && foreignKeys != 1 {
		return false, goeferr.NewUnknownDriver()
	}
	if err := setForeignKeys(ctx, conn, 0); err != nil {
		return false, safeError(err)
	}
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return false, lockError(ctx, err)
	}
	rollback := true
	defer func() {
		if !rollback {
			return
		}
		cleanup, cancel := cleanupContext(ctx)
		defer cancel()
		if _, rollbackErr := conn.ExecContext(cleanup, "ROLLBACK"); rollbackErr != nil {
			err = errors.Join(err, goeferr.NewRollbackFailed())
			return
		}
		discard = false
	}()

	changed, err := runLocked(ctx, conn, plan)
	if !changed || err != nil {
		return false, err
	}
	if err := ctx.Err(); err != nil {
		return false, safeError(err)
	}

	// Once COMMIT is dispatched no cleanup may imply it was unsuccessful.
	rollback = false
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return false, errors.Join(goeferr.NewCommitOutcomeUnknown(), contextError(err))
	}
	discard = false
	return true, nil
}

func cleanupContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
}

func release(conn *sql.Conn, discard bool) error {
	if discard {
		// Returning ErrBadConn from Raw closes the physical connection. A
		// connection already removed by database/sql is also safe to discard.
		err := conn.Raw(func(any) error { return driver.ErrBadConn })
		if !errors.Is(err, driver.ErrBadConn) && !errors.Is(err, sql.ErrConnDone) {
			return safeError(err)
		}
	}
	if err := conn.Close(); err != nil && !errors.Is(err, sql.ErrConnDone) {
		return safeError(err)
	}
	return nil
}

func setForeignKeys(ctx context.Context, conn *sql.Conn, state int) error {
	setting := "PRAGMA foreign_keys=OFF"
	if state == 1 {
		setting = "PRAGMA foreign_keys=ON"
	}
	if _, err := conn.ExecContext(ctx, setting); err != nil {
		return err
	}
	var actual int
	if err := conn.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&actual); err != nil {
		return err
	}
	if actual != state {
		return goeferr.NewInvalidState()
	}
	return nil
}

func runLocked(ctx context.Context, conn *sql.Conn, plan Plan) (bool, error) {
	var applicationID int32
	if err := conn.QueryRowContext(ctx, "PRAGMA main.application_id").Scan(&applicationID); err != nil {
		return false, safeError(err)
	}
	if applicationID != plan.ApplicationID {
		return false, goeferr.NewMigrationRecoveryRequired()
	}
	if _, err := conn.ExecContext(ctx, createHistory); err != nil {
		return false, safeError(err)
	}
	if err := verifyHistoryTable(ctx, conn); err != nil {
		return false, err
	}
	applied, err := readHistory(ctx, conn, len(plan.Lineage))
	if err != nil {
		return false, err
	}
	pending, err := migration.Pending(plan.Lineage, applied)
	if err != nil {
		return false, err
	}
	switch len(pending) {
	case 0:
		if err := verify(ctx, conn, plan.CheckTarget); err != nil {
			return false, err
		}
		return false, checkForeignKeys(ctx, conn)
	case 1:
		if err := verify(ctx, conn, plan.CheckSource); err != nil {
			return false, err
		}
	default:
		return false, goeferr.NewMigrationConflict()
	}
	if err := plan.Run(ctx, conn); err != nil {
		return false, safeError(err)
	}
	if err := verify(ctx, conn, plan.CheckTarget); err != nil {
		return false, err
	}
	if err := recordHistory(ctx, conn, plan.Lineage); err != nil {
		return false, err
	}
	if err := checkForeignKeys(ctx, conn); err != nil {
		return false, err
	}
	return true, nil
}

func verify(ctx context.Context, conn *sql.Conn, check func(context.Context, Queryer) (bool, error)) error {
	matches, err := check(ctx, conn)
	if err != nil {
		return errors.Join(goeferr.NewMigrationRecoveryRequired(), safeError(err))
	}
	if !matches {
		return goeferr.NewMigrationRecoveryRequired()
	}
	return nil
}

func checkForeignKeys(ctx context.Context, conn *sql.Conn) (err error) {
	rows, err := conn.QueryContext(ctx, "PRAGMA main.foreign_key_check")
	if err != nil {
		return safeError(err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, safeError(closeErr))
		}
	}()
	if rows.Next() {
		return goeferr.NewMigrationRecoveryRequired()
	}
	if err := rows.Err(); err != nil {
		return safeError(err)
	}
	return nil
}

func lockError(ctx context.Context, err error) error {
	if cause := ctx.Err(); cause != nil {
		return safeError(cause)
	}
	// Both pinned modernc drivers expose SQLite's extended result code.
	var native interface{ Code() int }
	if errors.As(err, &native) {
		switch native.Code() & 0xff {
		case 5, 6: // SQLITE_BUSY, SQLITE_LOCKED
			return goeferr.NewMigrationLocked()
		}
	}
	return safeError(err)
}

func safeError(err error) error {
	if cause := contextError(err); cause != nil {
		return cause
	}
	return goeferr.NewUnknownDriver()
}

func contextError(err error) error {
	switch {
	case errors.Is(err, context.Canceled):
		return errors.Join(goeferr.NewCanceled(), context.Canceled)
	case errors.Is(err, context.DeadlineExceeded):
		return errors.Join(goeferr.NewDeadlineExceeded(), context.DeadlineExceeded)
	default:
		return nil
	}
}
