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
	saveParentTable = "goef_save_parent"
	saveChildTable  = "goef_save_child"
)

func TestSaveChangesTransaction(t *testing.T) {
	if os.Getenv("GOEF_SAVE_CHANGES") != "1" {
		t.Skip("enable SaveChanges scenarios using the isolated lab runner")
	}
	provider, profile := os.Getenv("GOEF_DRIVERLAB_PROVIDER"), os.Getenv("GOEF_DRIVERLAB_PROFILE")
	engine, ok := ownedTxEngines[provider]
	if !ok || profile == "" {
		t.Fatal("SaveChanges scenarios require a provider and profile")
	}
	if provider == "sqlite" {
		versions := map[string]string{"sqlite-3.51": "3.51.3", "sqlite-3.54": "3.54.1"}
		if versions[profile] != sqliteVersion {
			t.Fatal("SQLite fixture runtime does not match the selected profile")
		}
	}

	for _, tc := range []struct {
		id  string
		run func(*saveChangesFixture, int64)
	}{
		{"TX-OWN-001-S03", (*saveChangesFixture).ownedGraph},
		{"TX-SAVEPOINT-001-S01", (*saveChangesFixture).externalRecovery},
		{"TX-SAVEPOINT-001-S02", (*saveChangesFixture).externalUnsupported},
	} {
		var version string
		passed := t.Run(tc.id, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			s := saveChangesFixture{savepointTxFixture{
				ownedTxFixture: ownedTxFixture{
					scenario: scenario{fixture: providers[provider](t), t: t, provider: provider, ctx: ctx},
					engine:   engine,
				},
				dialect: savepointDialects[provider],
			}}
			session := s.setupPool()
			tc.run(&s, session)
			if s.db.Stats().InUse != 0 {
				t.Fatal("SaveChanges fixture did not return every connection")
			}
			version = s.facts[engine.version]
			if version == "" {
				t.Fatal("SaveChanges fixture did not observe the engine version")
			}
		})
		if !passed {
			continue
		}
		result, err := json.Marshal(map[string]string{
			"provider": provider, "profile": profile, "engine_version": version,
			"scenario": tc.id, "status": "PASS",
		})
		if err != nil {
			t.Fatal("could not encode SaveChanges evidence")
		}
		fmt.Printf("GOEF_SAVE_CHANGES_RESULT=%s\n", result)
	}
}

type saveChangesFixture struct {
	savepointTxFixture
}

func (s *saveChangesFixture) ownedGraph(session int64) {
	s.t.Helper()
	s.createGraph()
	pending := pendingSaveGraph{
		id: -17, revision: 0, label: "parent",
		children: [2]pendingSaveChild{{"first", -17}, {"first", -17}},
	}
	initial := pending

	failed := s.beginSaveTransaction()
	defer s.closeSaveTransaction(failed)
	s.requireWrite(failed.Exec(s.ctx, s.insertOnOriginalSession(), 1, "A", session))
	firstWritten := false
	var secondError error
	saved, err := failed.SaveChanges(s.ctx, s.saveOptions(true), func(writer *transaction.SaveWriter) error {
		if err := s.verifySaveSession(writer, session); err != nil {
			return err
		}
		return s.writeGraph(writer, &pending, &firstWritten, &secondError)
	})
	if err == nil || saved != nil || !firstWritten || secondError == nil {
		s.t.Fatal("expected a failed save after the generated parent and first child were inserted")
	}
	if pending != initial || failed.State() != transaction.StateActive {
		s.t.Fatal("the failed save did not preserve the pending graph and its active transaction")
	}
	if err := failed.Commit(); err != nil || failed.State() != transaction.StateCommitted {
		s.t.Fatal("the recovered owned transaction could not commit")
	}
	s.observeSaveRows(session, func(observer *sql.Conn) {
		var code string
		if err := observer.QueryRowContext(s.ctx, "SELECT code FROM "+ownedTxTable+" WHERE id = "+s.bind(1), 1).
			Scan(&code); err != nil || code != "A" {
			s.t.Fatal("owned recovery did not preserve the earlier write")
		}
		for _, table := range []string{saveParentTable, saveChildTable} {
			var count int
			if err := observer.QueryRowContext(s.ctx, "SELECT COUNT(*) FROM "+table).Scan(&count); err != nil || count != 0 {
				s.t.Fatal("independent observer found a row from the failed graph")
			}
		}
	})

	pending.children[1].code = "second"
	corrected := pending
	retry := s.beginSaveTransaction()
	defer s.closeSaveTransaction(retry)
	firstWritten, secondError = false, nil
	saved, err = retry.SaveChanges(s.ctx, s.saveOptions(true), func(writer *transaction.SaveWriter) error {
		if err := s.verifySaveSession(writer, session); err != nil {
			return err
		}
		return s.writeGraph(writer, &pending, &firstWritten, &secondError)
	})
	if err != nil || saved == nil || !firstWritten || secondError != nil || pending != corrected {
		s.t.Fatal("corrected save failed or published provisional values before completion")
	}
	if err := retry.Commit(); err != nil || pending != corrected {
		s.t.Fatal("corrected graph did not commit with acceptance still deferred")
	}
	if err := saved.Accept(); err != nil || pending.id <= 0 || pending.revision != 1 ||
		pending.children[0].parentID != pending.id || pending.children[1].parentID != pending.id ||
		pending.label != corrected.label || pending.children[0].code != "first" || pending.children[1].code != "second" {
		s.t.Fatal("acceptance did not publish the generated key, version and child references")
	}
	s.observeSaveRows(session, func(observer *sql.Conn) {
		var id, revision int64
		var label string
		if err := observer.QueryRowContext(s.ctx, "SELECT id, revision, label FROM "+saveParentTable).
			Scan(&id, &revision, &label); err != nil || id != pending.id || revision != pending.revision || label != pending.label {
			s.t.Fatal("independent observer did not find the accepted parent")
		}
		rows, err := observer.QueryContext(s.ctx, "SELECT code, parent_id FROM "+saveChildTable+" ORDER BY code")
		if err != nil {
			s.t.Fatal("independent observer could not read the children")
		}
		defer rows.Close()
		for _, child := range pending.children {
			var code string
			var parentID int64
			if !rows.Next() || rows.Scan(&code, &parentID) != nil || code != child.code || parentID != child.parentID {
				s.t.Fatal("independent observer did not find the expected child and generated parent key")
			}
		}
		if rows.Next() || rows.Err() != nil {
			s.t.Fatal("independent observer found extra children or could not finish reading")
		}
	})
}

type pendingSaveGraph struct {
	id, revision int64
	label        string
	children     [2]pendingSaveChild
}

type pendingSaveChild struct {
	code     string
	parentID int64
}

func (s *saveChangesFixture) createGraph() {
	s.t.Helper()
	identity := map[string]string{
		"postgres":  "BIGINT GENERATED BY DEFAULT AS IDENTITY",
		"sqlite":    "INTEGER",
		"sqlserver": "BIGINT IDENTITY(1,1)",
		"mysql":     "BIGINT AUTO_INCREMENT",
	}[s.provider]
	suffix := ""
	if s.provider == "mysql" {
		suffix = " ENGINE=InnoDB"
	}

	for _, table := range []struct{ name, definition string }{
		{saveParentTable, "id " + identity + " PRIMARY KEY, label VARCHAR(8) NOT NULL, revision BIGINT NOT NULL DEFAULT 1"},
		{saveChildTable, "code VARCHAR(8) NOT NULL UNIQUE, parent_id BIGINT NOT NULL, FOREIGN KEY (parent_id) REFERENCES " + saveParentTable + "(id)"},
	} {
		if _, err := s.db.ExecContext(s.ctx, "CREATE TABLE "+table.name+" ("+table.definition+")"+suffix); err != nil {
			s.t.Fatalf("could not create the SaveChanges graph fixture (native code %q)", s.code(err))
		}
		s.t.Cleanup(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if _, err := s.db.ExecContext(ctx, "DROP TABLE "+table.name); err != nil {
				s.t.Error("could not remove a SaveChanges graph fixture table")
			}
		})
	}
}

func (s *saveChangesFixture) beginSaveTransaction() *transaction.Tx {
	s.t.Helper()
	tx, err := transaction.Begin(s.ctx, s.db)
	if err != nil || tx == nil {
		s.t.Fatal("could not begin an owned SaveChanges transaction")
	}
	if s.provider == "sqlserver" {
		// The shared driver fixture initializes XACT_ABORT ON. These recovery
		// scenarios require a constraint error that leaves the transaction committable.
		if _, err := tx.Exec(s.ctx, "SET XACT_ABORT OFF"); err != nil {
			s.closeSaveTransaction(tx)
			s.t.Fatal("could not enable recovery on the owned SQL Server session")
		}
	}
	return tx
}

func (s *saveChangesFixture) closeSaveTransaction(tx *transaction.Tx) {
	s.t.Helper()
	if err := tx.Close(); err != nil {
		s.t.Error("could not close the owned SaveChanges transaction")
	}
}

func (s *saveChangesFixture) saveOptions(enabled bool) transaction.SaveOptions {
	return transaction.SaveOptions{Dialect: s.dialect, Savepoints: enabled}
}

func (s *saveChangesFixture) verifySaveSession(writer *transaction.SaveWriter, session int64) error {
	var actual int64
	query := "SELECT " + s.engine.session
	if s.provider == "sqlserver" {
		query += " WHERE (@@OPTIONS & 16384) = 0"
	}
	if err := writer.Scan(query, nil, &actual); err != nil {
		return err
	}
	if actual != session {
		return errors.New("SaveChanges command did not retain the original physical session")
	}
	return nil
}

func (s *saveChangesFixture) writeGraph(writer *transaction.SaveWriter, graph *pendingSaveGraph, firstWritten *bool, secondError *error) error {
	parentID, revision, err := s.insertSaveParent(writer, graph.label)
	if err != nil {
		return err
	}
	if parentID <= 0 || revision != 1 {
		return errors.New("database did not supply generated parent values")
	}
	if err := transaction.StageValue(writer, &graph.id, parentID); err != nil {
		return err
	}
	if err := transaction.StageValue(writer, &graph.revision, revision); err != nil {
		return err
	}
	for i := range graph.children {
		if err := transaction.StageValue(writer, &graph.children[i].parentID, parentID); err != nil {
			return err
		}
	}
	insert := "INSERT INTO " + saveChildTable + " (code, parent_id) VALUES (" + s.bind(1) + ", " + s.bind(2) + ")"
	if err := requireSaveWrite(writer.Exec(insert, graph.children[0].code, parentID)); err != nil {
		return err
	}
	*firstWritten = true
	_, *secondError = writer.Exec(insert, graph.children[1].code, parentID)
	return *secondError
}

func (s *saveChangesFixture) insertSaveParent(writer *transaction.SaveWriter, label string) (int64, int64, error) {
	var parentID, revision int64
	insert := "INSERT INTO " + saveParentTable + " (label)"
	values := " VALUES (" + s.bind(1) + ")"
	var err error
	switch s.provider {
	case "sqlserver":
		err = writer.Scan(insert+" OUTPUT INSERTED.id, INSERTED.revision"+values, []any{label}, &parentID, &revision)
	case "mysql":
		result, insertErr := writer.Exec(insert+values, label)
		if insertErr != nil {
			return 0, 0, insertErr
		}
		parentID, err = result.LastInsertId()
		if err != nil {
			return 0, 0, err
		}
		err = writer.Scan("SELECT revision FROM "+saveParentTable+" WHERE id = "+s.bind(1), []any{parentID}, &revision)
	default:
		err = writer.Scan(insert+values+" RETURNING id, revision", []any{label}, &parentID, &revision)
	}
	return parentID, revision, err
}

func requireSaveWrite(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	if result == nil {
		return errors.New("SaveChanges command returned no result")
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return errors.New("SaveChanges command did not write exactly one row")
	}
	return nil
}

func (s *saveChangesFixture) observeSaveRows(session int64, check func(*sql.Conn)) {
	s.t.Helper()
	s.db.SetMaxOpenConns(2)
	defer s.db.SetMaxOpenConns(1)
	original := s.lease()
	defer s.closeConn(original)
	if s.sessionID(original) != session {
		s.t.Fatal("SaveChanges did not preserve the original connection")
	}
	observer := s.lease()
	defer func() {
		// Keep only the original session available for subsequent saves.
		s.db.SetMaxIdleConns(0)
		s.closeConn(observer)
		s.db.SetMaxIdleConns(1)
	}()
	if s.provider == "sqlite" {
		var markers int
		if err := observer.QueryRowContext(s.ctx,
			"SELECT COUNT(*) FROM temp.sqlite_master WHERE name = ?1", "goef_owned_tx_session").Scan(&markers); err != nil || markers != 0 {
			s.t.Fatal("SQLite save observer was not an independent physical connection")
		}
	} else if s.sessionID(observer) == session {
		s.t.Fatal("save observer reused the transaction session")
	}
	check(observer)
}

func (s *saveChangesFixture) externalRecovery(session int64) {
	s.t.Helper()
	s.completeExternalSave(session, true)
	s.observeSaveRows(session, func(observer *sql.Conn) {
		s.requireDurableExternalRows(observer, session, "A")
	})
	s.successfulExternalSave(session)
	s.observeSaveRows(session, func(observer *sql.Conn) {
		s.requireDurableExternalRows(observer, session, "A", "B")
	})
}

func (s *saveChangesFixture) externalUnsupported(session int64) {
	s.t.Helper()
	s.completeExternalSave(session, false)
	s.observeCommittedRow(session)
}

func (s *saveChangesFixture) completeExternalSave(session int64, supportsSavepoints bool) {
	s.t.Helper()
	s.withExternalSave(session, supportsSavepoints, func(native *sql.Tx, external *transaction.External) {
		insert := s.insertOnOriginalSession()
		s.requireWrite(native.ExecContext(s.ctx, insert, 1, "A", session))
		if supportsSavepoints {
			s.requireExternalRecovery(external, insert, session)
		} else {
			s.requireExternalPreflightRejection(external, insert, session, false)
			if s.provider == "sqlserver" {
				var abort int
				if err := native.QueryRowContext(s.ctx, "SELECT @@OPTIONS & 16384").Scan(&abort); err != nil || abort != 16384 {
					s.t.Fatal("SQL Server rejection fixture requires XACT_ABORT ON")
				}
				s.requireExternalPreflightRejection(external, insert, session, true)
			}
		}
		var count int
		if err := native.QueryRowContext(s.ctx, "SELECT COUNT(*) FROM "+ownedTxTable).Scan(&count); err != nil || count != 1 {
			s.t.Fatal("owner did not retain only its earlier pending write")
		}
	})
}

func (s *saveChangesFixture) successfulExternalSave(session int64) {
	s.t.Helper()
	value := "pending"
	var saved *transaction.SavedChanges
	s.withExternalSave(session, true, func(native *sql.Tx, external *transaction.External) {
		var err error
		saved, err = external.SaveChanges(s.ctx, s.saveOptions(true), func(writer *transaction.SaveWriter) error {
			if err := s.verifySaveSession(writer, session); err != nil {
				return err
			}
			if err := requireSaveWrite(writer.Exec(s.insertOnOriginalSession(), 2, "B", session)); err != nil {
				return err
			}
			return transaction.StageValue(writer, &value, "B")
		})
		if err != nil || saved == nil || value != "pending" {
			s.t.Fatal("successful external save did not defer acceptance")
		}
		var code string
		if err := native.QueryRowContext(s.ctx, "SELECT code FROM "+ownedTxTable+" WHERE id = "+s.bind(1), 2).
			Scan(&code); err != nil || code != "B" {
			s.t.Fatal("owner could not see the provisional external write")
		}
	})
	if value != "pending" || saved.Accept() != nil || value != "B" {
		s.t.Fatal("external receipt did not accept the staged value after the owner's commit")
	}
}

func (s *saveChangesFixture) requireDurableExternalRows(observer *sql.Conn, session int64, codes ...string) {
	s.t.Helper()
	rows, err := observer.QueryContext(s.ctx, "SELECT id, code, session_id FROM "+ownedTxTable+" ORDER BY id")
	if err != nil {
		s.t.Fatal("could not read durable external rows")
	}
	defer rows.Close()
	for i, expected := range codes {
		var id int
		var code string
		var actualSession int64
		if !rows.Next() || rows.Scan(&id, &code, &actualSession) != nil ||
			id != i+1 || code != expected || actualSession != session {
			s.t.Fatal("independent observer did not find an expected external row")
		}
	}
	if rows.Next() || rows.Err() != nil {
		s.t.Fatal("independent observer found an extra external row or could not finish reading")
	}
}

func (s *saveChangesFixture) withExternalSave(session int64, supportsSavepoints bool, run func(*sql.Tx, *transaction.External)) {
	s.t.Helper()
	owner := s.lease()
	defer s.closeConn(owner)
	if s.sessionID(owner) != session {
		s.t.Fatal("caller-owned SaveChanges received the wrong physical session")
	}
	native, err := owner.BeginTx(s.ctx, nil)
	if err != nil {
		s.t.Fatal("could not begin the caller-owned SaveChanges transaction")
	}
	defer func() {
		if err := native.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			s.t.Error("could not clean up the caller-owned transaction")
		}
	}()
	if s.provider == "sqlserver" && supportsSavepoints {
		if _, err := native.ExecContext(s.ctx, "SET XACT_ABORT OFF"); err != nil {
			s.t.Fatal("could not enable recovery on the caller-owned SQL Server session")
		}
	}
	external, err := transaction.NewExternal(native)
	if err != nil || external == nil {
		s.t.Fatal("could not coordinate the caller-owned SaveChanges transaction")
	}
	defer func() {
		if err := external.Close(); err != nil {
			s.t.Error("could not close the caller-owned SaveChanges coordinator")
		}
	}()

	run(native, external)
	if err := external.Close(); err != nil {
		s.t.Fatal("could not detach the caller-owned SaveChanges coordinator")
	}

	if err := native.Commit(); err != nil {
		s.t.Fatal("the external transaction owner could not commit its pending work")
	}
	if s.sessionID(owner) != session || s.db.Stats().InUse != 1 {
		s.t.Fatal("SaveChanges replaced or returned the caller's connection")
	}
}

func (s *saveChangesFixture) requireExternalPreflightRejection(external *transaction.External, insert string, session int64, supportsSavepoints bool) {
	s.t.Helper()
	invoked := false
	saved, err := external.SaveChanges(s.ctx, s.saveOptions(supportsSavepoints), func(writer *transaction.SaveWriter) error {
		invoked = true
		return requireSaveWrite(writer.Exec(insert, 2, "B", session))
	})
	if !errors.Is(err, goeferr.ErrUnsupportedFeature) || saved != nil || invoked {
		s.t.Fatal("unsafe external save did not reject before callback and DML")
	}
}

func (s *saveChangesFixture) requireExternalRecovery(external *transaction.External, insert string, session int64) {
	s.t.Helper()
	participant, err := external.Enlist()
	if err != nil || participant == nil {
		s.t.Fatal("could not enlist a participant for SaveChanges")
	}
	defer s.detach(participant)
	firstWritten := false
	var secondError error
	value := "pending"
	saved, err := participant.SaveChanges(s.ctx, s.saveOptions(true), func(writer *transaction.SaveWriter) error {
		if err := s.verifySaveSession(writer, session); err != nil {
			return err
		}
		if err := requireSaveWrite(writer.Exec(insert, 2, "B", session)); err != nil {
			return err
		}
		firstWritten = true
		if err := transaction.StageValue(writer, &value, "flushed"); err != nil {
			return err
		}
		_, secondError = writer.Exec(insert, 2, "C", session)
		return secondError
	})
	if err == nil || saved != nil || !firstWritten || secondError == nil || value != "pending" {
		s.t.Fatal("failed external save did not revert its partial write and preserve the pending value")
	}
}
