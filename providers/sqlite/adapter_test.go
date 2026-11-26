package sqlite

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/igorpooh1978/GoEF/goeferr"
	"github.com/igorpooh1978/GoEF/provider"
)

func openFixture(t *testing.T) *Backend {
	t.Helper()
	for _, db := range []*sql.DB{nil, sql.OpenDB(wrongConnector{})} {
		if _, err := New(db); !errors.Is(err, goeferr.ErrInvalidState) {
			t.Fatalf("wrong pool accepted: %v", err)
		}
	}
	path := filepath.ToSlash(filepath.Join(t.TempDir(), "adapter.db"))
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	dsn := (&url.URL{
		Scheme: "file", Path: path,
		RawQuery: "_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)",
	}).String()
	db, err := Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.DB().Close(); err != nil {
			t.Error(err)
		}
	})
	if d := db.Descriptor(); d.ID != "sqlite" || !d.Compatible() {
		t.Fatalf("descriptor: %+v", d)
	}
	return db
}

func acquire(t *testing.T, db *Backend) provider.Lease {
	t.Helper()
	lease, err := db.Acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := lease.Release(t.Context(), provider.Reuse); err != nil {
			t.Error(err)
		}
	})
	return lease
}

func nativeWrite(t *testing.T, lease provider.Lease, sqlText string, kind error, code string) {
	t.Helper()
	_, err := lease.Exec(t.Context(), provider.Command{SQL: sqlText}, nil)
	if kind == nil {
		if err != nil {
			t.Fatal(err)
		}
		return
	}
	var detail *goeferr.ErrorDetail
	if !errors.Is(err, kind) || !errors.As(err, &detail) || detail.NativeCode() != code ||
		detail.Provider() != "sqlite" || strings.Contains(err.Error(), "fixture") {
		t.Fatalf("expected %s/%v, got %+v (%v)", code, kind, detail, err)
	}
}

func TestSQLiteTransportClassifiesRealFailures(t *testing.T) {
	db := openFixture(t)
	l := acquire(t, db)
	nativeWrite(t, l, "CREATE TABLE fixture_parent (id INTEGER PRIMARY KEY)", nil, "")
	nativeWrite(t, l, "CREATE TABLE fixture (id INTEGER PRIMARY KEY, code TEXT UNIQUE NOT NULL, amount INTEGER NOT NULL CHECK(amount > 0), parent INTEGER REFERENCES fixture_parent(id))", nil, "")
	nativeWrite(t, l, "INSERT INTO fixture VALUES (1, 'fixture-key', 1, NULL)", nil, "")
	nativeWrite(t, l, "INSERT INTO fixture VALUES (1, 'second', 1, NULL)", goeferr.ErrUniqueViolation, "1555")
	nativeWrite(t, l, "INSERT INTO fixture VALUES (2, 'fixture-key', 1, NULL)", goeferr.ErrUniqueViolation, "2067")
	nativeWrite(t, l, "INSERT INTO fixture VALUES (2, NULL, 1, NULL)", goeferr.ErrNotNullViolation, "1299")
	nativeWrite(t, l, "INSERT INTO fixture VALUES (2, 'second', 0, NULL)", goeferr.ErrCheckViolation, "275")
	nativeWrite(t, l, "INSERT INTO fixture VALUES (2, 'second', 1, 99)", goeferr.ErrForeignKeyViolation, "787")
	nativeWrite(t, l, "SELECT * FROM fixture_missing", goeferr.ErrUnknownDriver, "1")
}

func TestSQLiteWALSnapshotAndExactBusy(t *testing.T) {
	db := openFixture(t)
	writer := acquire(t, db)
	reader := acquire(t, db)
	nativeWrite(t, writer, "CREATE TABLE fixture (id INTEGER PRIMARY KEY)", nil, "")
	nativeWrite(t, writer, "BEGIN IMMEDIATE", nil, "")
	nativeWrite(t, reader, "BEGIN IMMEDIATE", goeferr.ErrLockUnavailable, "5")
	nativeWrite(t, writer, "ROLLBACK", nil, "")
	nativeWrite(t, reader, "BEGIN DEFERRED", nil, "")
	command := provider.Command{SQL: "SELECT COUNT(*) FROM fixture", Columns: []provider.Column{{Kind: provider.KindInteger}}}
	rows, err := reader.Query(t.Context(), command, nil)
	if err != nil {
		t.Fatal(err)
	}
	if row, err := rows.Next(); err != nil || len(row) != 1 || row[0] != provider.Integer(0) {
		t.Fatalf("snapshot: %v / %v", row, err)
	}
	if _, err := rows.Next(); !errors.Is(err, io.EOF) {
		t.Fatalf("unexpected extra result: %v", err)
	}
	if err := rows.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
	nativeWrite(t, writer, "INSERT INTO fixture VALUES (1)", nil, "")
	nativeWrite(t, reader, "INSERT INTO fixture VALUES (2)", goeferr.ErrSerializationFailure, "517")
	nativeWrite(t, reader, "ROLLBACK", nil, "")
	if got, code := classify(errors.New("fake 1555")); got != nil || code != "" {
		t.Fatalf("unstructured error mapped to %v / %s", got, code)
	}
}

type wrongConnector struct{}

func (wrongConnector) Connect(context.Context) (driver.Conn, error) { return nil, driver.ErrBadConn }
func (wrongConnector) Driver() driver.Driver                        { return nil }
