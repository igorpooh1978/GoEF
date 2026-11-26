package postgres

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/igorpooh1978/GoEF/goeferr"
	"github.com/igorpooh1978/GoEF/provider"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/stdlib"
)

func TestPoolAndOpenRedaction(t *testing.T) {
	for _, db := range []*sql.DB{nil, sql.OpenDB(wrongConnector{})} {
		if _, err := New(db); !errors.Is(err, goeferr.ErrInvalidState) {
			t.Fatalf("wrong pool accepted: %v", err)
		}
	}
	b, err := Open("host=127.0.0.1 port=1 dbname=goef_unit user=goef_user")
	if err != nil {
		t.Fatal(err)
	}
	defer b.DB().Close()
	if d := b.Descriptor(); d.ID != "postgres" || !d.Compatible() {
		t.Fatalf("descriptor: %+v", d)
	}
	if _, err := Open("postgres://private:word@%"); !errors.Is(err, goeferr.ErrConnectionUnavailable) ||
		strings.Contains(err.Error(), "private") {
		t.Fatalf("invalid DSN leaked: %v", err)
	}
	if err := closePhysical(context.Background(), struct{}{}); !errors.Is(err, goeferr.ErrInvalidState) {
		t.Fatalf("unexpected driver connection: %v", err)
	}
}

func TestTypedErrorMapping(t *testing.T) {
	for _, tc := range []struct {
		code string
		want error
	}{
		{"23505", goeferr.ErrUniqueViolation},
		{"23503", goeferr.ErrForeignKeyViolation},
		{"23502", goeferr.ErrNotNullViolation},
		{"23514", goeferr.ErrCheckViolation},
		{"40P01", goeferr.ErrDeadlock},
		{"40001", goeferr.ErrSerializationFailure},
		{"57014", nil},
	} {
		native := &pgconn.PgError{Code: tc.code, Message: "private"}
		if got, code := classify(fmt.Errorf("outer: %w", native)); got != tc.want || code != tc.code {
			t.Fatalf("code %s mapped to %v / %s, want %v", tc.code, got, code, tc.want)
		}
	}
	if got, code := classify(errors.New("fake 23505")); got != nil || code != "" {
		t.Fatalf("unstructured error mapped to %v / %s", got, code)
	}
}

type wrongConnector struct{}

func (wrongConnector) Connect(context.Context) (driver.Conn, error) { return nil, driver.ErrBadConn }
func (wrongConnector) Driver() driver.Driver                        { return nil }

func TestReleaseDiscardsWrappedNativeConnection(t *testing.T) {
	connector := &wrappedConnector{}
	db := sql.OpenDB(connector)
	defer db.Close()

	backend, err := New(db)
	if err != nil {
		t.Fatal(err)
	}
	lease, err := backend.Acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	err = lease.Release(t.Context(), provider.Discard)
	var detail *goeferr.ErrorDetail
	if !errors.Is(err, goeferr.ErrUnknownDriver) || !errors.As(err, &detail) ||
		detail.Phase() != goeferr.PhaseRelease || detail.Provider() != "postgres" ||
		detail.NativeCode() != "" || err.Error() != goeferr.ErrUnknownDriver.Error() {
		t.Fatalf("expected redacted typed cleanup failure, got %v", err)
	}
	if connector.closed != 1 {
		t.Fatalf("discard must close the wrapped connection, got %d closes", connector.closed)
	}
	lease, err = backend.Acquire(t.Context())
	if err != nil || connector.opened != 2 {
		t.Fatalf("pool did not supply a replacement, got %d opens, %v", connector.opened, err)
	}
	if err := lease.Release(t.Context(), provider.Reuse); err != nil {
		t.Fatal(err)
	}
}

type wrappedConnector struct {
	opened, closed int
}

func (c *wrappedConnector) Driver() driver.Driver { return stdlib.GetDefaultDriver() }
func (c *wrappedConnector) Connect(context.Context) (driver.Conn, error) {
	c.opened++
	return opaqueConn{Conn: &testConn{onClose: func() { c.closed++ }}}, nil
}

// An ordinary driver.Conn wrapper can hide the concrete *stdlib.Conn type.
type opaqueConn struct{ driver.Conn }

type testConn struct{ onClose func() }

func (*testConn) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (*testConn) Begin() (driver.Tx, error)           { return nil, driver.ErrSkip }
func (c *testConn) Close() error {
	c.onClose()
	return nil
}
