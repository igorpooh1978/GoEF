package driverlab

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/igorpooh1978/GoEF/goeferr"
	"github.com/igorpooh1978/GoEF/transaction"
)

const (
	ownedTxTable        = "goef_owned_tx_rows"
	ownedTxSQLiteMarker = int64(9342)
)

type ownedTxEngine struct {
	session string
	version string
}

var ownedTxEngines = map[string]ownedTxEngine{
	"postgres":  {session: "pg_backend_pid()", version: "engine_version"},
	"sqlite":    {session: "(SELECT session_id FROM temp.goef_owned_tx_session)", version: "engine_version"},
	"sqlserver": {session: "@@SPID", version: "version"},
	"mysql":     {session: "CONNECTION_ID()", version: "server_version"},
}

func TestOwnedTransaction(t *testing.T) {
	if os.Getenv("GOEF_DRIVERLAB_OWNED_TX") != "1" {
		t.Skip("enable the owned transaction scenario using the isolated lab runner")
	}
	provider, profile := os.Getenv("GOEF_DRIVERLAB_PROVIDER"), os.Getenv("GOEF_DRIVERLAB_PROFILE")
	engine, ok := ownedTxEngines[provider]
	if !ok || profile == "" {
		t.Fatal("owned transaction scenario requires a provider and profile")
	}
	if provider == "sqlite" {
		versions := map[string]string{"sqlite-3.51": "3.51.3", "sqlite-3.54": "3.54.1"}
		if versions[profile] != sqliteVersion {
			t.Fatal("SQLite fixture runtime does not match the selected profile")
		}
	}

	var observedVersion string
	passed := t.Run(provider, func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		defer cancel()
		s := ownedTxFixture{
			scenario: scenario{fixture: providers[provider](t), t: t, provider: provider, ctx: ctx},
			engine:   engine,
		}
		session := s.setupPool()
		s.finish(session, 1, "A", transaction.StateCommitted, (*transaction.Tx).Commit)
		s.finish(session, 2, "B", transaction.StateRolledBack, (*transaction.Tx).Rollback)
		s.observeCommittedRow(session)
		if s.db.Stats().InUse != 0 {
			t.Fatal("the observer and original connection did not return to the pool")
		}
		observedVersion = s.facts[engine.version]
		if observedVersion == "" {
			t.Fatal("owned transaction fixture did not observe an engine version")
		}
	})
	if !passed || observedVersion == "" {
		return
	}
	result, err := json.Marshal(map[string]string{
		"provider": provider, "profile": profile, "engine_version": observedVersion,
		"scenario": "TX-OWN-001-S02", "status": "PASS",
	})
	if err != nil {
		t.Fatal("could not encode owned transaction evidence")
	}
	fmt.Printf("GOEF_OWNED_TX_RESULT=%s\n", result)
}

type ownedTxFixture struct {
	scenario
	engine ownedTxEngine
}

func (s *ownedTxFixture) setupPool() int64 {
	s.t.Helper()
	s.db.SetMaxOpenConns(1)
	s.db.SetMaxIdleConns(1)
	conn := s.lease()
	defer s.closeConn(conn)

	if s.provider != "sqlite" {
		s.verifyMarker(s.ctx, conn.QueryContext)
	} else {
		// SQLite TEMP state belongs only to the physical connection that creates it.
		if _, err := conn.ExecContext(s.ctx, "CREATE TEMP TABLE goef_owned_tx_session (session_id INTEGER NOT NULL)"); err != nil {
			s.t.Fatal("could not create the SQLite session marker")
		}
		if _, err := conn.ExecContext(s.ctx, "INSERT INTO temp.goef_owned_tx_session VALUES (?1)", ownedTxSQLiteMarker); err != nil {
			s.t.Fatal("could not set the SQLite session marker")
		}
	}

	create := "CREATE TABLE " + ownedTxTable + " (id INTEGER NOT NULL PRIMARY KEY, code VARCHAR(8) NOT NULL, session_id BIGINT NOT NULL)"
	if s.provider == "mysql" {
		create += " ENGINE=InnoDB"
	}
	if _, err := conn.ExecContext(s.ctx, create); err != nil {
		s.t.Fatalf("could not create the owned transaction fixture (native code %q)", s.code(err))
	}
	s.t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := s.db.ExecContext(ctx, "DROP TABLE "+ownedTxTable); err != nil {
			s.t.Error("could not remove the owned transaction fixture")
		}
	})
	session := s.sessionID(conn)
	if s.provider == "sqlite" && session != ownedTxSQLiteMarker {
		s.t.Fatal("SQLite fixture is missing its original connection marker")
	}
	return session
}

func (s *ownedTxFixture) closeConn(conn *sql.Conn) {
	s.t.Helper()
	if err := conn.Close(); err != nil {
		s.t.Error("could not return a fixture connection")
	}
}

func (s *ownedTxFixture) sessionID(conn *sql.Conn) int64 {
	s.t.Helper()
	var id int64
	if err := conn.QueryRowContext(s.ctx, "SELECT "+s.engine.session).Scan(&id); err != nil || id <= 0 {
		s.t.Fatal("could not verify the fixture connection identity")
	}
	return id
}

func (s *ownedTxFixture) finish(session, id int64, code string, terminal transaction.State, finish func(*transaction.Tx) error) {
	s.t.Helper()
	initial := s.db.Stats()
	if initial.MaxOpenConnections != 1 || initial.InUse != 0 {
		s.t.Fatal("owned transaction requires one initially available pool slot")
	}
	tx, err := transaction.Begin(s.ctx, s.db)
	if err != nil {
		s.t.Fatal("could not begin the GoEF-owned transaction")
	}
	defer func() {
		if err := tx.Close(); err != nil {
			s.t.Error("could not close the GoEF-owned transaction")
		}
	}()
	if tx.State() != transaction.StateActive || s.db.Stats().InUse != initial.InUse+1 {
		s.t.Fatal("a new GoEF transaction must be active and own the only connection")
	}

	insert := "INSERT INTO " + ownedTxTable + " (id, code, session_id) SELECT " +
		s.bind(1) + ", " + s.bind(2) + ", " + s.engine.session +
		" WHERE " + s.engine.session + " = " + s.bind(3)
	update := "UPDATE " + ownedTxTable + " SET code = " + s.bind(1) +
		" WHERE id = " + s.bind(2) + " AND session_id = " + s.engine.session +
		" AND session_id = " + s.bind(3)
	s.requireWrite(tx.Exec(s.ctx, insert, id, "pending", session))
	s.requireWrite(tx.Exec(s.ctx, update, code, id, session))

	// WaitCount records entry into database/sql's queue. The timer only bounds
	// the check, and never serves as evidence that the pool was exhausted.
	waitCtx, stopWaiting := context.WithTimeout(s.ctx, 8*time.Second)
	done := make(chan struct{})
	var returned *sql.Conn
	var waitErr error
	go func() {
		defer close(done)
		returned, waitErr = s.db.Conn(waitCtx)
	}()
	defer func() {
		stopWaiting()
		<-done
		if returned != nil {
			s.closeConn(returned)
		}
	}()
	poll := time.NewTicker(5 * time.Millisecond)
	defer poll.Stop()
	for s.db.Stats().WaitCount == initial.WaitCount {
		select {
		case <-done:
			s.t.Fatal("a second lease returned before it queued")
		case <-waitCtx.Done():
			s.t.Fatal("could not observe the waiting lease")
		case <-poll.C:
		}
	}
	select {
	case <-done:
		s.t.Fatal("another lease returned while the transaction owned the connection")
	default:
	}
	if s.db.Stats().WaitCount != initial.WaitCount+1 || s.db.Stats().InUse != initial.InUse+1 {
		s.t.Fatal("expected exactly one waiter blocked by the owned connection")
	}

	if err := finish(tx); err != nil || tx.State() != terminal {
		s.t.Fatal("GoEF transaction did not reach the requested terminal state")
	}
	select {
	case <-done:
	case <-waitCtx.Done():
		s.t.Fatal("the queued lease did not complete after transaction completion")
	}
	if waitErr != nil || returned == nil || s.sessionID(returned) != session {
		s.t.Fatal("the queued lease did not recover the same physical connection")
	}
	s.closeConn(returned)
	returned = nil
	if s.db.Stats().InUse != initial.InUse {
		s.t.Fatal("the pool did not return to its initial in-use count")
	}

	if _, err := tx.Exec(s.ctx, insert, 3, "invalid", session); !errors.Is(err, goeferr.ErrInvalidState) {
		s.t.Fatal("a terminal handle did not reject further execution")
	}
	if !errors.Is(tx.Commit(), goeferr.ErrInvalidState) || !errors.Is(tx.Rollback(), goeferr.ErrInvalidState) {
		s.t.Fatal("a terminal handle did not reject another completion")
	}
	if tx.State() != terminal {
		s.t.Fatal("a terminal operation changed the transaction state")
	}
}

func (s *ownedTxFixture) requireWrite(result sql.Result, err error) {
	s.t.Helper()
	if err != nil {
		s.t.Fatalf("transactional write failed (native code %q)", s.code(err))
	}
	if result == nil {
		s.t.Fatal("transactional write returned no result")
	}
	if count, err := result.RowsAffected(); err != nil || count != 1 {
		s.t.Fatal("transactional write did not run exactly once on the expected session")
	}
}

func (s *ownedTxFixture) observeCommittedRow(session int64) {
	s.t.Helper()
	// Hold the original connection while opening a second one to force an
	// independent physical observer. Neither transaction is still active.
	s.db.SetMaxOpenConns(2)
	original := s.lease()
	defer s.closeConn(original)
	if s.sessionID(original) != session {
		s.t.Fatal("transaction completion did not preserve the original session")
	}
	observer := s.lease()
	defer s.closeConn(observer)
	if s.provider == "sqlite" {
		var markers int
		if err := observer.QueryRowContext(s.ctx,
			"SELECT COUNT(*) FROM temp.sqlite_master WHERE name = ?1", "goef_owned_tx_session").Scan(&markers); err != nil || markers != 0 {
			s.t.Fatal("SQLite observer was not independent of the original connection")
		}
	} else if s.sessionID(observer) == session {
		s.t.Fatal("observer unexpectedly received the original physical session")
	}

	rows, err := observer.QueryContext(s.ctx, "SELECT id, code, session_id FROM "+ownedTxTable+" ORDER BY id")
	if err != nil {
		s.t.Fatal("independent observer could not read the transaction fixture")
	}
	defer rows.Close()
	var id, rowSession int64
	var code string
	if !rows.Next() || rows.Scan(&id, &code, &rowSession) != nil || id != 1 || code != "A" || rowSession != session ||
		rows.Next() || rows.Err() != nil {
		s.t.Fatal("independent observer did not find exactly the committed A row")
	}
}
