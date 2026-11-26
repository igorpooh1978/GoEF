package driverlab

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/igorpooh1978/GoEF/provider"
)

type fixture struct {
	db                         *sql.DB
	bind                       func(int) string
	code                       func(error) string
	uniqueCode, foreignKeyCode string
	facts                      map[string]string
}

type scenario struct {
	fixture
	t                *testing.T
	provider         string
	ctx              context.Context
	writer, observer *sql.Conn
	writeErr         error
	readBackend      provider.Backend
	canceledReader   *spiScenarioReader
}

func (s *scenario) setup() {
	s.t.Helper()
	s.fixture = providers[s.provider](s.t)
	ctx, cancel := context.WithTimeout(s.t.Context(), 30*time.Second)
	s.ctx = ctx
	s.t.Cleanup(cancel)

	s.writer = s.lease()
	s.observer = s.lease()
	if s.provider != "sqlite" {
		s.verifyMarker(s.ctx, s.writer.QueryContext)
	}
	s.createTable("CREATE TABLE goef_driver_parent ("+
		"id INTEGER NOT NULL PRIMARY KEY, code VARCHAR(40) NOT NULL, "+
		"CONSTRAINT goef_driver_uq UNIQUE (code))", "DROP TABLE goef_driver_parent")
	s.createTable("CREATE TABLE goef_driver_child ("+
		"id INTEGER NOT NULL PRIMARY KEY, parent_id INTEGER NOT NULL, "+
		"CONSTRAINT goef_driver_fk FOREIGN KEY (parent_id) REFERENCES goef_driver_parent(id))",
		"DROP TABLE goef_driver_child")
}

func (s *scenario) lease() *sql.Conn {
	s.t.Helper()
	conn, err := s.db.Conn(s.ctx)
	if err != nil {
		s.t.Fatal("could not lease a fixture connection")
	}
	s.t.Cleanup(func() {
		if err := conn.Close(); err != nil && !errors.Is(err, sql.ErrConnDone) {
			s.t.Error("could not release a fixture connection")
		}
	})
	return conn
}

func (s *scenario) verifyMarker(ctx context.Context, queryContext func(context.Context, string, ...any) (*sql.Rows, error)) {
	s.t.Helper()
	token := os.Getenv("GOEF_DRIVERLAB_TOKEN")
	if len(token) != 64 || strings.Trim(token, "0123456789abcdef") != "" {
		s.t.Fatal("invalid target marker configuration")
	}
	query := "SELECT token FROM goef_driver_marker"
	if s.provider == "mysql" {
		query = "SELECT token FROM goef_marker.target"
	} else if s.provider == "sqlserver" {
		query = "SELECT token FROM dbo.goef_driver_marker"
	}
	rows, err := queryContext(ctx, query)
	if err != nil {
		s.t.Fatal("could not read target marker")
	}
	defer rows.Close()
	var value string
	if !rows.Next() || rows.Scan(&value) != nil || value != token || rows.Next() || rows.Err() != nil {
		s.t.Fatal("target marker mismatch")
	}
}

// Register teardown only after the CREATE succeeds. An existing table is never
// adopted, and each DROP runs in reverse creation order on the same lease.
func (s *scenario) createTable(create, drop string) {
	s.t.Helper()
	if _, err := s.writer.ExecContext(s.ctx, create); err != nil {
		s.t.Fatal("could not create a fixture table")
	}
	s.t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := s.writer.ExecContext(ctx, drop); err != nil {
			s.t.Error("could not remove a fixture table")
		}
	})
}

func (s *scenario) insertParentSQL() string {
	return "INSERT INTO goef_driver_parent (id, code) VALUES (" + s.bind(1) + ", " + s.bind(2) + ")"
}

func (s *scenario) insertChildSQL() string {
	return "INSERT INTO goef_driver_child (id, parent_id) VALUES (" + s.bind(1) + ", " + s.bind(2) + ")"
}

func (s *scenario) commitParent() {
	s.transaction(1, "original", (*sql.Tx).Commit)
}

func (s *scenario) rollbackParent() {
	s.transaction(2, "second", (*sql.Tx).Rollback)
}

func (s *scenario) transaction(id int, code string, finish func(*sql.Tx) error) {
	s.t.Helper()
	tx, err := s.writer.BeginTx(s.ctx, nil)
	if err != nil {
		s.t.Fatal("could not start a transaction")
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			s.t.Error("could not release unfinished transaction")
		}
	}()
	if _, err := tx.ExecContext(s.ctx, s.insertParentSQL(), id, code); err != nil {
		s.t.Fatal("transactional insert failed")
	}
	if err := finish(tx); err != nil {
		s.t.Fatal("transaction did not finish as requested")
	}
}

func (s *scenario) seedParent() {
	s.t.Helper()
	if _, err := s.writer.ExecContext(s.ctx, s.insertParentSQL(), 1, "original"); err != nil {
		s.t.Fatal("could not seed parent")
	}
}

func (s *scenario) seedFamily() {
	s.t.Helper()
	s.seedParent()
	if _, err := s.writer.ExecContext(s.ctx, s.insertChildSQL(), 1, 1); err != nil {
		s.t.Fatal("could not seed child")
	}
}

func (s *scenario) duplicateParent() {
	_, s.writeErr = s.writer.ExecContext(s.ctx, s.insertParentSQL(), 2, "original")
}

func (s *scenario) orphanChild() {
	_, s.writeErr = s.writer.ExecContext(s.ctx, s.insertChildSQL(), 2, 99)
}

func (s *scenario) checkUniqueCode() {
	s.checkNativeCode(s.uniqueCode)
}

func (s *scenario) checkForeignKeyCode() {
	s.checkNativeCode(s.foreignKeyCode)
}

func (s *scenario) checkNativeCode(want string) {
	s.t.Helper()
	if s.writeErr == nil {
		s.t.Fatal("constraint violation succeeded")
	}
	if want == "" || s.code(s.writeErr) != want {
		s.t.Fatal("unexpected native constraint code")
	}
	if s.facts == nil {
		s.facts = make(map[string]string)
	}
	s.facts["native_code"] = want
}

func (s *scenario) checkParent() {
	s.t.Helper()
	s.checkParents("original")
}

func (s *scenario) checkParents(codes ...string) {
	s.t.Helper()
	rows, err := s.observer.QueryContext(s.ctx, "SELECT id, code FROM goef_driver_parent ORDER BY id")
	if err != nil {
		s.t.Fatal("observer could not read parents")
	}
	defer rows.Close()
	for i, want := range codes {
		var id int
		var code string
		if !rows.Next() || rows.Scan(&id, &code) != nil || id != i+1 || code != want {
			s.t.Fatal("observer did not find the expected parent")
		}
	}
	if rows.Next() || rows.Err() != nil {
		s.t.Fatal("observer did not find exactly the expected parents")
	}
}

func (s *scenario) checkFamily() {
	s.t.Helper()
	s.checkParent()
	rows, err := s.observer.QueryContext(s.ctx, "SELECT id, parent_id FROM goef_driver_child")
	if err != nil {
		s.t.Fatal("observer could not read children")
	}
	defer rows.Close()
	var id, parentID int
	if !rows.Next() || rows.Scan(&id, &parentID) != nil || id != 1 || parentID != 1 || rows.Next() || rows.Err() != nil {
		s.t.Fatal("observer did not find exactly the original child")
	}
}
