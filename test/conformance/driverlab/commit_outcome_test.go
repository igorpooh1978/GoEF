package driverlab

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/igorpooh1978/GoEF/goeferr"
	"github.com/igorpooh1978/GoEF/transaction"
)

const (
	outcomeMarkerTable    = "goef_commit_marker"
	outcomeAggregateTable = "goef_commit_aggregate"
)

func TestCommitOutcome(t *testing.T) {
	if os.Getenv("GOEF_COMMIT_OUTCOME") != "1" {
		t.Skip("enable the commit outcome scenarios using the isolated lab runner")
	}
	provider, profile := os.Getenv("GOEF_DRIVERLAB_PROVIDER"), os.Getenv("GOEF_DRIVERLAB_PROFILE")
	engine, ok := ownedTxEngines[provider]
	if !ok || profile == "" {
		t.Fatal("commit outcome scenarios require a provider and profile")
	}
	if provider == "sqlite" {
		versions := map[string]string{"sqlite-3.51": "3.51.3", "sqlite-3.54": "3.54.1"}
		if versions[profile] != sqliteVersion {
			t.Fatal("SQLite fixture runtime does not match the selected profile")
		}
	}

	for _, tc := range []struct {
		id            string
		commitDurably bool
	}{
		{"TX-OUTCOME-001-S03", true},
		{"TX-OUTCOME-001-S04", false},
	} {
		var version string
		passed := t.Run(tc.id, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			s := commitOutcomeFixture{scenario{fixture: providers[provider](t), t: t, provider: provider, ctx: ctx}}
			s.createTables()
			s.exercise(tc.commitDurably)
			version = s.facts[engine.version]
			if version == "" {
				t.Fatal("commit outcome fixture did not observe the engine version")
			}
		})
		if passed {
			result, err := json.Marshal(map[string]string{
				"provider": provider, "profile": profile, "engine_version": version,
				"scenario": tc.id, "status": "PASS",
			})
			if err != nil {
				t.Fatal("could not encode commit outcome evidence")
			}
			fmt.Printf("GOEF_COMMIT_OUTCOME_RESULT=%s\n", result)
		}
	}
}

type commitOutcomeFixture struct {
	scenario
}

func (s commitOutcomeFixture) createTables() {
	s.t.Helper()
	s.writer = s.lease()
	if s.provider != "sqlite" {
		s.verifyMarker(s.ctx, s.writer.QueryContext)
	}
	suffix := ""
	if s.provider == "mysql" {
		suffix = " ENGINE=InnoDB"
	}
	s.createTable("CREATE TABLE "+outcomeMarkerTable+" (id INTEGER NOT NULL PRIMARY KEY)"+suffix,
		"DROP TABLE "+outcomeMarkerTable)
	s.createTable("CREATE TABLE "+outcomeAggregateTable+" (id INTEGER NOT NULL PRIMARY KEY, amount INTEGER NOT NULL)"+suffix,
		"DROP TABLE "+outcomeAggregateTable)
}

func (s commitOutcomeFixture) exercise(commitDurably bool) {
	s.t.Helper()
	if s.connector == nil {
		s.t.Fatal("commit outcome fixture requires the native driver connector")
	}
	fault := &commitFaultConnector{Connector: s.connector, commitDurably: commitDurably}
	worker := sql.OpenDB(fault)
	defer s.closePool(worker)
	worker.SetMaxOpenConns(1)
	worker.SetMaxIdleConns(1)
	if s.provider != "sqlite" {
		s.verifyMarker(s.ctx, worker.QueryContext)
	}

	tx, err := transaction.Begin(s.ctx, worker)
	if err != nil {
		s.t.Fatal("could not begin the transaction on the fault-injected worker")
	}
	defer func() {
		if err := tx.Close(); err != nil {
			s.t.Error("could not close the transaction on the fault-injected worker")
		}
	}()

	if s.provider == "sqlserver" {
		if _, err := tx.Exec(s.ctx, "SET XACT_ABORT OFF"); err != nil {
			s.t.Fatal("could not enable SQL Server savepoint recovery for the fixture")
		}
	}

	const id, amount = 7, 31
	callbacks, acceptedAmount := 0, -1
	receipt, err := tx.SaveChanges(s.ctx, transaction.SaveOptions{
		Dialect: savepointDialects[s.provider], Savepoints: true,
	}, func(w *transaction.SaveWriter) error {
		callbacks++
		if _, err := w.Exec("INSERT INTO "+outcomeMarkerTable+" (id) VALUES ("+s.bind(1)+")", id); err != nil {
			return err
		}
		if _, err := w.Exec("INSERT INTO "+outcomeAggregateTable+" (id, amount) VALUES ("+
			s.bind(1)+", "+s.bind(2)+")", id, amount); err != nil {
			return err
		}
		return transaction.StageValue(w, &acceptedAmount, amount)
	})
	if err != nil || receipt == nil || acceptedAmount != -1 {
		s.t.Fatal("the batch did not write two effects and stage its value")
	}

	commitErr := tx.Commit()
	var detail *goeferr.ErrorDetail
	if !errors.Is(commitErr, goeferr.ErrCommitOutcomeUnknown) || !errors.As(commitErr, &detail) ||
		detail.ID() != goeferr.CommitOutcomeUnknownID || detail.Phase() != goeferr.PhaseCommit ||
		detail.Outcome() != goeferr.OutcomeUnknown || detail.RetryClass() != goeferr.RetryResolveOutcome ||
		errors.Is(commitErr, io.ErrUnexpectedEOF) || tx.State() != transaction.StateOutcomeUnknown {
		s.t.Fatal("the hidden delivery result did not produce a safe typed unknown outcome")
	}
	if !errors.Is(receipt.Accept(), goeferr.ErrInvalidState) || acceptedAmount != -1 {
		s.t.Fatal("the unknown outcome permitted acceptance of a provisional value")
	}
	if !errors.Is(tx.Commit(), goeferr.ErrInvalidState) || !errors.Is(tx.Rollback(), goeferr.ErrInvalidState) {
		s.t.Fatal("the terminal handle allowed another completion attempt")
	}

	var wantCommits, wantRollbacks int32 = 1, 0
	if !commitDurably {
		wantCommits, wantRollbacks = 0, 1
	}
	if callbacks != 1 || fault.calls.Load() != 1 || fault.commits.Load() != wantCommits ||
		fault.rollbacks.Load() != wantRollbacks {
		s.t.Fatal("the injected phase did not complete exactly once or the batch replayed")
	}
	if fault.opens.Load() != 1 || fault.closes.Load() != 1 || worker.Stats().InUse != 0 || worker.Stats().OpenConnections != 0 {
		s.t.Fatal("the worker did not discard and release the affected physical connection")
	}
	var check int
	if err := worker.QueryRowContext(s.ctx, "SELECT 1").Scan(&check); err != nil || check != 1 ||
		fault.opens.Load() != 2 || worker.Stats().InUse != 0 {
		s.t.Fatal("the worker pool did not recover with a fresh physical connection")
	}
	s.observe(commitDurably, id, amount)
}

func (s commitOutcomeFixture) closePool(db *sql.DB) {
	s.t.Helper()
	if err := db.Close(); err != nil {
		s.t.Error("could not close the commit outcome fixture pool")
	}
}

func (s commitOutcomeFixture) observe(commitDurably bool, id, wantAmount int) {
	s.t.Helper()
	// A separate native pool opens a new physical connection to the same target.
	observer := sql.OpenDB(s.connector)
	defer s.closePool(observer)
	conn, err := observer.Conn(s.ctx)
	if err != nil {
		s.t.Fatal("could not acquire a fresh observer connection")
	}
	defer func() {
		if err := conn.Close(); err != nil {
			s.t.Error("could not release the fresh observer connection")
		}
	}()
	if s.provider != "sqlite" {
		s.verifyMarker(s.ctx, conn.QueryContext)
	}
	wantCount := 0
	if commitDurably {
		wantCount = 1
	}
	for _, table := range []string{outcomeMarkerTable, outcomeAggregateTable} {
		var total int
		if err := conn.QueryRowContext(s.ctx, "SELECT COUNT(*) FROM "+table).Scan(&total); err != nil || total != wantCount {
			s.t.Fatal("the observer found an unexpected number of durable effects")
		}
		if commitDurably {
			var gotID int
			if err := conn.QueryRowContext(s.ctx, "SELECT id FROM "+table).Scan(&gotID); err != nil || gotID != id {
				s.t.Fatal("the durable effect did not match the unique operation marker")
			}
		}
	}
	if commitDurably {
		var amount int
		if err := conn.QueryRowContext(s.ctx, "SELECT amount FROM "+outcomeAggregateTable).Scan(&amount); err != nil ||
			amount != wantAmount {
			s.t.Fatal("the observer did not find the expected durable aggregate")
		}
	}
}

// commitFaultConnector wraps the selected, isolated engine driver. The caller
// receives the same failure after either a real commit or a pre-dispatch abort.
type commitFaultConnector struct {
	driver.Connector
	commitDurably bool
	calls         atomic.Int32
	commits       atomic.Int32
	rollbacks     atomic.Int32
	opens         atomic.Int32
	closes        atomic.Int32
}

func (c *commitFaultConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.Connector.Connect(ctx)
	if err != nil {
		return nil, err
	}
	c.opens.Add(1)
	return &commitFaultConn{Conn: conn, fault: c}, nil
}

type commitFaultConn struct {
	driver.Conn
	fault *commitFaultConnector
}

func (c *commitFaultConn) Close() error {
	err := c.Conn.Close()
	if err == nil {
		c.fault.closes.Add(1)
	}
	return err
}

func (c *commitFaultConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	beginner, ok := c.Conn.(driver.ConnBeginTx)
	if !ok {
		return nil, errors.New("commit outcome fixture requires context-aware transaction begin")
	}
	tx, err := beginner.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &commitFaultTx{Tx: tx, fault: c.fault}, nil
}

func (c *commitFaultConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if execer, ok := c.Conn.(driver.ExecerContext); ok {
		return execer.ExecContext(ctx, query, args)
	}
	return nil, driver.ErrSkip
}

func (c *commitFaultConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if queryer, ok := c.Conn.(driver.QueryerContext); ok {
		return queryer.QueryContext(ctx, query, args)
	}
	return nil, driver.ErrSkip
}

func (c *commitFaultConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	if preparer, ok := c.Conn.(driver.ConnPrepareContext); ok {
		return preparer.PrepareContext(ctx, query)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return c.Conn.Prepare(query)
}

func (c *commitFaultConn) CheckNamedValue(value *driver.NamedValue) error {
	if checker, ok := c.Conn.(driver.NamedValueChecker); ok {
		return checker.CheckNamedValue(value)
	}
	return driver.ErrSkip
}

type commitFaultTx struct {
	driver.Tx
	fault *commitFaultConnector
}

func (t *commitFaultTx) Commit() error {
	t.fault.calls.Add(1)
	if t.fault.commitDurably {
		if err := t.Tx.Commit(); err != nil {
			return err
		}
		t.fault.commits.Add(1)
	} else {
		if err := t.Tx.Rollback(); err != nil {
			return err
		}
		t.fault.rollbacks.Add(1)
	}
	return io.ErrUnexpectedEOF
}
