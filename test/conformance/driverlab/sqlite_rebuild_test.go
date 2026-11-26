package driverlab

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"
)

const sqliteRebuildInterruptedExit = 68

func TestSQLiteRebuildStep(t *testing.T) {
	stage, enabled := os.LookupEnv("GOEF_SQLITE_REBUILD_STAGE")
	if !enabled {
		t.Skip("run the SQLite compatibility lab")
	}
	switch stage {
	case "seed", "interrupt", "resume", "reopen":
	default:
		t.Fatal("invalid SQLite rebuild stage")
	}
	journal := os.Getenv("GOEF_SQLITE_COMPAT_JOURNAL")
	if journal != "delete" && journal != "wal" {
		t.Fatal("invalid SQLite rebuild journal mode")
	}
	marker := "GOEF_SQLITE_REBUILD_RESULT=" + sqliteVersion + "/" + journal + "/" + stage
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

		switch stage {
		case "seed":
			seedSQLiteRebuild(t, ctx, db)
			assertSQLiteRebuild(t, ctx, db, false, false)
		case "interrupt", "resume":
			rebuildSQLiteParent(t, ctx, db, stage == "interrupt", marker)
		case "reopen":
			assertSQLiteRebuild(t, ctx, db, true, true)
		}
	}) {
		fmt.Println(marker)
	}
}

func checkSQLiteRebuildAppID(t *testing.T, ctx context.Context, queryer sqliteFileQueryer) {
	t.Helper()
	var appID int
	if err := queryer.QueryRowContext(ctx, "PRAGMA application_id").Scan(&appID); err != nil || appID != sqliteFileAppID {
		t.Fatal("unexpected SQLite rebuild application ID")
	}
}

func seedSQLiteRebuild(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	tx := beginSQLiteFileTx(t, ctx, db)
	defer finishUnfinishedSQLiteFileTx(t, tx)
	for _, statement := range []string{
		"PRAGMA application_id = 0x474F4546",
		`CREATE TABLE parent (
			id INTEGER PRIMARY KEY AUTOINCREMENT, amount INTEGER NOT NULL,
			doubled INTEGER GENERATED ALWAYS AS (amount * 2) STORED, payload BLOB NOT NULL
		)`,
		"CREATE TABLE child (id INTEGER PRIMARY KEY, parent_id INTEGER NOT NULL REFERENCES parent(id))",
		"CREATE TABLE audit (parent_id INTEGER PRIMARY KEY, amount INTEGER NOT NULL)",
		"CREATE TABLE history (version INTEGER PRIMARY KEY, name TEXT NOT NULL)",
		sqliteRebuildIndex,
		sqliteRebuildTrigger,
		"INSERT INTO history VALUES (1, 'initial')",
		"INSERT INTO parent (id, amount, payload) VALUES (1, 7, X'00FF80'), (11, 1, X'11')",
		"DELETE FROM parent WHERE id = 11",
		"INSERT INTO child VALUES (10, 1)",
	} {
		execSQLiteFile(t, ctx, tx, statement)
	}
	if tx.Commit() != nil {
		t.Fatal("could not commit the SQLite rebuild seed")
	}
}

const (
	sqliteRebuildIndex   = "CREATE INDEX parent_amount_idx ON parent (amount)"
	sqliteRebuildTrigger = `CREATE TRIGGER parent_audit AFTER UPDATE OF amount ON parent
		BEGIN INSERT INTO audit (parent_id, amount) VALUES (NEW.id, NEW.amount); END`
)

func rebuildSQLiteParent(t *testing.T, ctx context.Context, db *sql.DB, interrupt bool, marker string) {
	t.Helper()
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatal("could not acquire the SQLite rebuild connection")
	}
	defer func() {
		if conn.Close() != nil {
			t.Error("could not close the SQLite rebuild connection")
		}
	}()
	checkSQLiteRebuildAppID(t, ctx, conn)
	setSQLiteRebuildForeignKeys(t, ctx, conn, false)
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal("could not begin the SQLite table rebuild")
	}
	defer finishUnfinishedSQLiteFileTx(t, tx)
	assertSQLiteRebuild(t, ctx, tx, false, false)
	for _, statement := range []string{
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
	} {
		execSQLiteFile(t, ctx, tx, statement)
	}
	assertSQLiteRebuild(t, ctx, tx, true, false)
	if interrupt {
		// Exit with the real DDL transaction open and no new history row.
		fmt.Fprintln(os.Stderr, marker)
		os.Exit(sqliteRebuildInterruptedExit)
	}
	execSQLiteFile(t, ctx, tx, "INSERT INTO history VALUES (2, 'rebuild')")
	if tx.Commit() != nil {
		t.Fatal("could not commit the SQLite rebuild and its history")
	}
	setSQLiteRebuildForeignKeys(t, ctx, conn, true)
	if _, err := conn.ExecContext(ctx, "INSERT INTO child VALUES (20, 99)"); sqliteNativeCode(err) != "787" {
		t.Fatal("the rebuilt relationship did not reject an orphan row")
	}
}

func setSQLiteRebuildForeignKeys(t *testing.T, ctx context.Context, conn *sql.Conn, enabled bool) {
	t.Helper()
	pragma, want := "PRAGMA foreign_keys=OFF", 0
	if enabled {
		pragma, want = "PRAGMA foreign_keys=ON", 1
	}
	if _, err := conn.ExecContext(ctx, pragma); err != nil {
		t.Fatal("could not set foreign keys on the SQLite rebuild connection")
	}
	var actual int
	if err := conn.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&actual); err != nil || actual != want {
		t.Fatal("unexpected foreign-key setting on the SQLite rebuild connection")
	}
}

func assertSQLiteRebuild(t *testing.T, ctx context.Context, queryer sqliteFileQueryer, rebuilt, committed bool) {
	t.Helper()
	var rowAmount, generated, seq, note, index, trigger, child, violations, parentCount, newID, audit, schemaScratch int
	var payload, integrity, history string
	err := queryer.QueryRowContext(ctx, `
		SELECT amount, doubled, hex(payload),
			(SELECT seq FROM sqlite_sequence WHERE name = 'parent'),
			(SELECT count(*) FROM pragma_table_xinfo('parent') WHERE name = 'note' AND type = 'TEXT'
				AND "notnull" = 1 AND dflt_value = '''preserved'''),
			(SELECT count(*) FROM pragma_index_info('parent_amount_idx') WHERE name = 'amount'),
			(SELECT count(*) FROM sqlite_schema WHERE type = 'trigger' AND name = 'parent_audit' AND tbl_name = 'parent'),
			(SELECT count(*) FROM pragma_foreign_key_list('child')
				WHERE "table" = 'parent' AND "from" = 'parent_id' AND "to" = 'id'),
			(SELECT count(*) FROM pragma_foreign_key_check),
			(SELECT count(*) FROM parent),
			(SELECT coalesce(max(id), 0) FROM parent WHERE amount = 1 AND doubled = 2 AND hex(payload) = '12'),
			(SELECT coalesce(sum(amount), 0) FROM audit WHERE parent_id = 1),
			(SELECT group_concat(version || '/' || name, ';' ORDER BY version) FROM history),
			(SELECT count(*) FROM sqlite_schema WHERE name = 'replacement'),
			(SELECT integrity_check FROM pragma_integrity_check)
		FROM parent WHERE id = 1
	`).Scan(&rowAmount, &generated, &payload, &seq, &note, &index, &trigger, &child, &violations,
		&parentCount, &newID, &audit, &history, &schemaScratch, &integrity)
	if err != nil {
		t.Fatalf("could not inspect SQLite rebuild state (native code %q)", sqliteNativeCode(err))
	}
	wantAmount, wantSeq, wantNote, wantParents, wantNewID, wantAudit := 7, 11, 0, 1, 0, 0
	if rebuilt {
		wantAmount, wantSeq, wantNote, wantParents, wantNewID, wantAudit = 9, 12, 1, 2, 12, 9
		var preserved int
		if err := queryer.QueryRowContext(ctx, "SELECT count(*) FROM parent WHERE note = 'preserved'").Scan(&preserved); err != nil || preserved != 2 {
			t.Fatal("SQLite rebuild did not preserve the default on both rows")
		}
	}
	wantHistory := "1/initial"
	if committed {
		wantHistory += ";2/rebuild"
	}
	var childRows string
	if err := queryer.QueryRowContext(ctx, "SELECT group_concat(id || '/' || parent_id, ';' ORDER BY id) FROM child").Scan(&childRows); err != nil || childRows != "10/1" {
		t.Fatal("SQLite rebuild changed the dependent rows")
	}
	if rowAmount != wantAmount || generated != 2*wantAmount || payload != "00FF80" || seq != wantSeq ||
		note != wantNote || index != 1 || trigger != 1 || child != 1 || violations != 0 ||
		parentCount != wantParents || newID != wantNewID || audit != wantAudit ||
		history != wantHistory || schemaScratch != 0 || integrity != "ok" {
		t.Fatal("SQLite rebuild schema, rows, sequence, dependencies, integrity or history differ")
	}
}
