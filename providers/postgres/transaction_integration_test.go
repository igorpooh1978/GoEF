package postgres_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/igorpooh1978/GoEF/goeferr"
	postgres "github.com/igorpooh1978/GoEF/providers/postgres"
	"github.com/igorpooh1978/GoEF/transaction"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const nativeTxInsert = "INSERT INTO goef_native_tx_probe (id) VALUES ($1)"
const nativeTxCount = "SELECT COUNT(*) FROM goef_native_tx_probe WHERE id = $1"
const nativeCommitLock = 792456

const nativeDeferredCommit = `
CREATE OR REPLACE FUNCTION goef_hold_commit() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    PERFORM pg_advisory_xact_lock(792456);
    RETURN NULL;
END
$$;
DROP TRIGGER IF EXISTS goef_defer_commit ON goef_native_tx_probe;
CREATE CONSTRAINT TRIGGER goef_defer_commit AFTER INSERT ON goef_native_tx_probe
DEFERRABLE INITIALLY DEFERRED FOR EACH ROW WHEN (NEW.id = 3)
EXECUTE FUNCTION goef_hold_commit()
`

func TestNativeTransactions(t *testing.T) {
	marker := os.Getenv("GOEF_NATIVE_TX_FIXTURE")
	if marker == "" {
		t.Skip("start the private PostgreSQL instance using test/conformance/lab/postgres_native_tx.py")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 35*time.Second)
	defer cancel()

	config, err := pgxpool.ParseConfig("")
	if err != nil || config.ConnConfig.Host != "127.0.0.1" || config.ConnConfig.User != "goef_native_tx" ||
		config.ConnConfig.Database != "postgres" || len(config.ConnConfig.Fallbacks) != 0 {
		t.Fatal("invalid native PostgreSQL test configuration")
	}
	config.MaxConns = 1
	config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	observer, err := pgx.ConnectConfig(ctx, config.ConnConfig.Copy())
	if err != nil {
		t.Fatal("failed to open the isolated PostgreSQL observer")
	}
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer stop()
		_ = observer.Close(cleanup)
	})
	var version int
	var observedMarker string
	if err := observer.QueryRow(ctx, "SELECT current_setting('server_version_num')::int, current_setting('goef.native_tx_fixture')").
		Scan(&version, &observedMarker); err != nil || version != 180007 || observedMarker != marker {
		t.Fatal("isolated observer did not match the PostgreSQL 18.7 fixture")
	}
	if _, err := observer.Exec(ctx, "CREATE TABLE IF NOT EXISTS goef_native_tx_probe (id integer PRIMARY KEY)"); err != nil {
		t.Fatal("failed to create the transaction fixture table")
	}

	if _, err := observer.Exec(ctx, nativeDeferredCommit); err != nil {
		t.Fatal("failed to arm the deferred transaction fixture")
	}

	t.Run("TX-PG-NATIVE-001-S01", func(t *testing.T) {
		blocker := newTransactionControlTracer("begin", true)
		pool := nativeTestPool(t, ctx, config, blocker)
		held, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatal("could not reserve the only pool slot")
		}
		wait, stop := context.WithTimeout(ctx, 75*time.Millisecond)
		tx, err := postgres.BeginTx(wait, pool, sql.TxOptions{})
		stop()
		held.Release()
		if tx != nil || !errors.Is(err, context.DeadlineExceeded) || !errors.Is(err, goeferr.ErrDeadlineExceeded) || blocker.calls.Load() != 0 {
			t.Fatalf("queued BeginTx = %v, %v", tx, err)
		}

		lifetime, stop := context.WithTimeout(ctx, time.Second)
		defer stop()
		beginErr := make(chan error, 1)
		go func() {
			tx, err := postgres.BeginTx(lifetime, pool, sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
			if tx != nil {
				_ = tx.Close()
				err = errors.New("canceled BeginTx returned a handle")
			}
			beginErr <- err
		}()
		waitForNativeControl(t, ctx, blocker)
		if pool.Stat().AcquiredConns() != 1 {
			t.Fatal("native BEGIN must reserve the connection before cancellation")
		}
		if err := awaitNativeError(t, ctx, beginErr); !errors.Is(err, context.DeadlineExceeded) || !errors.Is(err, goeferr.ErrDeadlineExceeded) {
			t.Fatalf("native BEGIN lost the deadline cause: %v", err)
		}
		if err := awaitNativeError(t, ctx, blocker.observed); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("native BEGIN did not observe caller's deadline: %v", err)
		}
		checkNativePool(t, ctx, pool)

		for _, option := range []struct {
			level sql.IsolationLevel
			name  string
		}{
			{sql.LevelReadCommitted, "read committed"},
			{sql.LevelRepeatableRead, "repeatable read"},
			{sql.LevelSerializable, "serializable"},
		} {
			tx, err := postgres.BeginTx(ctx, pool, sql.TxOptions{Isolation: option.level, ReadOnly: true})
			if err != nil {
				t.Fatalf("fresh native BEGIN with %s failed: %v", option.name, err)
			}
			t.Cleanup(func() { _ = tx.Close() })
			var isolation, readOnly string
			if err := tx.Scan(ctx, "SELECT current_setting('transaction_isolation'), current_setting('transaction_read_only')", nil, &isolation, &readOnly); err != nil || isolation != option.name || readOnly != "on" {
				t.Fatalf("native isolation or access mode differs: %v", err)
			}
			if err := tx.Rollback(); err != nil || tx.State() != transaction.StateRolledBack {
				t.Fatalf("native rollback failed: %v, %s", err, tx.State())
			}
			checkNativePool(t, ctx, pool)
		}
	})

	t.Run("TX-PG-NATIVE-001-S02", func(t *testing.T) {
		pool := nativeTestPool(t, ctx, config, nil)
		const lock = 792455
		holdNativeAdvisoryLock(t, ctx, observer, lock)

		lifetime, stop := context.WithCancel(ctx)
		defer stop()
		tx, err := postgres.BeginTx(lifetime, pool, sql.TxOptions{})
		if err != nil {
			t.Fatalf("could not begin the owned transaction: %v", err)
		}
		t.Cleanup(func() { _ = tx.Close() })
		var pid int
		if err := tx.Scan(ctx, "SELECT pg_backend_pid()", nil, &pid); err != nil || pid == 0 {
			t.Fatalf("owned transaction could not read its backend identity: %v", err)
		}
		if _, err := tx.Exec(ctx, nativeTxInsert, 2); err != nil {
			t.Fatalf("owned transaction could not write: %v", err)
		}
		queryErr := make(chan error, 1)
		go func() {
			var result bool
			queryErr <- tx.Scan(ctx, "SELECT true FROM pg_advisory_xact_lock($1)", []any{lock}, &result)
		}()
		waitForAdvisoryLock(t, ctx, observer, pid)
		if pool.Stat().AcquiredConns() != 1 {
			t.Fatal("owned transaction did not reserve the pool connection")
		}
		stop()
		if err := awaitNativeError(t, ctx, queryErr); !errors.Is(err, context.Canceled) || !errors.Is(err, goeferr.ErrCanceled) {
			t.Fatalf("in-flight cancellation lost its cause: %v", err)
		}
		// The default pgx cancellation watcher can close a busy connection
		// before an explicit rollback is acknowledged. Either outcome must
		// consume the checkout and must leave no committed writes.
		rollbackErr := tx.Close()
		switch tx.State() {
		case transaction.StateRolledBack:
			if rollbackErr != nil {
				t.Fatalf("confirmed rollback reported an error: %v", rollbackErr)
			}
		case transaction.StateFailed:
			if !errors.Is(rollbackErr, goeferr.ErrRollbackFailed) {
				t.Fatalf("failed rollback lost its typed cause: %v", rollbackErr)
			}
		default:
			t.Fatalf("cancellation left transaction state %s", tx.State())
		}
		checkNativePool(t, ctx, pool)
		checkNativeCount(t, ctx, observer, 2)
	})

	t.Run("TX-PG-NATIVE-001-S03", func(t *testing.T) {
		tracer := newTransactionControlTracer("commit", false)
		pool := nativeTestPool(t, ctx, config, tracer)
		holdNativeAdvisoryLock(t, ctx, observer, nativeCommitLock)
		lifetime, stop := context.WithCancel(ctx)
		defer stop()
		tx, err := postgres.BeginTx(lifetime, pool, sql.TxOptions{})
		if err != nil {
			t.Fatalf("could not begin the transaction: %v", err)
		}
		t.Cleanup(func() { _ = tx.Close() })
		var oldPID int
		if err := tx.Scan(ctx, "SELECT pg_backend_pid()", nil, &oldPID); err != nil {
			t.Fatalf("could not observe the original backend: %v", err)
		}
		if _, err := tx.Exec(ctx, nativeTxInsert, 3); err != nil {
			t.Fatalf("could not write the pending row: %v", err)
		}
		commitErr := make(chan error, 1)
		go func() { commitErr <- tx.Commit() }()
		// This lock can only be requested by the deferred trigger at commit.
		waitForAdvisoryLock(t, ctx, observer, oldPID)
		stop()
		err = awaitNativeError(t, ctx, commitErr)
		if !errors.Is(err, goeferr.ErrCommitOutcomeUnknown) || !errors.Is(err, context.Canceled) ||
			tx.State() != transaction.StateOutcomeUnknown || tracer.calls.Load() != 1 {
			t.Fatalf("failed native commit = %v, state = %s, attempts = %d", err, tx.State(), tracer.calls.Load())
		}
		checkNativePool(t, ctx, pool)
		var nextPID int
		if err := pool.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&nextPID); err != nil || oldPID == nextPID {
			t.Fatalf("unknown commit returned its backend to the pool: %v", err)
		}
		waitForBackend(t, ctx, observer, oldPID, "SELECT NOT EXISTS (SELECT 1 FROM pg_stat_activity WHERE pid = $1)")
		checkNativeCount(t, ctx, observer, 3)
	})

	t.Run("TX-PG-NATIVE-001-S04", func(t *testing.T) {
		pool := nativeTestPool(t, ctx, config, nil)
		native, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal("owner could not begin the native transaction")
		}
		t.Cleanup(func() {
			cleanup, stop := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer stop()
			_ = native.Rollback(cleanup)
		})
		external, err := postgres.NewExternal(native)
		if err != nil {
			t.Fatalf("could not coordinate the native transaction: %v", err)
		}
		t.Cleanup(func() { _ = external.Close() })
		participant, err := external.Enlist()
		if err != nil {
			t.Fatalf("could not enlist: %v", err)
		}
		result, err := participant.Exec(ctx, nativeTxInsert, 4)
		if err != nil {
			t.Fatalf("participant could not insert: %v", err)
		}
		if count, err := result.RowsAffected(); err != nil || count != 1 {
			t.Fatalf("participant row count = %d, %v", count, err)
		}
		probe, stop := context.WithCancel(ctx)
		stop()
		var id int
		if err := participant.Scan(probe, "SELECT id FROM goef_native_tx_probe WHERE id = $1", []any{4}, &id); !errors.Is(err, context.Canceled) || !errors.Is(err, goeferr.ErrCanceled) {
			t.Fatalf("canceled participant scan = %v", err)
		}
		if err := participant.Scan(ctx, "SELECT id FROM goef_native_tx_probe WHERE id = $1", []any{4}, &id); err != nil || id != 4 {
			t.Fatalf("participant cannot read caller's pending row: %d, %v", id, err)
		}
		checkNativeCount(t, ctx, observer, 4)
		point, err := external.Savepoint(ctx, transaction.DialectPostgreSQL)
		if err != nil {
			t.Fatalf("coordinator could not create a savepoint: %v", err)
		}
		saved, err := participant.SaveChanges(ctx, transaction.SaveOptions{Dialect: transaction.DialectPostgreSQL, Savepoints: true}, func(writer *transaction.SaveWriter) error {
			_, err := writer.Exec(nativeTxInsert, 44)
			return err
		})
		if err != nil {
			t.Fatalf("participant could not save a batch: %v", err)
		}
		if err := point.Rollback(ctx); err != nil {
			t.Fatalf("coordinator could not rewind its savepoint: %v", err)
		}
		if err := saved.Accept(); !errors.Is(err, goeferr.ErrInvalidState) {
			t.Fatalf("rewound receipt remained eligible for acceptance: %v", err)
		}
		if err := participant.Scan(ctx, "SELECT id FROM goef_native_tx_probe WHERE id = $1", []any{44}, &id); !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("rewound row remained visible: %v", err)
		}
		if err := external.Close(); err != nil {
			t.Fatalf("could not detach external coordinator: %v", err)
		}
		if err := participant.Scan(ctx, "SELECT 1", nil, &id); !errors.Is(err, goeferr.ErrInvalidState) {
			t.Fatalf("detached participant remained available: %v", err)
		}
		var count int
		if err := native.QueryRow(ctx, nativeTxCount, 4).Scan(&count); err != nil || count != 1 || pool.Stat().AcquiredConns() != 1 {
			t.Fatalf("detaching changed native ownership or pending state: %d, %v", count, err)
		}
		if err := native.Rollback(ctx); err != nil {
			t.Fatal("native owner could not roll back after GoEF detached")
		}
		checkNativePool(t, ctx, pool)
		checkNativeCount(t, ctx, observer, 4)
		checkNativeAlreadyCompleted(t, ctx, native)
	})

	t.Run("TX-PG-NATIVE-001-S05", func(t *testing.T) {
		pool := nativeTestPool(t, ctx, config, nil)
		tx, err := postgres.BeginTx(ctx, pool, sql.TxOptions{})
		if err != nil {
			t.Fatalf("could not begin the transaction: %v", err)
		}
		t.Cleanup(func() { _ = tx.Close() })
		var oldPID int
		if err := tx.Scan(ctx, "SELECT pg_backend_pid()", nil, &oldPID); err != nil {
			t.Fatalf("could not observe the original backend: %v", err)
		}
		if _, err := tx.Exec(ctx, nativeTxInsert, 5); err != nil {
			t.Fatalf("could not write the pending row: %v", err)
		}
		if _, err := tx.Exec(ctx, "SELECT 1 / 0"); err == nil {
			t.Fatal("negative control did not put PostgreSQL in an aborted transaction")
		}
		err = tx.Commit()
		if !errors.Is(err, goeferr.ErrCommitRolledBack) || errors.Is(err, pgx.ErrTxCommitRollback) ||
			tx.State() != transaction.StateRolledBack {
			t.Fatalf("confirmed native rollback = %v, state = %s", err, tx.State())
		}
		checkNativePool(t, ctx, pool)
		var nextPID int
		if err := pool.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&nextPID); err != nil || oldPID != nextPID {
			t.Fatalf("confirmed rollback lost an idle backend: %v", err)
		}
		checkNativeCount(t, ctx, observer, 5)

		fresh, err := postgres.BeginTx(ctx, pool, sql.TxOptions{})
		if err != nil {
			t.Fatalf("fresh native transaction could not begin: %v", err)
		}
		t.Cleanup(func() { _ = fresh.Close() })
		if err := fresh.Commit(); err != nil || fresh.State() != transaction.StateCommitted {
			t.Fatalf("fresh native transaction could not commit: %v, %s", err, fresh.State())
		}
	})
}

func checkNativeAlreadyCompleted(t *testing.T, ctx context.Context, native pgx.Tx) {
	t.Helper()
	closed, err := postgres.NewExternal(native)
	if err != nil {
		t.Fatalf("could not build coordinator to test completion detection: %v", err)
	}
	defer closed.Close()
	participant, err := closed.Enlist()
	if err != nil {
		t.Fatalf("could not create participant to observe completion: %v", err)
	}
	var id int
	if err := participant.Scan(ctx, "SELECT 1", nil, &id); !errors.Is(err, goeferr.ErrInvalidState) {
		t.Fatalf("participant did not detect native completion: %v", err)
	}
	if _, err := closed.Enlist(); !errors.Is(err, goeferr.ErrInvalidState) {
		t.Fatalf("external coordinator still accepted an enlistment after native completion: %v", err)
	}
}

func nativeTestPool(t *testing.T, ctx context.Context, config *pgxpool.Config, tracer pgx.QueryTracer) *pgxpool.Pool {
	t.Helper()
	copy := config.Copy()
	copy.ConnConfig.Tracer = tracer
	pool, err := pgxpool.NewWithConfig(ctx, copy)
	if err != nil {
		t.Fatal("could not create the isolated pgx pool")
	}
	t.Cleanup(pool.Close)
	return pool
}

func checkNativePool(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	if pool.Stat().AcquiredConns() != 0 {
		t.Fatal("transaction left the pool checkout in use")
	}
	if err := pool.Ping(ctx); err != nil {
		t.Fatal("the caller-owned pgx pool stopped working")
	}
}

func checkNativeCount(t *testing.T, ctx context.Context, observer *pgx.Conn, id int) {
	t.Helper()
	var count int
	if err := observer.QueryRow(ctx, nativeTxCount, id).Scan(&count); err != nil || count != 0 {
		t.Fatal("the observer found an unexpected row")
	}
}

func holdNativeAdvisoryLock(t *testing.T, ctx context.Context, observer *pgx.Conn, lock int) {
	t.Helper()
	if _, err := observer.Exec(ctx, "SELECT pg_advisory_lock($1)", lock); err != nil {
		t.Fatal("observer could not reserve the advisory lock")
	}
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer stop()
		_, _ = observer.Exec(cleanup, "SELECT pg_advisory_unlock($1)", lock)
	})
}

func waitForAdvisoryLock(t *testing.T, ctx context.Context, observer *pgx.Conn, pid int) {
	t.Helper()
	waitForBackend(t, ctx, observer, pid, "SELECT EXISTS (SELECT 1 FROM pg_locks WHERE pid = $1 AND locktype = 'advisory' AND NOT granted)")
}

func waitForBackend(t *testing.T, ctx context.Context, observer *pgx.Conn, pid int, query string) {
	t.Helper()
	wait, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var observed bool
		if err := observer.QueryRow(wait, query, pid).Scan(&observed); err != nil {
			t.Fatal("could not check the owning backend state")
		}
		if observed {
			return
		}
		select {
		case <-wait.Done():
			t.Fatal("did not observe the expected backend state")
		case <-ticker.C:
		}
	}
}

type transactionControlTracer struct {
	sqlPrefix string
	pause     bool
	calls     atomic.Int32
	entered   chan struct{}
	observed  chan error
}

func newTransactionControlTracer(prefix string, pause bool) *transactionControlTracer {
	return &transactionControlTracer{sqlPrefix: prefix, pause: pause, entered: make(chan struct{}), observed: make(chan error, 1)}
}

func (b *transactionControlTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if !strings.HasPrefix(data.SQL, b.sqlPrefix) || b.calls.Add(1) != 1 || !b.pause {
		return ctx
	}
	close(b.entered)
	select {
	case <-ctx.Done():
		b.observed <- ctx.Err()
	case <-time.After(5 * time.Second):
		b.observed <- errors.New("native control did not receive the caller's context")
	}
	return ctx
}

func (*transactionControlTracer) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func waitForNativeControl(t *testing.T, ctx context.Context, blocker *transactionControlTracer) {
	t.Helper()
	select {
	case <-blocker.entered:
	case <-ctx.Done():
		t.Fatal("pgx did not start the expected transaction control command")
	}
}

func awaitNativeError(t *testing.T, ctx context.Context, result <-chan error) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-ctx.Done():
		t.Fatal("native transaction command did not return")
		return ctx.Err()
	}
}
