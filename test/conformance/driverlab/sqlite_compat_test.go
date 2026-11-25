package driverlab

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	sqliteFileName       = "compatibility.db"
	sqliteFileMarker     = ".goef-sqlite-exchange"
	sqliteFileAppID      = 0x474F4546
	sqliteFileMarkerSize = 64
)

type sqliteFileQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func TestSQLiteCompatibilityStep(t *testing.T) {
	stage, enabled := os.LookupEnv("GOEF_SQLITE_COMPAT_STAGE")
	if !enabled {
		t.Skip("run the SQLite compatibility lab")
	}
	journal := os.Getenv("GOEF_SQLITE_COMPAT_JOURNAL")
	if journal != "delete" && journal != "wal" {
		t.Fatal("invalid SQLite compatibility journal mode")
	}
	switch stage {
	case "create", "mutate", "reopen":
	default:
		t.Fatal("invalid SQLite compatibility stage")
	}

	if t.Run(stage, func(t *testing.T) {
		path := sqliteFilePath(t, stage == "create")
		ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
		defer cancel()

		mode := "rw"
		if stage == "reopen" {
			mode = "ro"
		}
		db := openSQLitePool(t, path, mode)
		if stage == "create" {
			setSQLiteJournalMode(t, ctx, db, journal)
		}
		checkSQLiteConnections(t, ctx, db, journal)
		if stage != "create" {
			var appID int
			if err := db.QueryRowContext(ctx, "PRAGMA application_id").Scan(&appID); err != nil || appID != sqliteFileAppID {
				t.Fatal("SQLite compatibility file has an unexpected application ID")
			}
		}
		switch stage {
		case "create":
			createSQLiteFileSchema(t, ctx, db)
			assertSQLiteFileState(t, ctx, db, false)
		case "mutate":
			mutateSQLiteFile(t, ctx, db)
		case "reopen":
			assertSQLiteFileState(t, ctx, db, true)
			_, err := db.ExecContext(ctx, "DELETE FROM compat_parent WHERE id = ?", 2)
			if sqliteNativeCode(err) != "8" { // SQLITE_READONLY
				t.Fatal("read-only SQLite reopen did not reject a write")
			}
		}
	}) {
		// A result is published only after the pool and all leases close.
		fmt.Printf("GOEF_SQLITE_FILE_RESULT=%s/%s/%s\n", sqliteVersion, journal, stage)
	}
}

func sqliteFilePath(t *testing.T, create bool) string {
	t.Helper()

	token := os.Getenv("GOEF_SQLITE_COMPAT_TOKEN")
	if len(token) != sqliteFileMarkerSize || strings.Trim(token, "0123456789abcdef") != "" {
		t.Fatal("invalid SQLite compatibility ownership token")
	}
	marker, err := os.Lstat(sqliteFileMarker)
	if err != nil || !marker.Mode().IsRegular() || marker.Size() != sqliteFileMarkerSize {
		t.Fatal("SQLite compatibility ownership marker is missing or invalid")
	}
	value, err := os.ReadFile(sqliteFileMarker)
	if err != nil || string(value) != token {
		t.Fatal("SQLite compatibility ownership marker mismatch")
	}

	path, err := filepath.Abs(sqliteFileName)
	if err != nil {
		t.Fatal("could not resolve the SQLite compatibility file")
	}
	if create {
		// Never adopt an existing file on a create step.
		handle, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			t.Fatal("could not create a new SQLite compatibility file")
		}
		if handle.Close() != nil {
			t.Fatal("could not close the new SQLite compatibility file")
		}
	} else {
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() {
			t.Fatal("SQLite compatibility file is missing or invalid")
		}
	}
	return path
}

func createSQLiteFileSchema(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()

	tx := beginSQLiteFileTx(t, ctx, db)
	defer finishUnfinishedSQLiteFileTx(t, tx)
	for _, statement := range []string{
		"PRAGMA application_id = 0x474F4546",
		`CREATE TABLE compat_parent (
			id INTEGER PRIMARY KEY, code TEXT NOT NULL UNIQUE, amount INTEGER NOT NULL,
			twice_amount INTEGER GENERATED ALWAYS AS (amount * 2) STORED, payload BLOB
		) STRICT`,
		`CREATE TABLE compat_child (
			id INTEGER PRIMARY KEY, parent_id INTEGER NOT NULL REFERENCES compat_parent(id)
		) STRICT`,
		"CREATE TABLE compat_history (version INTEGER PRIMARY KEY, name TEXT NOT NULL) STRICT",
		"CREATE TABLE compat_audit (parent_id INTEGER PRIMARY KEY, amount INTEGER NOT NULL) STRICT",
		"CREATE INDEX compat_amount_idx ON compat_parent (amount)",
		`CREATE TRIGGER compat_parent_audit AFTER UPDATE OF amount ON compat_parent
			BEGIN INSERT INTO compat_audit (parent_id, amount) VALUES (NEW.id, NEW.amount); END`,
	} {
		execSQLiteFile(t, ctx, tx, statement)
	}
	execSQLiteFile(t, ctx, tx, "INSERT INTO compat_parent (id, code, amount, payload) VALUES (?, ?, ?, ?)",
		1, "seed", 7, []byte{0x00, 0xff, 0x80})
	execSQLiteFile(t, ctx, tx, "INSERT INTO compat_child (id, parent_id) VALUES (?, ?)", 10, 1)
	execSQLiteFile(t, ctx, tx, "INSERT INTO compat_history (version, name) VALUES (?, ?)", 1, "initial")
	if tx.Commit() != nil {
		t.Fatal("could not commit the initial SQLite compatibility file")
	}
}

func mutateSQLiteFile(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()

	observer, err := db.Conn(ctx)
	if err != nil {
		t.Fatal("could not acquire SQLite compatibility observer")
	}
	defer func() {
		if observer.Close() != nil {
			t.Error("could not release SQLite compatibility observer")
		}
	}()
	assertSQLiteFileState(t, ctx, observer, false)

	failed := beginSQLiteFileTx(t, ctx, db)
	defer finishUnfinishedSQLiteFileTx(t, failed)
	changeSQLiteFile(t, ctx, failed)
	_, err = failed.ExecContext(ctx, "INSERT INTO compat_parent (id, code, amount) VALUES (?, ?, ?)", 3, "seed", 13)
	if sqliteNativeCode(err) != "2067" { // SQLITE_CONSTRAINT_UNIQUE
		t.Fatal("failed SQLite migration did not return the expected unique constraint code")
	}
	assertSQLiteFileState(t, ctx, observer, false)
	if failed.Rollback() != nil {
		t.Fatal("could not roll back the failed SQLite migration")
	}
	assertSQLiteFileState(t, ctx, observer, false)

	succeeded := beginSQLiteFileTx(t, ctx, db)
	defer finishUnfinishedSQLiteFileTx(t, succeeded)
	changeSQLiteFile(t, ctx, succeeded)
	assertSQLiteFileState(t, ctx, observer, false)
	if succeeded.Commit() != nil {
		t.Fatal("could not commit the compatible SQLite migration")
	}
	assertSQLiteFileState(t, ctx, observer, true)
}

func changeSQLiteFile(t *testing.T, ctx context.Context, tx *sql.Tx) {
	t.Helper()

	execSQLiteFile(t, ctx, tx, "ALTER TABLE compat_parent ADD COLUMN note TEXT NOT NULL DEFAULT 'seeded'")
	execSQLiteFile(t, ctx, tx, "UPDATE compat_parent SET amount = ?, note = ? WHERE id = ?", 9, "updated", 1)
	execSQLiteFile(t, ctx, tx, "INSERT INTO compat_parent (id, code, amount, payload, note) VALUES (?, ?, ?, ?, ?)",
		2, "added", 11, nil, "new")
	execSQLiteFile(t, ctx, tx, "INSERT INTO compat_child (id, parent_id) VALUES (?, ?)", 20, 2)
	execSQLiteFile(t, ctx, tx, "INSERT INTO compat_history (version, name) VALUES (?, ?)", 2, "add-note")
}

func assertSQLiteFileState(t *testing.T, ctx context.Context, queryer sqliteFileQueryer, migrated bool) {
	t.Helper()

	wantParents, wantChildren, wantHistory, wantAudit := "1/seed/7/14/blob/00FF80", "10/1", "1/initial", ""
	var wantNotes string
	wantNoteColumn := 0
	if migrated {
		wantParents = "1/seed/9/18/blob/00FF80;2/added/11/22/null/"
		wantChildren = "10/1;20/2"
		wantHistory = "1/initial;2/add-note"
		wantAudit = "1/9"
		wantNotes = "updated;new"
		wantNoteColumn = 1
	}

	var parents, children, history, audit string
	err := queryer.QueryRowContext(ctx, `
		SELECT
			(SELECT group_concat(id || '/' || code || '/' || amount || '/' || twice_amount ||
				'/' || typeof(payload) || '/' || hex(payload), ';' ORDER BY id) FROM compat_parent),
			(SELECT group_concat(id || '/' || parent_id, ';' ORDER BY id) FROM compat_child),
			(SELECT group_concat(version || '/' || name, ';' ORDER BY version) FROM compat_history),
			(SELECT coalesce(group_concat(parent_id || '/' || amount, ';' ORDER BY parent_id), '') FROM compat_audit)
	`).Scan(&parents, &children, &history, &audit)
	if err != nil {
		t.Fatalf("read SQLite file rows failed (native code %q)", sqliteNativeCode(err))
	}
	if parents != wantParents || children != wantChildren || history != wantHistory || audit != wantAudit {
		t.Fatal("SQLite file rows, generated values, trigger audit or migration history differ")
	}

	var strict, generated, index, noteColumn, trigger, foreignKey, foreignKeyErrors int
	err = queryer.QueryRowContext(ctx, `
		SELECT
			(SELECT strict FROM pragma_table_list('compat_parent') WHERE schema = 'main'),
			(SELECT count(*) FROM pragma_table_xinfo('compat_parent') WHERE name = 'twice_amount' AND hidden = 3),
			(SELECT count(*) FROM pragma_index_info('compat_amount_idx') WHERE name = 'amount'),
			(SELECT count(*) FROM pragma_table_xinfo('compat_parent') WHERE name = 'note' AND "notnull" = 1),
			(SELECT count(*) FROM sqlite_schema WHERE type = 'trigger' AND name = 'compat_parent_audit' AND tbl_name = 'compat_parent'),
			(SELECT count(*) FROM pragma_foreign_key_list('compat_child')
				WHERE "table" = 'compat_parent' AND "from" = 'parent_id' AND "to" = 'id'),
			(SELECT count(*) FROM pragma_foreign_key_check)
	`).Scan(&strict, &generated, &index, &noteColumn, &trigger, &foreignKey, &foreignKeyErrors)
	if err != nil {
		t.Fatalf("inspect SQLite file schema failed (native code %q)", sqliteNativeCode(err))
	}
	if strict != 1 || generated != 1 || index != 1 || noteColumn != wantNoteColumn || trigger != 1 || foreignKey != 1 || foreignKeyErrors != 0 {
		t.Fatal("SQLite file schema or foreign-key integrity differs")
	}
	if migrated {
		var notes string
		if err := queryer.QueryRowContext(ctx, "SELECT group_concat(note, ';' ORDER BY id) FROM compat_parent").Scan(&notes); err != nil || notes != wantNotes {
			t.Fatal("SQLite file did not preserve migrated column values")
		}
	}
	var integrity string
	if err := queryer.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&integrity); err != nil || integrity != "ok" {
		t.Fatal("SQLite file integrity check failed")
	}
}

func beginSQLiteFileTx(t *testing.T, ctx context.Context, db *sql.DB) *sql.Tx {
	t.Helper()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin SQLite file transaction failed (native code %q)", sqliteNativeCode(err))
	}
	return tx
}

func finishUnfinishedSQLiteFileTx(t *testing.T, tx *sql.Tx) {
	t.Helper()
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		t.Error("could not clean up the SQLite file transaction")
	}
}

func execSQLiteFile(t *testing.T, ctx context.Context, tx *sql.Tx, statement string, args ...any) {
	t.Helper()
	if _, err := tx.ExecContext(ctx, statement, args...); err != nil {
		t.Fatalf("SQLite compatibility transaction operation failed (native code %q)", sqliteNativeCode(err))
	}
}
