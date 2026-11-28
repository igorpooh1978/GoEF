package driverlab

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/igorpooh1978/GoEF/goeferr"
	"github.com/igorpooh1978/GoEF/migration"
	migrations "github.com/igorpooh1978/GoEF/providers/postgres/migration"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresMigrations(t *testing.T) {
	if os.Getenv("GOEF_DRIVERLAB_POSTGRES_MIGRATIONS") != "1" {
		t.Skip("select this test with the isolated postgres_migrations.py runner")
	}
	if os.Getenv("GOEF_DRIVERLAB_PROVIDER") != "postgres" || os.Getenv("GOEF_DRIVERLAB_PROFILE") != "postgres-18" {
		t.Fatal("migration scenarios require the pinned PostgreSQL 18 profile")
	}
	setup := openPostgres(t)
	ctx, cancel := context.WithTimeout(t.Context(), 35*time.Second)
	defer cancel()
	s := scenario{fixture: setup, t: t, provider: "postgres", ctx: ctx}
	s.verifyMarker(ctx, setup.db.QueryContext)

	config := postgresMigrationPoolConfig(t)
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal("could not create migration fixture pool")
	}
	defer pool.Close()
	for i, test := range []struct {
		name string
		run  func(postgresMigrationFixture)
	}{
		{"MIG-POSTGRES-001-S01", postgresMigrationApply},
		{"MIG-POSTGRES-001-S02", postgresMigrationConflicts},
		{"MIG-POSTGRES-001-S03", postgresMigrationRollback},
		{"MIG-POSTGRES-001-S04", postgresMigrationContention},
		{"MIG-POSTGRES-001-S05", postgresMigrationCancel},
		{"MIG-POSTGRES-001-S06", postgresMigrationCommitFault},
	} {
		passed := t.Run(test.name, func(t *testing.T) {
			fixture := postgresMigrationFixture{
				t: t, ctx: ctx, pool: pool, config: config,
				table: fmt.Sprintf("public.goef_migration_data_%d", i), key: int64(350001 + i),
				history: fmt.Sprintf("goef_migration_history_%d", i),
			}
			test.run(fixture)
			waitNextProbe(t, ctx, "migration checkout release", func(context.Context) (bool, error) {
				return pool.Stat().AcquiredConns() == 0, nil
			})
		})
		if passed {
			report, err := json.Marshal(map[string]string{
				"provider": "postgres", "profile": "postgres-18", "engine_version": setup.facts["engine_version"],
				"scenario": test.name, "status": "PASS",
			})
			if err != nil {
				t.Fatal("could not encode migration scenario evidence")
			}
			fmt.Printf("GOEF_POSTGRES_MIGRATION_RESULT=%s\n", report)
		}
	}
}

func postgresMigrationPoolConfig(t *testing.T) *pgxpool.Config {
	t.Helper()
	const options = "host=127.0.0.1 port=5432 dbname=goef_smoke user=goef_driver password=placeholder " +
		"connect_timeout=5 sslmode=disable sslcertmode=disable sslrootcert='' sslnegotiation=postgres " +
		"target_session_attrs=any min_protocol_version=3.0 max_protocol_version=3.0 " +
		"channel_binding=disable require_auth=scram-sha-256"
	config, err := pgxpool.ParseConfig(options)
	if err != nil || config.ConnConfig.Host != "127.0.0.1" || config.ConnConfig.Port != 5432 ||
		config.ConnConfig.TLSConfig != nil || len(config.ConnConfig.Fallbacks) != 0 {
		t.Fatal("migration pool target did not match the fixed lab endpoint")
	}
	config.ConnConfig.Password = os.Getenv("GOEF_DRIVERLAB_PASSWORD")
	config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	config.ConnConfig.RuntimeParams = map[string]string{"application_name": "goef_migration_fixture"}
	config.MaxConns = 4
	return config
}

type postgresMigrationFixture struct {
	t       *testing.T
	ctx     context.Context
	pool    *pgxpool.Pool
	config  *pgxpool.Config
	table   string
	history string
	key     int64
}

func (s postgresMigrationFixture) plan() migrations.Plan {
	create := "CREATE TABLE " + s.table + " (id integer PRIMARY KEY, note text NOT NULL)"
	seed := "INSERT INTO " + s.table + " VALUES ($1, $2)"
	initial := migration.Entry{ID: "001_create", Digest: sha256.Sum256([]byte(create))}
	return migrations.Plan{
		Database: "goef_smoke", HistorySchema: "public", HistoryTable: s.history,
		LockKey: s.key, LockTimeout: 5 * time.Second,
		Artifacts: []migrations.Artifact{
			{
				Entry:       initial,
				CheckSource: s.tableAbsent,
				Run: func(ctx context.Context, session migrations.Session) error {
					_, err := session.Exec(ctx, create)
					return err
				},
				CheckTarget: s.rowsEqual(0),
			},
			{
				Entry:       migration.Entry{ID: "002_seed", Predecessor: initial.ID, Digest: sha256.Sum256([]byte(seed + "|1|seed"))},
				CheckSource: s.rowsEqual(0),
				Run: func(ctx context.Context, session migrations.Session) error {
					_, err := session.Exec(ctx, seed, 1, "seed")
					return err
				},
				CheckTarget: s.rowsEqual(1),
			},
		},
	}
}

func (s postgresMigrationFixture) tableAbsent(ctx context.Context, q migrations.Queryer) (bool, error) {
	var absent bool
	err := q.QueryRow(ctx, "SELECT pg_catalog.to_regclass($1) IS NULL", s.table).Scan(&absent)
	return absent, err
}

func (s postgresMigrationFixture) rowsEqual(want int) func(context.Context, migrations.Queryer) (bool, error) {
	return func(ctx context.Context, q migrations.Queryer) (bool, error) {
		var count int
		var intact bool
		err := q.QueryRow(ctx, "SELECT count(*), coalesce(bool_and(id = 1 AND note = 'seed'), true) FROM "+s.table).Scan(&count, &intact)
		return count == want && intact, err
	}
}

func (s postgresMigrationFixture) assertApplied(plan migrations.Plan, want int) {
	s.t.Helper()
	s.assertApplyResult(plan, want, nil)
}

func (s postgresMigrationFixture) assertApplyResult(plan migrations.Plan, want int, wantErr error) {
	s.t.Helper()
	n, err := migrations.Apply(s.ctx, s.pool, plan)
	if n != want || !errors.Is(err, wantErr) {
		s.t.Fatalf("apply returned count %d and error %v, want count %d and %v", n, err, want, wantErr)
	}
}

func (s postgresMigrationFixture) assertRows(want int) {
	s.t.Helper()
	if matches, err := s.rowsEqual(want)(s.ctx, s.pool); err != nil || !matches {
		s.t.Fatal("migration data rows did not match the expected durable state")
	}
}

func (s postgresMigrationFixture) assertHistory(want []migrations.Artifact) {
	s.t.Helper()
	rows, err := s.pool.Query(s.ctx, "SELECT position, id, predecessor, digest FROM public."+s.history+" ORDER BY position")
	if err != nil {
		s.t.Fatal("could not read fixture history")
	}
	defer rows.Close()
	for i, artifact := range want {
		var position int
		var id, predecessor string
		var digest []byte
		entry := artifact.Entry
		if !rows.Next() || rows.Scan(&position, &id, &predecessor, &digest) != nil ||
			position != i+1 || id != entry.ID || predecessor != entry.Predecessor || !bytes.Equal(digest, entry.Digest[:]) {
			s.t.Fatal("durable history did not match the ordered artifact")
		}
	}
	if rows.Next() || rows.Err() != nil {
		s.t.Fatal("migration history contained an unexpected row")
	}
}

func (s postgresMigrationFixture) assertNoRelation(name string) {
	s.t.Helper()
	var absent bool
	if err := s.pool.QueryRow(s.ctx, "SELECT pg_catalog.to_regclass($1) IS NULL", name).Scan(&absent); err != nil || !absent {
		s.t.Fatal("unexpected durable fixture relation")
	}
}

func postgresMigrationApply(s postgresMigrationFixture) {
	plan := s.plan()
	first := plan
	first.Artifacts = plan.Artifacts[:1]
	s.assertApplied(first, 1)
	s.assertHistory(first.Artifacts)
	s.assertRows(0)

	s.assertApplied(plan, 1)
	s.assertRows(1)
	s.assertHistory(plan.Artifacts)
	s.assertApplied(plan, 0)
	s.assertHistory(plan.Artifacts)
}

func postgresMigrationConflicts(s postgresMigrationFixture) {
	for _, test := range []struct {
		name   string
		change func(*migrations.Plan)
		want   error
	}{
		{"missing lock budget", func(p *migrations.Plan) { p.LockTimeout = 0 }, goeferr.ErrInvalidState},
		{"truncated identifier", func(p *migrations.Plan) { p.HistoryTable = strings.Repeat("x", 64) }, goeferr.ErrInvalidState},
		{"missing callback", func(p *migrations.Plan) { p.Artifacts[0].Run = nil }, goeferr.ErrInvalidState},
		{"invalid text ID", func(p *migrations.Plan) { p.Artifacts[0].Entry.ID = "migration\x00one" }, goeferr.ErrMigrationConflict},
		{"invalid predecessor", func(p *migrations.Plan) { p.Artifacts[1].Entry.Predecessor = "bad" }, goeferr.ErrMigrationConflict},
	} {
		s.t.Run(test.name, func(t *testing.T) {
			s := s
			s.t = t
			plan := s.plan()
			test.change(&plan)
			s.assertApplyResult(plan, 0, test.want)
			s.assertNoRelation("public." + s.history)
		})
	}
	plan := s.plan()
	if n, err := migrations.Apply(nil, s.pool, plan); n != 0 || !errors.Is(err, goeferr.ErrInvalidState) {
		s.t.Fatal("missing context was not rejected")
	}
	expired, cancel := context.WithDeadline(s.ctx, time.Now().Add(-time.Second))
	defer cancel()
	if n, err := migrations.Apply(expired, s.pool, plan); n != 0 || !errors.Is(err, context.DeadlineExceeded) {
		s.t.Fatal("expired request was not rejected")
	}
	invalidSource := s.plan()
	invalidSource.Artifacts[0].CheckSource = func(context.Context, migrations.Queryer) (bool, error) {
		return false, nil
	}
	s.assertApplyResult(invalidSource, 0, goeferr.ErrMigrationRecoveryRequired)
	s.assertNoRelation("public." + s.history)
	plan.Database = "goef_other_database"
	s.assertApplyResult(plan, 0, goeferr.ErrMigrationRecoveryRequired)
	s.assertNoRelation("public." + s.history)
	plan.Database = "goef_smoke"
	wrongSchema := plan
	wrongSchema.HistorySchema = "goef_missing_migration_schema"
	s.assertApplyResult(wrongSchema, 0, goeferr.ErrUnknownDriver)
	s.assertApplied(plan, 2)

	changed := s.plan()
	changed.Artifacts[0].Entry.Digest[0] ^= 1
	s.assertApplyResult(changed, 0, goeferr.ErrMigrationConflict)
	s.assertHistory(plan.Artifacts)

	unverified := s.plan()
	unverified.Artifacts[1].CheckTarget = func(context.Context, migrations.Queryer) (bool, error) { return false, nil }
	s.assertApplyResult(unverified, 0, goeferr.ErrMigrationRecoveryRequired)
	s.assertHistory(plan.Artifacts)
	s.assertRows(1)

	short := s.plan()
	short.Artifacts = short.Artifacts[:1]
	s.assertApplyResult(short, 0, goeferr.ErrMigrationConflict)
	if _, err := s.pool.Exec(s.ctx, "UPDATE public."+s.history+" SET position = 4 WHERE position = 2"); err != nil {
		s.t.Fatal("could not arrange a hole in migration history")
	}
	s.assertApplyResult(plan, 0, goeferr.ErrMigrationConflict)
	if _, err := s.pool.Exec(s.ctx, "REVOKE SELECT ON public."+s.history+" FROM goef_driver"); err != nil {
		s.t.Fatal("could not limit fixture history visibility")
	}
	s.assertApplyResult(plan, 0, goeferr.ErrUnknownDriver)

	const legacy = "goef_migration_legacy_history"
	if _, err := s.pool.Exec(s.ctx, "CREATE TABLE public."+legacy+" (position bigint, id text, predecessor text, digest bytea)"); err != nil {
		s.t.Fatal("could not prepare the legacy history control")
	}
	if _, err := s.pool.Exec(s.ctx, "INSERT INTO public."+legacy+" VALUES (1, $1, '', NULL)", plan.Artifacts[0].Entry.ID); err != nil {
		s.t.Fatal("could not record the legacy history control")
	}
	plan.HistoryTable = legacy
	s.assertApplyResult(plan, 0, goeferr.ErrMigrationConflict)
	var retained bool
	if s.pool.QueryRow(s.ctx, "SELECT count(*) = 1 AND bool_and(digest IS NULL) FROM public."+legacy).Scan(&retained) != nil || !retained {
		s.t.Fatal("legacy history was changed")
	}
}

func postgresMigrationRollback(s postgresMigrationFixture) {
	plan := s.plan()
	broken := plan.Artifacts[1].Run
	plan.Artifacts[1].Run = func(ctx context.Context, session migrations.Session) error {
		if err := broken(ctx, session); err != nil {
			return err
		}
		_, err := session.Exec(ctx, "CREATE INDEX CONCURRENTLY goef_migration_no_transaction ON "+s.table+" (note)")
		return err
	}
	s.assertApplyResult(plan, 0, goeferr.ErrUnknownDriver)
	s.assertNoRelation(s.table)
	s.assertNoRelation("public." + s.history)

	plan.Artifacts[1].Run = func(ctx context.Context, session migrations.Session) error {
		if err := broken(ctx, session); err != nil {
			return err
		}
		if _, err := session.Exec(ctx, "CREATE TABLE public.goef_migration_deferred_child (parent_id integer REFERENCES "+
			s.table+"(id) DEFERRABLE INITIALLY DEFERRED)"); err != nil {
			return err
		}
		_, err := session.Exec(ctx, "INSERT INTO public.goef_migration_deferred_child VALUES (999)")
		return err
	}
	// The checks pass but PostgreSQL rejects COMMIT with a known rollback.
	s.assertApplyResult(plan, 0, goeferr.ErrCommitRolledBack)
	s.assertNoRelation("public.goef_migration_deferred_child")
	s.assertNoRelation(s.table)
	s.assertNoRelation("public." + s.history)
	plan = s.plan()
	plan.Artifacts[1].CheckTarget = func(context.Context, migrations.Queryer) (bool, error) { return false, nil }
	s.assertApplyResult(plan, 0, goeferr.ErrMigrationRecoveryRequired)
	s.assertNoRelation(s.table)
	s.assertNoRelation("public." + s.history)

	s.verifyHistoryMutationRollback()
	plan = s.plan()
	s.assertApplied(plan, 2)
	s.assertHistory(plan.Artifacts)
	s.assertRows(1)
}

func (s postgresMigrationFixture) verifyHistoryMutationRollback() {
	s.t.Helper()
	const parked = "goef_migration_parked_history"
	history := "public." + s.history
	for _, test := range []struct {
		name       string
		statements []string
		want       error
	}{
		{"hidden history", []string{"ALTER TABLE " + history + " ENABLE ROW LEVEL SECURITY"}, goeferr.ErrMigrationConflict},
		{"replaced history", []string{
			"ALTER TABLE " + history + " RENAME TO " + parked,
			"CREATE TABLE " + history + " (LIKE public." + parked + " INCLUDING ALL)",
		}, goeferr.ErrMigrationRecoveryRequired},
		{"history writes denied", []string{"REVOKE INSERT ON " + history + " FROM goef_driver"}, goeferr.ErrUnknownDriver},
		{"changed prior digest", []string{
			"UPDATE " + history + " SET digest = decode(repeat('ab', 32), 'hex') WHERE position = 1",
		}, goeferr.ErrMigrationRecoveryRequired},
		{"deleted prior entry", []string{"DELETE FROM " + history + " WHERE position = 1"}, goeferr.ErrMigrationConflict},
	} {
		s.t.Run(test.name, func(t *testing.T) {
			s := s
			s.t = t
			plan := s.plan()
			run := plan.Artifacts[1].Run
			plan.Artifacts[1].Run = func(ctx context.Context, session migrations.Session) error {
				if err := run(ctx, session); err != nil {
					return err
				}
				for _, statement := range test.statements {
					if _, err := session.Exec(ctx, statement); err != nil {
						t.Fatalf("could not arrange the history fault (native code %q)", postgresErrorCode(err))
					}
				}
				return nil
			}
			s.assertApplyResult(plan, 0, test.want)
			s.assertNoRelation(s.table)
			s.assertNoRelation(history)
			s.assertNoRelation("public." + parked)
		})
	}
}

type postgresMigrationResult struct {
	count int
	err   error
}

func (s postgresMigrationFixture) start(ctx context.Context, plan migrations.Plan) <-chan postgresMigrationResult {
	result := make(chan postgresMigrationResult, 1)
	go func() {
		count, err := migrations.Apply(ctx, s.pool, plan)
		result <- postgresMigrationResult{count: count, err: err}
	}()
	return result
}

func (s postgresMigrationFixture) waitForLock() {
	s.t.Helper()
	waitNextProbe(s.t, s.ctx, "migration advisory lock wait", func(ctx context.Context) (bool, error) {
		var waiting bool
		err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_catalog.pg_locks WHERE locktype = 'advisory'
			AND classid = 0 AND objid = $1::oid AND objsubid = 1 AND NOT granted)`, uint32(s.key)).Scan(&waiting)
		return waiting, err
	})
}

func (s postgresMigrationFixture) await(result <-chan postgresMigrationResult, want int, wantErr error) {
	s.t.Helper()
	select {
	case got := <-result:
		if got.count != want || !errors.Is(got.err, wantErr) {
			s.t.Fatalf("migrator returned count %d and error %v, want count %d and %v", got.count, got.err, want, wantErr)
		}
	case <-s.ctx.Done():
		s.t.Fatal("migration result did not arrive within the fixture deadline")
	}
}

func postgresMigrationContention(s postgresMigrationFixture) {
	ownerContext, stopOwner := context.WithCancel(s.ctx)
	defer stopOwner()
	owner := s.plan()
	ready := make(chan struct{})
	finish := make(chan struct{})
	run := owner.Artifacts[0].Run
	owner.Artifacts[0].Run = func(ctx context.Context, session migrations.Session) error {
		close(ready)
		select {
		case <-finish:
			return run(ctx, session)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	first := s.start(ownerContext, owner)
	select {
	case <-ready:
	case got := <-first:
		s.t.Fatalf("owner exited before the lock barrier with error %v", got.err)
	case <-s.ctx.Done():
		s.t.Fatal("owner did not reach the lock barrier")
	}
	second := s.start(ownerContext, s.plan())
	s.waitForLock()
	close(finish)
	s.await(first, 2, nil)
	s.await(second, 0, nil)
	s.assertHistory(owner.Artifacts)
	s.assertRows(1)
}

func postgresMigrationCancel(s postgresMigrationFixture) {
	holder, err := s.pool.Begin(s.ctx)
	if err != nil {
		s.t.Fatal("could not begin the lock owner transaction")
	}
	defer func() {
		if err := holder.Rollback(s.ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			s.t.Error("could not clean up the lock owner")
		}
	}()
	if _, err := holder.Exec(s.ctx, "SELECT pg_catalog.pg_advisory_xact_lock($1)", s.key); err != nil {
		s.t.Fatal("could not acquire the lock owner transaction lock")
	}
	waitContext, stopWait := context.WithCancel(s.ctx)
	defer stopWait()
	plan := s.plan()
	waiter := s.start(waitContext, plan)
	s.waitForLock()
	stopWait()
	s.await(waiter, 0, context.Canceled)

	plan.LockTimeout = 50 * time.Millisecond
	s.assertApplyResult(plan, 0, goeferr.ErrMigrationLocked)
	plan.LockTimeout = 5 * time.Second
	s.assertServerTimeout(plan, "lock_timeout", goeferr.ErrMigrationLocked)
	s.assertServerTimeout(plan, "statement_timeout", goeferr.ErrUnknownDriver)
	s.assertNoRelation("public." + s.history)
	var owned bool
	inspect := `SELECT EXISTS (SELECT 1 FROM pg_catalog.pg_locks WHERE locktype = 'advisory'
		AND classid = 0 AND objid = $1::oid AND objsubid = 1 AND granted AND pid = pg_catalog.pg_backend_pid())`
	if err := holder.QueryRow(s.ctx, inspect, uint32(s.key)).Scan(&owned); err != nil || !owned {
		s.t.Fatal("the waiter disturbed the owner's advisory lock")
	}
	if err := holder.Rollback(s.ctx); err != nil {
		s.t.Fatal("could not release the lock owner")
	}
	s.assertApplied(plan, 2)
}

func (s postgresMigrationFixture) assertServerTimeout(plan migrations.Plan, setting string, want error) {
	s.t.Helper()
	config := s.config.Copy()
	config.ConnConfig.RuntimeParams[setting] = "100ms"
	pool, err := pgxpool.NewWithConfig(s.ctx, config)
	if err != nil {
		s.t.Fatal("could not create the native timeout pool")
	}
	defer pool.Close()
	s.pool = pool
	s.assertApplyResult(plan, 0, want)
}

func postgresMigrationCommitFault(s postgresMigrationFixture) {
	config := s.config.Copy()
	config.MaxConns = 1
	var dropped atomic.Int64
	dial := config.ConnConfig.DialFunc
	config.ConnConfig.DialFunc = func(ctx context.Context, network, address string) (net.Conn, error) {
		conn, err := dial(ctx, network, address)
		if err != nil {
			return nil, err
		}
		return &postgresMigrationCommitConn{Conn: conn, dropped: &dropped}, nil
	}
	pool, err := pgxpool.NewWithConfig(s.ctx, config)
	if err != nil {
		s.t.Fatal("could not create the commit fault pool")
	}
	defer pool.Close()
	var before, after int
	if err := pool.QueryRow(s.ctx, "SELECT pg_backend_pid()").Scan(&before); err != nil {
		s.t.Fatal("could not identify the commit fault connection")
	}
	plan := s.plan()
	count, err := migrations.Apply(s.ctx, pool, plan)
	if count != 0 || !errors.Is(err, goeferr.ErrCommitOutcomeUnknown) || dropped.Load() != 1 ||
		strings.Contains(err.Error(), "fault-commit-ack") {
		s.t.Fatalf("migration commit fault returned count %d, faults %d and error %v", count, dropped.Load(), err)
	}
	if err := pool.QueryRow(s.ctx, "SELECT pg_backend_pid()").Scan(&after); err != nil || before == after {
		s.t.Fatal("the uncertain migration connection was not replaced")
	}
	s.assertHistory(plan.Artifacts)
	s.assertRows(1)
	s.assertApplied(plan, 0)
}

// The fixture's unencrypted protocol sends COMMIT as one simple-query frame.
// Withhold the first response after PostgreSQL has produced the commit reply.
type postgresMigrationCommitConn struct {
	net.Conn
	dropped *atomic.Int64
	waiting atomic.Bool
}

func (c *postgresMigrationCommitConn) Write(p []byte) (int, error) {
	if bytes.Equal(p, []byte("Q\x00\x00\x00\x0bcommit\x00")) {
		c.waiting.Store(true)
	}
	return c.Conn.Write(p)
}

func (c *postgresMigrationCommitConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	if c.waiting.CompareAndSwap(true, false) && n > 0 {
		c.dropped.Add(1)
		return 0, errors.Join(io.ErrUnexpectedEOF, errors.New("fault-commit-ack"))
	}
	return n, err
}
