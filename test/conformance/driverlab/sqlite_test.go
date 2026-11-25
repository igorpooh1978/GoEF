package driverlab

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/url"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"modernc.org/sqlite"
)

const (
	sqliteVersion       = "3.54.1"
	sqliteSourceID      = "2026-10-16 14:45:20 555e07ef33f69a0957d24d8e555b702464a131d6f1607f4c3c1e4551e8e6ca76"
	sqliteBusyTimeoutMS = 1000
)

func openSQLite(t *testing.T) fixture {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	path := filepath.ToSlash(filepath.Join(t.TempDir(), "driverlab.db"))
	if !strings.HasPrefix(path, "/") {
		path = "/" + path // SQLite file URIs spell Windows drive paths as /C:/...
	}
	settings := url.Values{
		"_pragma": {"foreign_keys(1)", "busy_timeout(" + strconv.Itoa(sqliteBusyTimeoutMS) + ")"},
	}
	dsn := url.URL{Scheme: "file", Path: path, RawQuery: settings.Encode()}
	db, err := sql.Open("sqlite", dsn.String())
	if err != nil {
		t.Fatal("open isolated SQLite pool failed")
	}
	t.Cleanup(func() {
		if db.Close() != nil {
			t.Error("close isolated SQLite pool failed")
		}
	})
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(2)

	var journalMode string
	if err := db.QueryRowContext(ctx, "PRAGMA journal_mode=WAL").Scan(&journalMode); err != nil {
		t.Fatalf("enable SQLite WAL failed (native code %q)", sqliteNativeCode(err))
	}
	if journalMode != "wal" {
		t.Fatal("SQLite fixture requires WAL journal mode")
	}

	checkSQLiteConnections(t, ctx, db)
	options := sqliteCompileOptions(t, ctx, db)
	optionsDigest := sha256.Sum256([]byte(strings.Join(options, "\n")))
	return fixture{
		db:             db,
		bind:           func(i int) string { return "?" + strconv.Itoa(i) },
		code:           sqliteNativeCode,
		uniqueCode:     "2067", // SQLITE_CONSTRAINT_UNIQUE
		foreignKeyCode: "787",  // SQLITE_CONSTRAINT_FOREIGNKEY
		facts: map[string]string{
			"engine_version":         sqliteVersion,
			"source_id":              sqliteSourceID,
			"journal_mode":           journalMode,
			"foreign_keys":           "1",
			"busy_timeout_ms":        strconv.Itoa(sqliteBusyTimeoutMS),
			"compile_options_count":  strconv.Itoa(len(options)),
			"compile_options_sha256": hex.EncodeToString(optionsDigest[:]),
		},
	}
}

func checkSQLiteConnections(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()

	// Keeping each lease until the end forces database/sql to open two physical connections.
	for i := 1; i <= 2; i++ {
		conn, err := db.Conn(ctx)
		if err != nil {
			t.Fatalf("acquire SQLite connection %d failed (native code %q)", i, sqliteNativeCode(err))
		}
		defer func() {
			if conn.Close() != nil {
				t.Errorf("release SQLite connection %d failed", i)
			}
		}()

		const query = `SELECT sqlite_version(), sqlite_source_id(), journal_mode, foreign_keys, timeout
			FROM pragma_journal_mode(), pragma_foreign_keys(), pragma_busy_timeout()`
		var version, sourceID, journalMode string
		var foreignKeys, busyTimeout int
		if err := conn.QueryRowContext(ctx, query).Scan(&version, &sourceID, &journalMode, &foreignKeys, &busyTimeout); err != nil {
			t.Fatalf("inspect SQLite connection %d failed (native code %q)", i, sqliteNativeCode(err))
		}
		if version != sqliteVersion || sourceID != sqliteSourceID {
			t.Fatalf("SQLite connection %d does not match the pinned engine identity", i)
		}
		if journalMode != "wal" || foreignKeys != 1 || busyTimeout != sqliteBusyTimeoutMS {
			t.Fatalf("SQLite connection %d has unexpected journal mode, foreign keys, or busy timeout", i)
		}
	}
}

func sqliteCompileOptions(t *testing.T, ctx context.Context, db *sql.DB) []string {
	t.Helper()

	rows, err := db.QueryContext(ctx, "PRAGMA compile_options")
	if err != nil {
		t.Fatalf("query SQLite compile options failed (native code %q)", sqliteNativeCode(err))
	}
	defer func() {
		if rows.Close() != nil {
			t.Error("close SQLite compile options failed")
		}
	}()

	var options []string
	for rows.Next() {
		var option string
		if err := rows.Scan(&option); err != nil {
			t.Fatal("scan SQLite compile option failed")
		}
		options = append(options, option)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read SQLite compile options failed (native code %q)", sqliteNativeCode(err))
	}
	if len(options) == 0 {
		t.Fatal("SQLite reported no compile options")
	}
	slices.Sort(options)
	return options
}

func sqliteNativeCode(err error) string {
	var native *sqlite.Error
	if errors.As(err, &native) {
		return strconv.Itoa(native.Code())
	}
	return ""
}
