package driverlab

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"github.com/igorpooh1978/GoEF/goeferr"
	"github.com/igorpooh1978/GoEF/provider"
	"github.com/igorpooh1978/GoEF/providers/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

const sessionReuseSQL = "SELECT pg_backend_pid(), $1::text, NULL::bigint"
const sessionReuseValue = "O'Brien, café, \\"

func sessionReuseCommand(statement string) provider.Command {
	return provider.Command{
		SQL:   statement,
		Slots: []provider.Slot{{Kind: provider.KindText}},
		Columns: []provider.Column{
			{Kind: provider.KindInteger}, {Kind: provider.KindText}, {Kind: provider.KindInteger, Nullable: true},
		},
	}
}

func openPostgresNativePool(t testing.TB, db *sql.DB, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	config, err := pgxpool.ParseConfig("host=127.0.0.1 dbname=goef_smoke user=goef_driver password=placeholder")
	if err != nil {
		t.Fatal("could not prepare the native pool")
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatal("could not read the scoped driver configuration")
	}
	rawErr := conn.Raw(func(native any) error {
		config.ConnConfig = native.(*stdlib.Conn).Conn().Config()
		return nil
	})
	if closeErr := conn.Close(); rawErr != nil || closeErr != nil {
		t.Fatal("could not return the configuration lease")
	}
	config.MaxConns = 1
	config.MinConns = 0
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal("could not open native pool")
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestPostgresSessionReuse(t *testing.T) {
	if os.Getenv("GOEF_DRIVERLAB_PROVIDER") != "postgres" {
		t.Skip("requires the isolated PostgreSQL driver lab")
	}
	direct := openPostgres(t).db
	direct.SetMaxOpenConns(1)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	s := &scenario{t: t, provider: "postgres"}
	s.verifyMarker(ctx, direct.QueryContext)

	native := openPostgresNativePool(t, direct, ctx)
	wrapped := stdlib.OpenDBFromPool(native)
	t.Cleanup(func() { _ = wrapped.Close() })
	wrapped.SetMaxOpenConns(1)
	for _, tc := range []struct {
		name string
		db   *sql.DB
	}{
		{name: "direct", db: direct},
		{name: "pgxpool", db: wrapped},
	} {
		t.Run(tc.name, func(t *testing.T) { checkPostgresSessionReuse(t, ctx, tc.db) })
	}
	if err := wrapped.Close(); err != nil {
		t.Fatal("wrapper did not close cleanly")
	}
	if err := native.Ping(ctx); err != nil {
		t.Fatal("the caller-owned pgx pool closed with its wrapper")
	}
}

func checkPostgresSessionReuse(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	backend, err := postgres.New(db)
	if err != nil {
		t.Fatal(err)
	}
	readSession := func(t *testing.T) int64 {
		t.Helper()
		var pid int64
		var name string
		var setting sql.NullString
		if err := db.QueryRowContext(ctx, "SELECT pg_backend_pid(), current_setting('application_name'), current_setting('goef.fixture', true)").Scan(&pid, &name, &setting); err != nil || name != "goef_driverlab" || setting.Valid {
			t.Fatal("did not observe a usable, clean session")
		}
		return pid
	}
	for _, tc := range []struct {
		name        string
		statement   string
		disposition provider.Disposition
		mode        string
		reuse       bool
	}{
		{name: "PROVIDER-REUSE-001-S01/drained", statement: sessionReuseSQL, disposition: provider.Reuse, reuse: true},
		{name: "PROVIDER-REUSE-001-S02/raw", statement: sessionReuseSQL, disposition: provider.Reuse, mode: "raw"},
		{name: "PROVIDER-REUSE-001-S02/raw-side-effect", statement: "SELECT pg_backend_pid(), set_config('goef.fixture', $1, false), NULL::bigint", disposition: provider.Reuse, mode: "raw"},
		{name: "PROVIDER-REUSE-001-S02/not-drained", statement: sessionReuseSQL, disposition: provider.Reuse, mode: "early"},
		{name: "PROVIDER-REUSE-001-S02/not-closed", statement: sessionReuseSQL, disposition: provider.Reuse, mode: "open"},
		{name: "PROVIDER-REUSE-001-S02/canceled", statement: sessionReuseSQL, disposition: provider.Reuse, mode: "cancel"},
		{name: "PROVIDER-REUSE-001-S02/query-error", statement: "SELECT pg_backend_pid() / 0, $1::text, NULL::bigint", disposition: provider.Reuse, mode: "query-error"},
		{name: "PROVIDER-REUSE-001-S02/materialization-error", statement: "SELECT pg_backend_pid(), $1::text, 'wrong-kind'::text", disposition: provider.Reuse, mode: "row-error"},
		{name: "PROVIDER-REUSE-001-S02/earlier-command", statement: sessionReuseSQL, disposition: provider.Reuse, mode: "used"},
		{name: "PROVIDER-REUSE-001-S03/discard", statement: sessionReuseSQL, disposition: provider.Discard},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := readSession(t)
			lease, err := backend.Acquire(ctx)
			if err != nil {
				t.Fatal(err)
			}
			released := false
			defer func() {
				if !released {
					_ = lease.Release(ctx, provider.Discard)
				}
			}()
			if tc.mode == "used" {
				if _, err := lease.Exec(ctx, provider.Command{SQL: "SET application_name = 'retire_me'"}, nil); err != nil {
					t.Fatal(err)
				}
			}
			query := lease.Query
			if tc.mode != "raw" {
				capability, ok := lease.(provider.SessionNeutralQuerier)
				if !ok {
					t.Fatal("lease does not provide session-neutral reads")
				}
				query = capability.QuerySessionNeutral
			}
			queryCtx, stop := context.WithCancel(ctx)
			defer stop()
			rows, err := query(queryCtx, sessionReuseCommand(tc.statement), provider.Bindings{provider.Text(sessionReuseValue)})
			if tc.mode == "query-error" {
				var detail *goeferr.ErrorDetail
				if rows != nil || !errors.As(err, &detail) || detail.NativeCode() != "22012" {
					t.Fatalf("expected division-by-zero error, got %v", err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				row, err := rows.Next()
				if tc.mode == "row-error" {
					if !errors.Is(err, goeferr.ErrMaterialization) {
						t.Fatalf("expected materialization error, got %v", err)
					}
				} else if err != nil || len(row) != 3 || row[0] != provider.Integer(before) ||
					row[1] != provider.Text(sessionReuseValue) || row[2] != (provider.Null{}) {
					t.Fatalf("unexpected result on the leased session: %v", err)
				}
				if tc.mode == "cancel" {
					stop()
					if _, err := rows.Next(); !errors.Is(err, context.Canceled) {
						t.Fatalf("expected causal cancellation, got %v", err)
					}
				} else if tc.mode != "early" && tc.mode != "row-error" {
					if _, err := rows.Next(); err != io.EOF {
						t.Fatalf("expected clean EOF, got %v", err)
					}
				}
				if tc.mode != "open" {
					if err := rows.Close(ctx); err != nil {
						t.Fatal(err)
					}
				}
			}
			releaseErr := lease.Release(ctx, tc.disposition)
			released = true
			if tc.mode == "open" {
				if !errors.Is(releaseErr, goeferr.ErrContextBusy) {
					t.Fatalf("expected release to report open rows, got %v", releaseErr)
				}
			} else if releaseErr != nil {
				t.Fatal(releaseErr)
			}
			if after := readSession(t); (before == after) != tc.reuse {
				t.Fatalf("unexpected backend PID transition %d to %d (reuse=%t)", before, after, tc.reuse)
			}
		})
	}
}

func BenchmarkPostgresSessionReuse(b *testing.B) {
	if os.Getenv("GOEF_DRIVERLAB_PROVIDER") != "postgres" {
		b.Skip("requires the isolated PostgreSQL driver lab")
	}
	direct := openPostgresFixture(b).db
	ctx, cancel := context.WithTimeout(b.Context(), 2*time.Minute)
	defer cancel()
	native := openPostgresNativePool(b, direct, ctx)
	wrapped := stdlib.OpenDBFromPool(native)
	defer wrapped.Close()
	wrapped.SetMaxOpenConns(1)
	backend, err := postgres.New(wrapped)
	if err != nil {
		b.Fatal(err)
	}
	command := sessionReuseCommand(sessionReuseSQL)
	bindings := provider.Bindings{provider.Text(sessionReuseValue)}

	b.Run("pgxpool", func(b *testing.B) {
		for b.Loop() {
			var pid int64
			var value string
			var nullable sql.NullInt64
			if err := native.QueryRow(ctx, sessionReuseSQL, sessionReuseValue).Scan(&pid, &value, &nullable); err != nil || pid <= 0 || value != sessionReuseValue || nullable.Valid {
				b.Fatalf("unexpected native result: %v", err)
			}
		}
	})
	for _, tc := range []struct {
		name    string
		neutral bool
	}{{name: "raw"}, {name: "session-neutral", neutral: true}} {
		b.Run(tc.name, func(b *testing.B) {
			if err := native.Ping(ctx); err != nil {
				b.Fatal("could not warm the native pool")
			}
			before := native.Stat().NewConnsCount()
			for b.Loop() {
				lease, err := backend.Acquire(ctx)
				if err != nil {
					b.Fatal(err)
				}
				query := lease.Query
				if tc.neutral {
					query = lease.(provider.SessionNeutralQuerier).QuerySessionNeutral
				}
				rows, err := query(ctx, command, bindings)
				if err != nil {
					_ = lease.Release(ctx, provider.Discard)
					b.Fatal(err)
				}
				row, readErr := rows.Next()
				valid := readErr == nil && len(row) == 3 && row[0].(provider.Integer) > 0 && row[1] == provider.Text(sessionReuseValue) && row[2] == (provider.Null{})
				_, endErr := rows.Next()
				closeErr := rows.Close(ctx)
				releaseErr := lease.Release(ctx, provider.Reuse)
				if !valid || endErr != io.EOF || closeErr != nil || releaseErr != nil {
					b.Fatalf("unexpected adapter result or cleanup: %v", errors.Join(readErr, endErr, closeErr, releaseErr))
				}
			}
			b.ReportMetric(float64(native.Stat().NewConnsCount()-before)/float64(b.N), "connections/op")
		})
	}
}
