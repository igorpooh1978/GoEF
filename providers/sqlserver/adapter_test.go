package sqlserver

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/igorpooh1978/GoEF/goeferr"
	mssql "github.com/microsoft/go-mssqldb"
)

func TestPoolAndOpenRedaction(t *testing.T) {
	for _, db := range []*sql.DB{nil, sql.OpenDB(wrongConnector{})} {
		if _, err := New(db); !errors.Is(err, goeferr.ErrInvalidState) {
			t.Fatalf("wrong pool accepted: %v", err)
		}
	}
	b, err := Open("sqlserver://goef:goef@127.0.0.1:1?database=goef")
	if err != nil {
		t.Fatal(err)
	}
	defer b.DB().Close()
	if d := b.Descriptor(); d.ID != "sqlserver" || !d.Compatible() {
		t.Fatalf("descriptor: %+v", d)
	}
	if _, err := Open("sqlserver://private:word@%"); !errors.Is(err, goeferr.ErrConnectionUnavailable) ||
		strings.Contains(err.Error(), "private") {
		t.Fatalf("invalid DSN leaked: %v", err)
	}
}

func TestTypedErrorMapping(t *testing.T) {
	for _, tc := range []struct {
		all    []mssql.Error
		number int32
		want   error
		code   string
	}{
		{nil, 2627, goeferr.ErrUniqueViolation, "2627"},
		{[]mssql.Error{{Number: 2601}, {Number: 3621}}, 3621, goeferr.ErrUniqueViolation, "2601"},
		{[]mssql.Error{{Number: 2601}, {Number: 2627}}, 2627, goeferr.ErrUniqueViolation, "2627"},
		{nil, 547, nil, "547"},
		{[]mssql.Error{{Number: 547}, {Number: 3621}}, 3621, nil, "547"},
		{[]mssql.Error{{Number: 3621}}, 3621, nil, "3621"},
		{nil, 1205, goeferr.ErrDeadlock, "1205"},
		{[]mssql.Error{{Number: 2627}, {Number: 1205}}, 1205, nil, ""},
		{[]mssql.Error{{Number: 547}, {Number: 102}}, 102, nil, ""},
	} {
		native := mssql.Error{Number: tc.number, All: tc.all, Message: "private"}
		if got, code := classify(fmt.Errorf("outer: %w", native)); got != tc.want || code != tc.code {
			t.Fatalf("code %d mapped to %v / %q, want %v / %q", tc.number, got, code, tc.want, tc.code)
		}
	}
	if got, code := classify(errors.New("fake 2627")); got != nil || code != "" {
		t.Fatalf("unstructured error mapped to %v / %s", got, code)
	}
}

type wrongConnector struct{}

func (wrongConnector) Connect(context.Context) (driver.Conn, error) { return nil, driver.ErrBadConn }
func (wrongConnector) Driver() driver.Driver                        { return nil }
