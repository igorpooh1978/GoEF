package mysql

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/igorpooh1978/GoEF/goeferr"
)

func TestPoolAndOpenRedaction(t *testing.T) {
	for _, db := range []*sql.DB{nil, sql.OpenDB(wrongConnector{})} {
		if _, err := New(db); !errors.Is(err, goeferr.ErrInvalidState) {
			t.Fatalf("wrong pool accepted: %v", err)
		}
	}
	b, err := Open("goef:goef@tcp(127.0.0.1:1)/goef")
	if err != nil {
		t.Fatal(err)
	}
	defer b.DB().Close()
	if d := b.Descriptor(); d.ID != "mysql" || !d.Compatible() {
		t.Fatalf("descriptor: %+v", d)
	}
	if _, err := Open("private:malformed"); !errors.Is(err, goeferr.ErrConnectionUnavailable) ||
		strings.Contains(err.Error(), "private") {
		t.Fatalf("invalid DSN leaked: %v", err)
	}
}

func TestTypedErrorMapping(t *testing.T) {
	for _, tc := range []struct {
		code uint16
		want error
	}{
		{1062, goeferr.ErrUniqueViolation},
		{1451, goeferr.ErrForeignKeyViolation},
		{1452, goeferr.ErrForeignKeyViolation},
		{1213, goeferr.ErrDeadlock},
		{1205, goeferr.ErrTimeout},
		{1317, nil},
	} {
		native := &mysqlDriver.MySQLError{Number: tc.code, Message: "private"}
		if got, code := classify(fmt.Errorf("outer: %w", native)); got != tc.want || code != strconv.Itoa(int(tc.code)) {
			t.Fatalf("code %d mapped to %v / %s, want %v", tc.code, got, code, tc.want)
		}
	}
	if got, code := classify(errors.New("fake 1062")); got != nil || code != "" {
		t.Fatalf("unstructured error mapped to %v / %s", got, code)
	}
}

type wrongConnector struct{}

func (wrongConnector) Connect(context.Context) (driver.Conn, error) { return nil, driver.ErrBadConn }
func (wrongConnector) Driver() driver.Driver                        { return nil }
