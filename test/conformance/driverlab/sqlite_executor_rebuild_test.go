package driverlab

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/igorpooh1978/GoEF/goeferr"
	"github.com/igorpooh1978/GoEF/migration"
	migrations "github.com/igorpooh1978/GoEF/migration/sqlite"
)

const executorRebuildInterruptedExit = 69

func TestSQLiteExecutorRebuildStep(t *testing.T) {
	stage, enabled := os.LookupEnv("GOEF_SQLITE_EXECUTOR_REBUILD_STAGE")
	if !enabled {
		t.Skip("run the SQLite compatibility lab")
	}
	switch stage {
	case "seed", "interrupt", "resume", "reopen":
	default:
		t.Fatal("invalid SQLite executor rebuild stage")
	}
	journal := os.Getenv("GOEF_SQLITE_COMPAT_JOURNAL")
	if journal != "delete" && journal != "wal" {
		t.Fatal("invalid SQLite executor rebuild journal mode")
	}
	marker := "GOEF_SQLITE_EXECUTOR_REBUILD_RESULT=" + sqliteVersion + "/" + journal + "/" + stage
	if t.Run(stage, func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
		defer cancel()
		path := sqliteFilePath(t, stage == "seed")
		mode := "rw"
		if stage == "reopen" {
			mode = "ro"
		}
		db := openSQLitePool(t, path, mode)
		if stage == "seed" {
			setSQLiteJournalMode(t, ctx, db, journal)
		}
		checkSQLiteConnections(t, ctx, db, journal)
		if stage != "seed" {
			checkSQLiteRebuildAppID(t, ctx, db)
		}
		// All migration and pool reuse checks must share one physical lease
		// so the fixture detects a leaked foreign_keys setting.
		db.SetMaxOpenConns(1)

		plan := sqliteExecutorRebuildPlan()
		switch stage {
		case "seed":
			seedSQLiteRebuild(t, ctx, db)
			initial := plan
			initial.Lineage = initial.Lineage[:1]
			initial.Run = func(context.Context, migrations.Session) error { return nil }
			initial.CheckTarget = initial.CheckSource
			if applied, err := migrations.Apply(ctx, db, initial); err != nil || !applied {
				t.Fatalf("could not register the verified initial artifact: %v", err)
			}
			assertSQLiteRebuild(t, ctx, db, false, false)
			assertExecutorRebuildHistory(t, ctx, db, initial.Lineage)
		case "interrupt":
			target := plan.CheckTarget
			plan.CheckTarget = func(ctx context.Context, query migrations.Queryer) (bool, error) {
				matches, err := target(ctx, query)
				if err != nil || !matches {
					return matches, err
				}
				assertSQLiteRebuild(t, ctx, query, true, false)
				assertExecutorRebuildHistory(t, ctx, query, plan.Lineage[:1])
				if t.Failed() {
					return false, nil
				}
				// Abruptly end the worker after the DDL/data work and before
				// the executor writes the history record or commits.
				fmt.Fprintln(os.Stderr, marker)
				os.Exit(executorRebuildInterruptedExit)
				return false, nil
			}
			if _, err := migrations.Apply(ctx, db, plan); err != nil {
				t.Fatalf("the GoEF rebuild did not reach the interruption point: %v", err)
			}
			t.Fatal("the GoEF rebuild unexpectedly returned")
		case "resume":
			assertSQLiteRebuild(t, ctx, db, false, false)
			assertExecutorRebuildHistory(t, ctx, db, plan.Lineage[:1])
			checkExecutorRebuildRejections(t, ctx, path, db, plan)
			if applied, err := migrations.Apply(ctx, db, plan); err != nil || !applied {
				t.Fatalf("the GoEF executor did not apply the proven unapplied rebuild: %v", err)
			}
			assertSQLiteRebuild(t, ctx, db, true, false)
			assertExecutorRebuildHistory(t, ctx, db, plan.Lineage)
			checkExecutorRebuildForeignKeys(t, ctx, db)

			plan.Run = func(context.Context, migrations.Session) error {
				t.Error("the executor replayed an already applied migration")
				return errors.New("unexpected replay")
			}
			if applied, err := migrations.Apply(ctx, db, plan); err != nil || applied {
				t.Fatalf("a verified completed rebuild must be a no-op: %v", err)
			}
			// History alone cannot authorize a no-op when live data differs.
			execExecutorFixture(t, ctx, db, "UPDATE parent SET note = 'ambiguous' WHERE id = 1")
			requireExecutorError(t, ctx, db, plan, goeferr.ErrMigrationRecoveryRequired)
			execExecutorFixture(t, ctx, db, "UPDATE parent SET note = 'preserved' WHERE id = 1")
			assertExecutorRebuildHistory(t, ctx, db, plan.Lineage)
		case "reopen":
			assertSQLiteRebuild(t, ctx, db, true, false)
			assertExecutorRebuildHistory(t, ctx, db, plan.Lineage)
			if _, err := db.ExecContext(ctx, "UPDATE parent SET note = 'readonly' WHERE id = 1"); sqliteNativeCode(err) != "8" {
				t.Fatal("the read-only observer did not reject a write")
			}
		}
	}) {
		fmt.Println(marker)
	}
}

var executorRebuildCommands = []string{
	`CREATE TABLE replacement (
		id INTEGER PRIMARY KEY AUTOINCREMENT, amount INTEGER NOT NULL,
		doubled INTEGER GENERATED ALWAYS AS (amount * 2) STORED, payload BLOB NOT NULL,
		note TEXT NOT NULL DEFAULT 'preserved'
	)`,
	"INSERT INTO replacement (id, amount, payload) SELECT id, amount, payload FROM parent",
	"UPDATE sqlite_sequence SET seq = (SELECT seq FROM sqlite_sequence WHERE name = 'parent') WHERE name = 'replacement'",
	"DROP TABLE parent",
	"ALTER TABLE replacement RENAME TO parent",
	sqliteRebuildIndex,
	sqliteRebuildTrigger,
	"UPDATE parent SET amount = 9 WHERE id = 1",
	"INSERT INTO parent (amount, payload) VALUES (1, X'12')",
}

func sqliteExecutorRebuildPlan() migrations.Plan {
	return migrations.Plan{
		ApplicationID: sqliteFileAppID,
		Lineage: []migration.Entry{
			{ID: "initial", Digest: sha256.Sum256([]byte("synthetic initial fixture v1"))},
			{ID: "rebuild", Predecessor: "initial", Digest: sha256.Sum256([]byte(strings.Join(executorRebuildCommands, "\n")))},
		},
		CheckSource: func(ctx context.Context, query migrations.Queryer) (bool, error) {
			return verifyExecutorRebuildState(ctx, query, 7, 11, 0, 1)
		},
		CheckTarget: func(ctx context.Context, query migrations.Queryer) (bool, error) {
			return verifyExecutorRebuildState(ctx, query, 9, 12, 1, 2)
		},
		Run: func(ctx context.Context, session migrations.Session) error {
			for _, statement := range executorRebuildCommands {
				if _, err := session.ExecContext(ctx, statement); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func verifyExecutorRebuildState(ctx context.Context, query migrations.Queryer, amount, sequence, note, rowCount int) (bool, error) {
	var matches bool
	err := query.QueryRowContext(ctx, `
		SELECT coalesce((SELECT amount = ? AND doubled = amount * 2 AND hex(payload) = '00FF80' FROM parent WHERE id = 1), 0)
		    AND (SELECT seq FROM sqlite_sequence WHERE name = 'parent') = ?
		    AND (SELECT count(*) FROM pragma_table_xinfo('parent') WHERE name = 'note') = ?
		    AND (SELECT count(*) FROM parent) = ?
		    AND (SELECT group_concat(type || ':' || name, ';' ORDER BY type)
				FROM main.sqlite_schema WHERE tbl_name = 'parent') = 'index:parent_amount_idx;table:parent;trigger:parent_audit'
		    AND (SELECT count(*) FROM main.sqlite_schema WHERE type = 'view' OR name = 'replacement') = 0
		    AND (SELECT count(*) FROM pragma_foreign_key_list('child')
				WHERE "table" = 'parent' AND "from" = 'parent_id' AND "to" = 'id') = 1
		    AND EXISTS (SELECT 1 FROM child WHERE id = 10 AND parent_id = 1)
	`, amount, sequence, note, rowCount).Scan(&matches)
	// The source schema has no note column.
	if err == nil && matches && note == 1 {
		err = query.QueryRowContext(ctx, "SELECT count(*) = ? FROM parent WHERE note = 'preserved'", rowCount).Scan(&matches)
	}
	return matches, err
}

func checkExecutorRebuildRejections(t *testing.T, ctx context.Context, path string, db *sql.DB, plan migrations.Plan) {
	t.Helper()
	reject := plan
	reject.Run = func(context.Context, migrations.Session) error {
		t.Error("an unverified migration reached execution")
		return errors.New("unexpected dispatch")
	}
	reject.ApplicationID++
	requireExecutorError(t, ctx, db, reject, goeferr.ErrMigrationRecoveryRequired)
	reject.ApplicationID = plan.ApplicationID
	reject.Lineage = slices.Clone(plan.Lineage)
	reject.Lineage[0].Digest[0] ^= 0xff
	requireExecutorError(t, ctx, db, reject, goeferr.ErrMigrationConflict)
	reject.Lineage = plan.Lineage

	execExecutorFixture(t, ctx, db, "CREATE VIEW unexpected_parent_view AS SELECT id FROM parent")
	requireExecutorError(t, ctx, db, reject, goeferr.ErrMigrationRecoveryRequired)
	execExecutorFixture(t, ctx, db, "DROP VIEW unexpected_parent_view")
	assertSQLiteRebuild(t, ctx, db, false, false)

	reject.Run = func(ctx context.Context, session migrations.Session) error {
		if err := plan.Run(ctx, session); err != nil {
			t.Fatal("the rollback control did not reach the expected fault")
		}
		return errors.New("private migration callback value")
	}
	err := requireExecutorError(t, ctx, db, reject, goeferr.ErrUnknownDriver)
	if strings.Contains(err.Error(), "private migration callback value") {
		t.Fatal("the executor leaked private migration text")
	}
	assertSQLiteRebuild(t, ctx, db, false, false)
	checkExecutorRebuildForeignKeys(t, ctx, db)

	reject.Run = func(ctx context.Context, session migrations.Session) error {
		if err := plan.Run(ctx, session); err != nil {
			return err
		}
		_, err := session.ExecContext(ctx, "INSERT INTO child VALUES (25, 99)")
		return err
	}
	requireExecutorError(t, ctx, db, reject, goeferr.ErrMigrationRecoveryRequired)
	assertSQLiteRebuild(t, ctx, db, false, false)
	assertExecutorRebuildHistory(t, ctx, db, plan.Lineage[:1])

	holderPool := openSQLitePool(t, path, "rw")
	holder, err := holderPool.Conn(ctx)
	if err != nil {
		t.Fatal("could not acquire the competing SQLite writer")
	}
	defer holder.Close()
	execExecutorFixture(t, ctx, holder, "BEGIN IMMEDIATE")
	defer func() { execExecutorFixture(t, ctx, holder, "ROLLBACK") }()

	// Give acquisition a finite native wait on the exact contender lease.
	execExecutorFixture(t, ctx, db, "PRAGMA busy_timeout=10")
	requireExecutorError(t, ctx, db, reject, goeferr.ErrMigrationLocked)
	assertExecutorRebuildHistory(t, ctx, db, plan.Lineage[:1])
}

func requireExecutorError(t *testing.T, ctx context.Context, db *sql.DB, plan migrations.Plan, want error) error {
	t.Helper()
	applied, err := migrations.Apply(ctx, db, plan)
	if applied || !errors.Is(err, want) {
		t.Fatalf("Apply = %t, %v; want %v", applied, err, want)
	}
	return err
}

func checkExecutorRebuildForeignKeys(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	var enabled int
	if err := db.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&enabled); err != nil || enabled != 1 {
		t.Fatal("the executor returned a lease without foreign-key enforcement")
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO child VALUES (20, 99)"); sqliteNativeCode(err) != "787" {
		t.Fatal("the pool did not reject an orphan after migration")
	}
}

func assertExecutorRebuildHistory(t *testing.T, ctx context.Context, query migrations.Queryer, entries []migration.Entry) {
	t.Helper()
	rows, err := query.QueryContext(ctx, "SELECT position, id, predecessor, digest FROM main.__goef_migrations_history ORDER BY position")
	if err != nil {
		t.Fatal("could not inspect the GoEF migration history")
	}
	defer rows.Close()
	for index, entry := range entries {
		var position int
		var id, predecessor string
		var digest []byte
		if !rows.Next() || rows.Scan(&position, &id, &predecessor, &digest) != nil ||
			position != index+1 || id != entry.ID || predecessor != entry.Predecessor || !slices.Equal(digest, entry.Digest[:]) {
			t.Fatal("the GoEF history differs from the expected artifact lineage")
		}
	}
	if rows.Next() || rows.Err() != nil || rows.Close() != nil {
		t.Fatal("the GoEF history has extra or unreadable entries")
	}
}

type executorFixtureWriter interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func execExecutorFixture(t *testing.T, ctx context.Context, writer executorFixtureWriter, statement string) {
	t.Helper()
	if _, err := writer.ExecContext(ctx, statement); err != nil {
		t.Fatal("could not execute an owned SQLite executor fixture statement")
	}
}
