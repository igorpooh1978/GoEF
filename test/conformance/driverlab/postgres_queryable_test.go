package driverlab

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/igorpooh1978/GoEF/examples/linq/generated"
	"github.com/igorpooh1978/GoEF/goeferr"
	"github.com/igorpooh1978/GoEF/linq/expressions"
	"github.com/igorpooh1978/GoEF/linq/queryable"
	"github.com/igorpooh1978/GoEF/model"
	"github.com/igorpooh1978/GoEF/provider"
	"github.com/igorpooh1978/GoEF/providers/postgres"
)

type queryableSettings struct {
	Key       string     `goef:"column=setting key"`
	Revision  int64      `goef:"column=saved_revision"`
	Applied   *int64     `goef:"column=applied_revision"`
	SavedAt   time.Time  `goef:"column=saved_at"`
	AppliedAt *time.Time `goef:"column=applied_at"`
	Enabled   bool       `goef:"column=enabled"`
	Label     string     `goef:"column=label"`
}

func TestPostgresQueryableEquality(t *testing.T) {
	if os.Getenv("GOEF_DRIVERLAB_POSTGRES_QUERYABLE") != "1" {
		t.Skip("use the isolated PostgreSQL Queryable runner")
	}

	s := &scenario{t: t, provider: "postgres", fixture: openPostgres(t)}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	s.ctx = ctx
	s.writer = s.lease()
	s.verifyMarker(ctx, s.writer.QueryContext)
	s.createTable(`CREATE TABLE public."goef Settings""fixture" (
		"setting key" text NOT NULL, saved_revision bigint NOT NULL, applied_revision bigint,
		saved_at timestamptz NOT NULL, applied_at timestamptz, enabled boolean NOT NULL, label text NOT NULL
	)`, `DROP TABLE public."goef Settings""fixture"`)
	s.createTable(`CREATE TABLE public."goef Customers""fixture" (city text, name text NOT NULL)`,
		`DROP TABLE public."goef Customers""fixture"`)

	const firstKey, secondKey = "Ка'фе\\ключ", "second"
	const label = "café's settings"
	savedAt := time.Date(2026, time.November, 26, 12, 34, 56, 123456000, time.UTC)
	appliedAt := savedAt.Add(time.Hour)
	const insert = `INSERT INTO public."goef Settings""fixture" (
		"setting key", saved_revision, applied_revision, saved_at, applied_at, enabled, label
	) VALUES ($1, $2, $3, $4, $5, $6, $7)`
	if _, err := s.writer.ExecContext(ctx, insert, firstKey, int64(42), nil, savedAt, nil, true, label); err != nil {
		t.Fatal("could not seed nullable settings row")
	}
	if _, err := s.writer.ExecContext(ctx, insert, secondKey, int64(52), int64(51), savedAt, appliedAt, false, ""); err != nil {
		t.Fatal("could not seed applied settings row")
	}
	if _, err := s.writer.ExecContext(ctx, `INSERT INTO public."goef Customers""fixture" (city, name) VALUES ($1, $2)`, "London", "Beverages"); err != nil {
		t.Fatal("could not seed generated-binding row")
	}

	backend, err := postgres.New(s.db)
	if err != nil {
		t.Fatal(err)
	}
	trace := &queryableTrace{Backend: backend}
	compiler := postgres.EqualityCompiler{Schema: "public"}
	entity := queryableEntity(t, model.Struct[queryableSettings]("Settings", nil, model.ToTable(`goef Settings"fixture`)))
	plan, err := queryable.Compile(compiler, entity, expressions.EqualField[queryableSettings, string]("Key"),
		"Revision", "Applied", "SavedAt", "AppliedAt", "Enabled", "Label")
	if err != nil {
		t.Fatal(err)
	}

	t.Run("LINQ-PG-001-S01", func(t *testing.T) {
		first, found, err := plan.Fetch(ctx, trace, firstKey)
		if err != nil || !found || first.Key != "" || first.Revision != 42 || first.Applied != nil ||
			!first.SavedAt.Equal(savedAt) || first.AppliedAt != nil || !first.Enabled || first.Label != label {
			t.Fatal("nullable settings projection did not match")
		}
		second, found, err := plan.Fetch(ctx, trace, secondKey)
		if err != nil || !found || second.Revision != 52 || second.Applied == nil || *second.Applied != 51 ||
			second.AppliedAt == nil || !second.AppliedAt.Equal(appliedAt) || second.Enabled || second.Label != "" {
			t.Fatal("present settings projection did not match")
		}
		missing, found, err := plan.Fetch(ctx, trace, firstKey+" absent")
		if err != nil || found || missing != (queryableSettings{}) {
			t.Fatal("missing result did not return zero, false, nil")
		}
		const expected = `SELECT "saved_revision", "applied_revision", "saved_at", "applied_at", "enabled", "label" FROM "public"."goef Settings""fixture" WHERE "setting key" = $1 LIMIT 2`
		if len(trace.commands) != 3 || trace.acquisitions != 3 {
			t.Fatal("each terminal read must dispatch exactly once")
		}
		for i, expectedValue := range []provider.Text{firstKey, secondKey, firstKey + " absent"} {
			if trace.commands[i] != expected || trace.arguments[i] != expectedValue {
				t.Fatal("compiled command and bound value did not remain separate")
			}
		}
	})

	t.Run("LINQ-PG-001-S02", func(t *testing.T) {
		mapping := queryableEntity(t, model.Struct[generated.Customer]("Customer", nil,
			model.ToTable(`goef Customers"fixture`), model.ColumnName("City", "city"), model.ColumnName("Name", "name")))
		invocation := generated.FindCustomer("London")
		plan, err := queryable.CompileGenerated(compiler, mapping, invocation)
		if err != nil {
			t.Fatal(err)
		}
		if name, found, err := plan.Fetch(ctx, trace, invocation.Value()); err != nil || !found || name != "Beverages" {
			t.Fatal("generated scalar projection did not match")
		}
		before := trace.acquisitions
		invalid, err := queryable.Compile(compiler, entity, expressions.EqualField[queryableSettings, int64]("Key"), "Revision")
		if !errors.Is(err, goeferr.ErrQueryTranslation) {
			t.Fatal("field/parameter mismatch did not fail translation")
		}
		if _, _, err := invalid.Fetch(ctx, trace, int64(42)); !errors.Is(err, goeferr.ErrInvalidState) || trace.acquisitions != before {
			t.Fatal("invalid plan acquired a connection")
		}
	})

	t.Run("LINQ-PG-001-S04", func(t *testing.T) {
		// The writer occupies the first slot. The second is the same physical
		// connection observed by the plain checkout and both compiled reads.
		var before, after int
		if err := s.db.QueryRowContext(ctx, "SELECT pg_backend_pid()").Scan(&before); err != nil {
			t.Fatal("could not observe the original PostgreSQL session")
		}
		projected, found, err := plan.FetchSessionNeutral(ctx, backend, firstKey)
		if err != nil || !found || projected.Revision != 42 {
			t.Fatal("reviewed settings read did not return the expected row")
		}
		if _, found, err := plan.FetchSessionNeutral(ctx, backend, firstKey+" absent"); err != nil || found {
			t.Fatal("a missing row did not complete cleanly")
		}
		if err := s.db.QueryRowContext(ctx, "SELECT pg_backend_pid()").Scan(&after); err != nil || after != before {
			t.Fatal("reviewed settings reads did not preserve the PostgreSQL session")
		}
	})

	t.Run("LINQ-PG-001-S03", func(t *testing.T) {
		if _, err := s.writer.ExecContext(ctx, insert, firstKey, int64(43), nil, savedAt, nil, false, ""); err != nil {
			t.Fatal("could not seed duplicate settings row")
		}
		if _, found, err := plan.Fetch(ctx, trace, firstKey); found || !errors.Is(err, goeferr.ErrMultipleResults) {
			t.Fatal("duplicate matches did not report a cardinality error")
		}
		blocker, err := s.writer.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal("could not begin PostgreSQL lock fixture")
		}
		defer blocker.Rollback()
		if _, err := blocker.ExecContext(ctx, `LOCK TABLE public."goef Settings""fixture" IN ACCESS EXCLUSIVE MODE`); err != nil {
			t.Fatal("could not acquire PostgreSQL fixture lock")
		}

		readCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		result := make(chan error, 1)
		go func() {
			_, _, queryErr := plan.Fetch(readCtx, backend, secondKey)
			result <- queryErr
		}()
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
	observe:
		for {
			var waiting bool
			err := blocker.QueryRowContext(ctx, `SELECT EXISTS (
				SELECT 1 FROM pg_catalog.pg_locks
				WHERE relation = 'public."goef Settings""fixture"'::regclass AND NOT granted
			)`).Scan(&waiting)
			if err != nil {
				cancel()
				<-result
				t.Fatal("could not observe the blocked PostgreSQL read")
			}
			if waiting {
				break observe
			}
			select {
			case err := <-result:
				t.Fatalf("query completed before the lock wait was observed: %v", err)
			case <-ctx.Done():
				cancel()
				<-result
				t.Fatal("PostgreSQL lock wait was not observed in time")
			case <-ticker.C:
			}
		}
		cancel()
		if err := <-result; !errors.Is(err, context.Canceled) || !errors.Is(err, goeferr.ErrCanceled) {
			t.Fatalf("canceled read lost its cause: %v", err)
		}
		if err := blocker.Rollback(); err != nil {
			t.Fatal("could not release PostgreSQL fixture lock")
		}
		if recovered, found, err := plan.Fetch(ctx, trace, secondKey); err != nil || !found || recovered.Revision != 52 {
			t.Fatal("pool did not recover after cancellation")
		}
		if stats := s.db.Stats(); stats.InUse != 1 {
			t.Fatal("compiled queries left a lease checked out")
		}
	})
}

func queryableEntity(t *testing.T, mapping model.EntityMapping) model.MappedEntity {
	t.Helper()
	mapped, err := model.Map(mapping)
	if err != nil {
		t.Fatal(err)
	}
	return mapped.Entities()[0]
}

type queryableTrace struct {
	provider.Backend
	acquisitions int
	commands     []string
	arguments    []provider.Text
}

func (b *queryableTrace) Acquire(ctx context.Context) (provider.Lease, error) {
	b.acquisitions++
	lease, err := b.Backend.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	return queryableTraceLease{Lease: lease, trace: b}, nil
}

type queryableTraceLease struct {
	provider.Lease
	trace *queryableTrace
}

func (l queryableTraceLease) Query(ctx context.Context, cmd provider.Command, args provider.Bindings) (provider.Rows, error) {
	l.trace.commands = append(l.trace.commands, cmd.SQL)
	l.trace.arguments = append(l.trace.arguments, args[0].(provider.Text))
	return l.Lease.Query(ctx, cmd, args)
}
