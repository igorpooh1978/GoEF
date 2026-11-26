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

var savepointDialects = map[string]transaction.Dialect{
	"postgres":  transaction.DialectPostgreSQL,
	"sqlite":    transaction.DialectSQLite,
	"sqlserver": transaction.DialectSQLServer,
	"mysql":     transaction.DialectMySQL,
}

func TestSavepointAndExternalTransaction(t *testing.T) {
	if os.Getenv("GOEF_DRIVERLAB_SAVEPOINT_TX") != "1" {
		t.Skip("enable the scenarios using the isolated lab runner")
	}
	provider, profile := os.Getenv("GOEF_DRIVERLAB_PROVIDER"), os.Getenv("GOEF_DRIVERLAB_PROFILE")
	engine, ok := ownedTxEngines[provider]
	if !ok || profile == "" {
		t.Fatal("transaction scenarios require a provider and profile")
	}
	if provider == "sqlite" {
		versions := map[string]string{"sqlite-3.51": "3.51.3", "sqlite-3.54": "3.54.1"}
		if versions[profile] != sqliteVersion {
			t.Fatal("SQLite fixture runtime does not match the selected profile")
		}
	}

	for _, tc := range []struct {
		id  string
		run func(*savepointTxFixture, int64)
	}{
		{"TX-SAVEPOINT-001-S03", (*savepointTxFixture).ownedSavepoints},
		{"TX-OWN-001-S01", (*savepointTxFixture).externalOwnership},
	} {
		var version string
		passed := t.Run(tc.id, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			s := savepointTxFixture{
				ownedTxFixture: ownedTxFixture{
					scenario: scenario{fixture: providers[provider](t), t: t, provider: provider, ctx: ctx},
					engine:   engine,
				},
				dialect: savepointDialects[provider],
			}
			session := s.setupPool()
			tc.run(&s, session)
			if s.db.Stats().InUse != 0 {
				t.Fatal("transaction fixture did not return all leases")
			}
			version = s.facts[engine.version]
			if version == "" {
				t.Fatal("transaction fixture did not observe an engine version")
			}
		})
		if !passed || version == "" {
			continue
		}
		result, err := json.Marshal(map[string]string{
			"provider": provider, "profile": profile, "engine_version": version,
			"scenario": tc.id, "status": "PASS",
		})
		if err != nil {
			t.Fatal("could not encode transaction evidence")
		}
		fmt.Printf("GOEF_SAVEPOINT_TX_RESULT=%s\n", result)
	}
}

type savepointTxFixture struct {
	ownedTxFixture
	dialect transaction.Dialect
}

func (s *savepointTxFixture) insertOnOriginalSession() string {
	return "INSERT INTO " + ownedTxTable + " (id, code, session_id) SELECT " +
		s.bind(1) + ", " + s.bind(2) + ", " + s.engine.session +
		" WHERE " + s.engine.session + " = " + s.bind(3)
}

func (s *savepointTxFixture) requireSavepoint(point *transaction.Savepoint, err error) *transaction.Savepoint {
	s.t.Helper()
	if err != nil || point == nil {
		s.t.Fatal("could not establish the savepoint")
	}
	return point
}

func (s *savepointTxFixture) requireRetired(point *transaction.Savepoint) {
	s.t.Helper()
	if !errors.Is(point.Rollback(s.ctx), goeferr.ErrInvalidState) ||
		!errors.Is(point.Release(s.ctx), goeferr.ErrInvalidState) {
		s.t.Fatal("a retired savepoint accepted rollback or release")
	}
}

func (s *savepointTxFixture) ownedSavepoints(session int64) {
	s.t.Helper()
	tx, err := transaction.Begin(s.ctx, s.db)
	if err != nil || tx == nil {
		s.t.Fatal("could not begin the owned savepoint transaction")
	}
	defer func() {
		if err := tx.Close(); err != nil {
			s.t.Error("could not close the owned savepoint transaction")
		}
	}()
	insert := s.insertOnOriginalSession()
	s.requireWrite(tx.Exec(s.ctx, insert, 1, "A", session))
	outer := s.requireSavepoint(tx.Savepoint(s.ctx, s.dialect))
	s.requireWrite(tx.Exec(s.ctx, insert, 2, "B", session))
	inner := s.requireSavepoint(tx.Savepoint(s.ctx, s.dialect))
	s.requireWrite(tx.Exec(s.ctx, insert, 3, "C", session))
	if !errors.Is(outer.Release(s.ctx), goeferr.ErrInvalidState) {
		s.t.Fatal("release accepted a savepoint while a younger point was active")
	}
	if err := outer.Rollback(s.ctx); err != nil {
		s.t.Fatal("could not roll back to the outer savepoint")
	}
	s.requireRetired(inner)

	// Reusing the target proves that rollback-to preserved it.
	s.requireWrite(tx.Exec(s.ctx, insert, 4, "D", session))
	if err := outer.Rollback(s.ctx); err != nil {
		s.t.Fatal("could not roll back to the same savepoint again")
	}
	if err := outer.Release(s.ctx); err != nil {
		s.t.Fatal("could not release the remaining savepoint")
	}
	s.requireRetired(outer)
	if err := tx.Commit(); err != nil || tx.State() != transaction.StateCommitted {
		s.t.Fatal("savepoint rollback did not leave the outer transaction able to commit")
	}
	s.observeCommittedRow(session)
}

func (s *savepointTxFixture) externalOwnership(session int64) {
	s.t.Helper()
	owner := s.lease()
	defer s.closeConn(owner)
	if s.sessionID(owner) != session {
		s.t.Fatal("external owner did not receive the expected physical connection")
	}
	native, err := owner.BeginTx(s.ctx, nil)
	if err != nil {
		s.t.Fatal("could not begin the caller-owned transaction")
	}
	defer func() {
		if err := native.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			s.t.Error("could not clean up the caller-owned transaction")
		}
	}()
	external, err := transaction.NewExternal(native)
	if err != nil || external == nil {
		s.t.Fatal("could not wrap the caller-owned transaction")
	}
	defer func() {
		if err := external.Close(); err != nil {
			s.t.Error("could not close the external transaction coordinator")
		}
	}()
	first, err := external.Enlist()
	if err != nil || first == nil {
		s.t.Fatal("could not enlist the first participant")
	}
	defer s.detach(first)
	second, err := external.Enlist()
	if err != nil || second == nil {
		s.t.Fatal("could not enlist the second participant")
	}
	defer s.detach(second)

	insert := s.insertOnOriginalSession()
	s.requireWrite(first.Exec(s.ctx, insert, 1, "A", session))
	point := s.requireSavepoint(external.Savepoint(s.ctx, s.dialect))
	s.requireWrite(second.Exec(s.ctx, insert, 2, "B", session))
	if err := point.Rollback(s.ctx); err != nil {
		s.t.Fatal("could not roll back to a savepoint in the caller-owned transaction")
	}
	if err := point.Release(s.ctx); err != nil {
		s.t.Fatal("could not release a savepoint in the caller-owned transaction")
	}
	s.requireRetired(point)
	s.detach(first)
	if _, err := first.Exec(s.ctx, insert, 3, "C", session); !errors.Is(err, goeferr.ErrInvalidState) {
		s.t.Fatal("a detached participant accepted a command")
	}
	s.requireWrite(second.Exec(s.ctx, insert, 3, "C", session))
	abandoned := s.requireSavepoint(external.Savepoint(s.ctx, s.dialect))
	s.detach(second)
	if _, err := second.Exec(s.ctx, insert, 4, "D", session); !errors.Is(err, goeferr.ErrInvalidState) {
		s.t.Fatal("a detached participant accepted a command")
	}
	if err := external.Close(); err != nil {
		s.t.Fatal("could not close the external transaction coordinator")
	}
	s.requireRetired(abandoned)
	if _, err := external.Enlist(); !errors.Is(err, goeferr.ErrInvalidState) {
		s.t.Fatal("a closed coordinator accepted a new participant")
	}

	s.requireOwnerRows(native, session)
	if err := native.Rollback(); err != nil {
		s.t.Fatal("only the owner should be able to roll back the native transaction")
	}
	if s.sessionID(owner) != session || s.db.Stats().InUse != 1 {
		s.t.Fatal("participant cleanup released or replaced the owner's connection")
	}
	s.requireIndependentObserverEmpty(session)
}

func (s *savepointTxFixture) detach(participant *transaction.Participant) {
	s.t.Helper()
	if err := participant.Close(); err != nil {
		s.t.Error("could not detach the transaction participant")
	}
}

func (s *savepointTxFixture) requireOwnerRows(native *sql.Tx, session int64) {
	s.t.Helper()
	rows, err := native.QueryContext(s.ctx, "SELECT id, code, session_id FROM "+ownedTxTable+" ORDER BY id")
	if err != nil {
		s.t.Fatal("the owner could not read pending work after both detachments")
	}
	defer rows.Close()
	for _, want := range []struct {
		id   int
		code string
	}{{1, "A"}, {3, "C"}} {
		var id int
		var code string
		var rowSession int64
		if !rows.Next() || rows.Scan(&id, &code, &rowSession) != nil ||
			id != want.id || code != want.code || rowSession != session {
			s.t.Fatal("the owner did not find exactly the expected pending rows")
		}
	}
	if rows.Next() || rows.Err() != nil {
		s.t.Fatal("the owner found unexpected rows or could not finish reading")
	}
}

func (s *savepointTxFixture) requireIndependentObserverEmpty(session int64) {
	s.t.Helper()
	// The owner still has its checkout, so an observer must open a second connection.
	s.db.SetMaxOpenConns(2)
	observer := s.lease()
	defer s.closeConn(observer)
	if s.provider == "sqlite" {
		var markers int
		if err := observer.QueryRowContext(s.ctx,
			"SELECT COUNT(*) FROM temp.sqlite_master WHERE name = ?1", "goef_owned_tx_session").Scan(&markers); err != nil || markers != 0 {
			s.t.Fatal("SQLite observer was not independent of the owner")
		}
	} else if s.sessionID(observer) == session {
		s.t.Fatal("observer unexpectedly received the owner's physical session")
	}
	var count int
	if err := observer.QueryRowContext(s.ctx, "SELECT COUNT(*) FROM "+ownedTxTable).Scan(&count); err != nil || count != 0 {
		s.t.Fatal("external owner rollback did not remove all pending rows")
	}
}
