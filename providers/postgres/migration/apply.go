// Package migration applies reviewed, transactional PostgreSQL migrations.
package migration

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/igorpooh1978/GoEF/goeferr"
	"github.com/igorpooh1978/GoEF/migration"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Queryer reads on the bound migration transaction.
type Queryer interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

// Session reads and executes statements on the bound migration transaction.
type Session interface {
	Queryer
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

// Artifact is an immutable forward migration and its reviewed checks.
type Artifact struct {
	// Entry's digest identifies the reviewed operations and both checks.
	Entry migration.Entry
	// CheckSource verifies the schema and data before Run.
	CheckSource func(context.Context, Queryer) (bool, error)
	// Run executes the reviewed transactional change.
	Run func(context.Context, Session) error
	// CheckTarget verifies the new schema and data. Apply also calls the last
	// artifact's CheckTarget when all entries are already recorded.
	CheckTarget func(context.Context, Queryer) (bool, error)
}

// Plan identifies the database, history and ordered migration lineage.
type Plan struct {
	// Database must match the server's current database.
	Database string
	// HistorySchema must already exist. HistoryTable is reserved for this
	// executor. Supply exact PostgreSQL identifiers with up to 63 bytes.
	HistorySchema string
	HistoryTable  string
	// LockKey must be nonzero. Every cooperating migrator that shares this
	// history or changes conflicting objects must use the same database and key.
	LockKey int64
	// LockTimeout bounds the advisory lock wait independently of the caller.
	LockTimeout time.Duration
	// Artifacts contains the complete, nonempty registry in predecessor order.
	Artifacts []Artifact
}

// Apply locks the migration scope and commits the pending suffix and its
// history atomically in one PostgreSQL READ COMMITTED transaction. It returns
// the number of migrations in an acknowledged commit or zero on a verified
// no-op. An uncertain commit returns [goeferr.ErrCommitOutcomeUnknown] and requires
// independent inspection before another Apply. Apply never retries.
//
// Use a caller-owned, dedicated migration pool and bound the call with ctx.
// Keep the plan and callbacks stable until Apply returns. Callbacks execute
// synchronously on the bound transaction and must close rows and propagate
// SQL errors. They must leave transaction control, connection settings and
// history to Apply. Checks are read-only. Run may use only reviewed,
// transactional SQL and may not produce effects outside the transaction.
func Apply(ctx context.Context, pool *pgxpool.Pool, plan Plan) (count int, err error) {
	if ctx == nil || pool == nil {
		return 0, goeferr.NewInvalidState()
	}
	lineage, err := validatePlan(plan)
	if err != nil {
		return 0, err
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return 0, safeError(err)
	}
	defer conn.Release()
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted, AccessMode: pgx.ReadWrite})
	if err != nil {
		return 0, safeError(err)
	}
	defer func() {
		cleanup, cancel := cleanupContext(ctx)
		defer cancel()
		if rollbackErr := tx.Rollback(cleanup); rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			count = 0
			err = errors.Join(err, goeferr.NewRollbackFailed())
		}
	}()

	if err := lock(ctx, tx, plan); err != nil {
		return 0, err
	}
	count, err = runLocked(ctx, tx, plan, lineage)
	if err != nil || count == 0 {
		return 0, err
	}
	if err := ctx.Err(); err != nil {
		return 0, safeError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		if errors.Is(err, pgx.ErrTxCommitRollback) || rejectedCommit(conn.Conn(), err) {
			return 0, goeferr.NewCommitRolledBack()
		}
		cleanup, cancel := cleanupContext(ctx)
		defer cancel()
		// A pgxpool checkout is not eligible for reuse after an ambiguous commit.
		closeErr := conn.Hijack().Close(cleanup)
		return 0, errors.Join(goeferr.NewCommitOutcomeUnknown(), contextError(err), safeError(closeErr))
	}
	return count, nil
}

func validatePlan(plan Plan) ([]migration.Entry, error) {
	if plan.LockKey == 0 || plan.LockTimeout <= 0 || len(plan.Artifacts) == 0 ||
		!validIdentifier(plan.Database) || !validIdentifier(plan.HistorySchema) || !validIdentifier(plan.HistoryTable) {
		return nil, goeferr.NewInvalidState()
	}
	lineage := make([]migration.Entry, len(plan.Artifacts))
	for i, artifact := range plan.Artifacts {
		if artifact.CheckSource == nil || artifact.Run == nil || artifact.CheckTarget == nil {
			return nil, goeferr.NewInvalidState()
		}
		if strings.ContainsRune(artifact.Entry.ID, 0) {
			return nil, goeferr.NewMigrationConflict()
		}
		lineage[i] = artifact.Entry
	}
	if _, err := migration.Pending(lineage, nil); err != nil {
		return nil, err
	}
	return lineage, nil
}

func validIdentifier(name string) bool {
	return len(name) > 0 && len(name) <= 63 && utf8.ValidString(name) && !strings.ContainsRune(name, 0)
}

func lock(ctx context.Context, tx pgx.Tx, plan Plan) error {
	var database string
	if err := tx.QueryRow(ctx, "SELECT pg_catalog.current_database()").Scan(&database); err != nil {
		return safeError(err)
	}
	if database != plan.Database {
		return goeferr.NewMigrationRecoveryRequired()
	}
	wait, cancel := context.WithTimeoutCause(ctx, plan.LockTimeout, goeferr.ErrMigrationLocked)
	defer cancel()
	_, err := tx.Exec(wait, "SELECT pg_catalog.pg_advisory_xact_lock($1::bigint)", plan.LockKey)
	lockBudgetExpired := errors.Is(err, context.DeadlineExceeded) &&
		errors.Is(context.Cause(wait), goeferr.ErrMigrationLocked)
	var native *pgconn.PgError
	if lockBudgetExpired || (errors.As(err, &native) && native.Code == "55P03") {
		return goeferr.NewMigrationLocked()
	}
	return safeError(err)
}

func runLocked(ctx context.Context, tx pgx.Tx, plan Plan, lineage []migration.Entry) (int, error) {
	history := pgx.Identifier{plan.HistorySchema, plan.HistoryTable}.Sanitize()
	if _, err := tx.Exec(ctx, "CREATE TABLE IF NOT EXISTS "+history+historyDefinition); err != nil {
		return 0, safeError(err)
	}
	oid, err := historyOID(ctx, tx, history)
	if err != nil {
		return 0, err
	}
	applied, err := readHistory(ctx, tx, history, len(lineage))
	if err != nil {
		return 0, err
	}
	pending, err := migration.Pending(lineage, applied)
	if err != nil {
		return 0, err
	}
	query := struct{ Queryer }{tx}
	if len(pending) == 0 {
		return 0, verify(ctx, query, plan.Artifacts[len(plan.Artifacts)-1].CheckTarget)
	}

	session := struct{ Session }{tx}
	for i := len(applied); i < len(lineage); i++ {
		artifact := plan.Artifacts[i]
		if err := verify(ctx, query, artifact.CheckSource); err != nil {
			return 0, err
		}
		if err := artifact.Run(ctx, session); err != nil {
			return 0, safeError(err)
		}
		if err := verify(ctx, query, artifact.CheckTarget); err != nil {
			return 0, err
		}
		if err := recordHistory(ctx, tx, history, oid, i+1, lineage[i]); err != nil {
			return 0, err
		}
	}
	observed, err := readHistory(ctx, tx, history, len(lineage))
	if err != nil {
		return 0, err
	}
	if remaining, err := migration.Pending(lineage, observed); err != nil || len(remaining) != 0 {
		return 0, goeferr.NewMigrationRecoveryRequired()
	}
	return len(pending), nil
}

func verify(ctx context.Context, query Queryer, check func(context.Context, Queryer) (bool, error)) error {
	matches, err := check(ctx, query)
	if err != nil || !matches {
		return errors.Join(goeferr.NewMigrationRecoveryRequired(), safeError(err))
	}
	return nil
}

func cleanupContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
}

func rejectedCommit(conn *pgx.Conn, err error) bool {
	// PostgreSQL can reject COMMIT, for example when a deferred constraint
	// fails. An ERROR response followed by an idle connection confirms rollback.
	var native *pgconn.PgError
	return errors.As(err, &native) && native.SeverityUnlocalized == "ERROR" &&
		!conn.IsClosed() && conn.PgConn().TxStatus() == 'I'
}

func safeError(err error) error {
	if err == nil {
		return nil
	}
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
