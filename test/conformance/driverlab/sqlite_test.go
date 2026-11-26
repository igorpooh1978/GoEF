package driverlab

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
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

const sqliteBusyTimeoutMS = 1000

func openSQLite(t *testing.T) fixture {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	path := filepath.Join(t.TempDir(), "driverlab.db")
	db := openSQLitePool(t, path, "rwc")
	setSQLiteJournalMode(t, ctx, db, "wal")
	checkSQLiteConnections(t, ctx, db, "wal")
	options := sqliteCompileOptions(t, ctx, db)
	optionsDigest := sha256.Sum256([]byte(strings.Join(options, "\n")))
	return fixture{
		db:             db,
		connector:      sqliteFixtureConnector{drv: db.Driver(), dsn: sqliteFixtureDSN(path, "rw")},
		bind:           func(i int) string { return "?" + strconv.Itoa(i) },
		code:           sqliteNativeCode,
		uniqueCode:     "2067", // SQLITE_CONSTRAINT_UNIQUE
		foreignKeyCode: "787",  // SQLITE_CONSTRAINT_FOREIGNKEY
		facts: map[string]string{
			"engine_version":         sqliteVersion,
			"source_id":              sqliteSourceID,
			"journal_mode":           "wal",
			"foreign_keys":           "1",
			"busy_timeout_ms":        strconv.Itoa(sqliteBusyTimeoutMS),
			"compile_options_count":  strconv.Itoa(len(options)),
			"compile_options_sha256": hex.EncodeToString(optionsDigest[:]),
		},
	}
}

// Both pinned SQLite drivers implement Driver.Open. The older runtime does
// not expose a driver.Connector constructor.
type sqliteFixtureConnector struct {
	drv driver.Driver
	dsn string
}

func (c sqliteFixtureConnector) Driver() driver.Driver { return c.drv }

func (c sqliteFixtureConnector) Connect(ctx context.Context) (driver.Conn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return c.drv.Open(c.dsn)
}

func sqliteFixtureDSN(filename, mode string) string {
	path := filepath.ToSlash(filename)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path // SQLite file URIs spell Windows drive paths as /C:/...
	}
	settings := url.Values{
		"_pragma": {"foreign_keys(1)", "busy_timeout(" + strconv.Itoa(sqliteBusyTimeoutMS) + ")"},
		"mode":    {mode},
	}
	dsn := url.URL{Scheme: "file", Path: path, RawQuery: settings.Encode()}
	return dsn.String()
}

func openSQLitePool(t *testing.T, filename, mode string) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", sqliteFixtureDSN(filename, mode))
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
	return db
}

func setSQLiteJournalMode(t *testing.T, ctx context.Context, db *sql.DB, want string) {
	t.Helper()
	var query string
	switch want {
	case "delete":
		query = "PRAGMA journal_mode=DELETE"
	case "wal":
		query = "PRAGMA journal_mode=WAL"
	default:
		t.Fatal("invalid SQLite journal mode")
	}
	var journalMode string
	if err := db.QueryRowContext(ctx, query).Scan(&journalMode); err != nil {
		t.Fatalf("set SQLite journal mode failed (native code %q)", sqliteNativeCode(err))
	}
	if journalMode != want {
		t.Fatal("SQLite fixture did not enter the requested journal mode")
	}
}

func checkSQLiteConnections(t *testing.T, ctx context.Context, db *sql.DB, wantJournalMode string) {
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
		if journalMode != wantJournalMode || foreignKeys != 1 || busyTimeout != sqliteBusyTimeoutMS {
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
