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

const transactionOptionsRow = int64(1)

func TestTransactionOptions(t *testing.T) {
	if os.Getenv("GOEF_TX_OPTIONS") != "1" {
		t.Skip("enable the transaction options scenario using the isolated lab runner")
	}
	provider, profile := os.Getenv("GOEF_DRIVERLAB_PROVIDER"), os.Getenv("GOEF_DRIVERLAB_PROFILE")
	engine, ok := ownedTxEngines[provider]
	if !ok || profile == "" {
		t.Fatal("transaction options scenario requires a provider and profile")
	}
	if provider == "sqlite" {
		versions := map[string]string{"sqlite-3.51": "3.51.3", "sqlite-3.54": "3.54.1"}
		if versions[profile] != sqliteVersion {
			t.Fatal("SQLite fixture runtime does not match the selected profile")
		}
	}

	var observedVersion string
	passed := t.Run(provider, func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
		defer cancel()
		f := transactionOptionsFixture{
			ownedTxFixture: ownedTxFixture{
				scenario: scenario{fixture: providers[provider](t), t: t, provider: provider, ctx: ctx},
				engine:   engine,
			},
			dialect: transaction.Dialect(provider),
		}
		f.run()
		observedVersion = f.facts[engine.version]
		if observedVersion == "" {
			t.Fatal("transaction options fixture did not observe an engine version")
		}
	})
	if !passed || observedVersion == "" {
		return
	}
	result, err := json.Marshal(map[string]string{
		"provider": provider, "profile": profile, "engine_version": observedVersion,
		"scenario": "TX-ISOLATION-001-S01", "status": "PASS",
	})
	if err != nil {
		t.Fatal("could not encode transaction options evidence")
	}
	fmt.Printf("GOEF_TX_OPTIONS_RESULT=%s\n", result)
}

type transactionOptionsFixture struct {
	ownedTxFixture
	dialect    transaction.Dialect
	session    int64
	diagnostic *sql.Conn
}

func (f *transactionOptionsFixture) run() {
	f.t.Helper()
	f.session = f.setupPool()
	f.db.SetMaxOpenConns(2)
	primary := f.lease()
	defer primary.Close()
	f.observer = f.lease()
	defer f.observer.Close()
	f.checkUnsupportedBeforeAcquire()
	f.closeConn(primary)

	if f.provider == "sqlserver" {
		f.diagnostic = f.openSQLServerLockObserver()
	}
	insert := "INSERT INTO " + ownedTxTable + " (id, code, session_id) VALUES (" +
		f.bind(1) + ", " + f.bind(2) + ", " + f.bind(3) + ")"
	f.requireWrite(f.observer.ExecContext(f.ctx, insert, transactionOptionsRow, "seed", 0))

	levels := []sql.IsolationLevel{sql.LevelReadCommitted, sql.LevelRepeatableRead, sql.LevelSerializable}
	if f.provider == "sqlite" {
		levels = []sql.IsolationLevel{sql.LevelSerializable}
		f.checkSQLiteUnsafeMode()
	}
	for _, level := range levels {
		f.checkIsolation(level)
	}
	if f.provider == "postgres" || f.provider == "mysql" {
		f.checkReadOnly()
	}
	f.closeConn(f.observer)
	if f.db.Stats().InUse != 0 {
		f.t.Fatal("transaction options fixture left a connection checked out")
	}
}

func (f *transactionOptionsFixture) checkUnsupportedBeforeAcquire() {
	f.t.Helper()
	before := f.db.Stats()
	if before.InUse != 2 {
		f.t.Fatal("fixture needs both pool slots held for preflight test")
	}
	invalid := []sql.TxOptions{{Isolation: sql.LevelLinearizable}}
	switch f.provider {
	case "sqlite":
		invalid = append(invalid, sql.TxOptions{ReadOnly: true},
			sql.TxOptions{Isolation: sql.LevelReadCommitted}, sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	case "sqlserver":
		invalid = append(invalid, sql.TxOptions{ReadOnly: true})
	case "postgres":
		invalid = append(invalid, sql.TxOptions{Isolation: sql.LevelReadUncommitted},
			sql.TxOptions{Isolation: sql.LevelSnapshot})
	}
	for _, options := range invalid {
		ctx, cancel := context.WithTimeout(f.ctx, time.Second)
		tx, err := transaction.BeginWithOptions(ctx, f.db, f.dialect, options)
		cancel()
		if tx != nil {
			_ = tx.Close()
		}
		if tx != nil || !errors.Is(err, goeferr.ErrUnsupportedFeature) {
			f.t.Fatalf("unsupported options acquired a lease or lacked typed rejection (isolation %s, read-only %t)", options.Isolation, options.ReadOnly)
		}
	}
	if got := f.db.Stats(); got.WaitCount != before.WaitCount || got.InUse != before.InUse {
		f.t.Fatal("unsupported options touched the pool queue")
	}
}

func (f *transactionOptionsFixture) checkSQLiteUnsafeMode() {
	f.t.Helper()
	conn := f.lease()
	if f.sessionID(conn) != f.session {
		f.t.Fatal("fixture did not get original SQLite worker lease")
	}
	if _, err := conn.ExecContext(f.ctx, "PRAGMA read_uncommitted = 1"); err != nil {
		f.t.Fatal("could not enable SQLite read_uncommitted negative control")
	}
	f.closeConn(conn)
	// The observer still holds the second slot, so GoEF must inspect the exact
	// SQLite connection whose PRAGMA was changed.
	tx, err := transaction.BeginWithOptions(f.ctx, f.db, f.dialect, sql.TxOptions{Isolation: sql.LevelSerializable})
	if tx != nil {
		_ = tx.Close()
	}
	if tx != nil || !errors.Is(err, goeferr.ErrUnsupportedFeature) {
		f.t.Fatal("SQLite accepted Serializable with read_uncommitted enabled")
	}
	conn = f.lease()
	defer f.closeConn(conn)
	var enabled int
	if err := conn.QueryRowContext(f.ctx, "PRAGMA read_uncommitted").Scan(&enabled); err != nil ||
		enabled != 1 || f.sessionID(conn) != f.session {
		f.t.Fatal("the rejected SQLite isolation request changed the connection or its settings")
	}
	if _, err := conn.ExecContext(f.ctx, "PRAGMA read_uncommitted = 0"); err != nil {
		f.t.Fatal("could not restore SQLite read_uncommitted")
	}
}

func (f *transactionOptionsFixture) beginWith(options sql.TxOptions) *transaction.Tx {
	f.t.Helper()
	tx, err := transaction.BeginWithOptions(f.ctx, f.db, f.dialect, options)
	if err != nil || tx == nil {
		f.t.Fatalf("could not begin GoEF transaction (isolation %s, read-only %t)", options.Isolation, options.ReadOnly)
	}
	if f.db.Stats().InUse != 2 {
		_ = tx.Close()
		f.t.Fatal("transaction did not occupy the one available worker slot")
	}
	return tx
}

func (f *transactionOptionsFixture) workerSession(tx *transaction.Tx) int64 {
	f.t.Helper()
	var session int64
	if err := tx.Scan(f.ctx, "SELECT "+f.engine.session, nil, &session); err != nil || session <= 0 {
		f.t.Fatal("could not read the active transaction's session")
	}
	return session
}

func (f *transactionOptionsFixture) checkIsolation(level sql.IsolationLevel) {
	f.t.Helper()
	f.requireWrite(f.observer.ExecContext(f.ctx, f.updateSQL(), "before", transactionOptionsRow))
	tx := f.beginWith(sql.TxOptions{Isolation: level})
	defer tx.Close()
	session := f.workerSession(tx)
	if session != f.session {
		f.t.Fatalf("isolation %s began on a different worker session", level)
	}
	var physical string
	if f.provider == "sqlserver" {
		physical = f.sqlServerPhysicalID(session)
	}
	f.checkSettings(tx, level, false)
	if f.readCode(tx) != "before" {
		f.t.Fatal("worker did not observe the initial committed row")
	}

	if f.provider == "mysql" && level == sql.LevelSerializable ||
		f.provider == "sqlserver" && level != sql.LevelReadCommitted {
		f.checkBlockingWrite(tx, session)
	} else {
		f.requireWrite(f.observer.ExecContext(f.ctx, f.updateSQL(), "after", transactionOptionsRow))
		want := "before"
		if level == sql.LevelReadCommitted {
			want = "after"
		}
		if got := f.readCode(tx); got != want {
			f.t.Fatalf("isolation %s observed the wrong second read", level)
		}
		if err := tx.Rollback(); err != nil {
			f.t.Fatalf("could not roll back isolation %s", level)
		}
	}
	f.requireObserverCode("after")
	f.checkDefaultAfter(level, session, physical)
}

func (f *transactionOptionsFixture) checkSettings(tx *transaction.Tx, level sql.IsolationLevel, readOnly bool) {
	f.t.Helper()
	switch f.provider {
	case "postgres":
		names := map[sql.IsolationLevel]string{
			sql.LevelReadCommitted: "read committed", sql.LevelRepeatableRead: "repeatable read", sql.LevelSerializable: "serializable",
		}
		var actualLevel string
		var actualReadOnly bool
		if err := tx.Scan(f.ctx,
			"SELECT current_setting('transaction_isolation'), current_setting('transaction_read_only')::boolean",
			nil, &actualLevel, &actualReadOnly); err != nil || actualLevel != names[level] || actualReadOnly != readOnly {
			f.t.Fatalf("PostgreSQL reported unexpected settings for isolation %s", level)
		}
	case "sqlite":
		var readUncommitted int
		if err := tx.Scan(f.ctx, "PRAGMA read_uncommitted", nil, &readUncommitted); err != nil || readUncommitted != 0 {
			f.t.Fatal("SQLite worker does not have read_uncommitted disabled")
		}
	case "sqlserver":
		modes := map[sql.IsolationLevel]int{sql.LevelReadCommitted: 2, sql.LevelRepeatableRead: 3, sql.LevelSerializable: 4}
		var mode int
		if err := tx.Scan(f.ctx, "SELECT transaction_isolation_level FROM sys.dm_exec_sessions WHERE session_id = @@SPID",
			nil, &mode); err != nil || mode != modes[level] {
			f.t.Fatalf("SQL Server reported an unexpected session setting for isolation %s", level)
		}
	}
}

func (f *transactionOptionsFixture) checkBlockingWrite(tx *transaction.Tx, holderID int64) {
	f.t.Helper()
	waiterID := f.sessionID(f.observer)
	if waiterID == holderID {
		f.t.Fatal("lock fixture requires separate worker and writer sessions")
	}

	var holderThread, waiterThread uint64
	if f.provider == "mysql" {
		if err := tx.Scan(f.ctx, "SELECT PS_CURRENT_THREAD_ID()", nil, &holderThread); err != nil || holderThread == 0 {
			f.t.Fatal("could not identify the MySQL lock holder thread")
		}
		waiterThread = f.mysqlLockIdentity(f.observer.QueryRowContext).thread
		if holderThread == waiterThread {
			f.t.Fatal("MySQL lock fixture requires disparate worker threads")
		}
	}

	request, cancel := context.WithTimeout(f.ctx, 6*time.Second)
	defer cancel()
	type updateResult struct {
		result sql.Result
		err    error
	}
	done := make(chan updateResult, 1)
	go func() {
		result, err := f.observer.ExecContext(request, f.updateSQL(), "after", transactionOptionsRow)
		done <- updateResult{result: result, err: err}
	}()
	collected := false
	defer func() {
		if !collected {
			cancel()
			<-done
		}
	}()

	probe, stop := context.WithTimeout(request, 3*time.Second)
	defer stop()
	if f.provider == "sqlserver" {
		if err := observeSQLServerLockWait(probe, f.diagnostic, int(holderID), int(waiterID)); err != nil {
			f.t.Fatal("SQL Server writer did not exhibit the reader's lock")
		}
	} else {
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		const query = "SELECT EXISTS (SELECT 1 FROM performance_schema.data_lock_waits WHERE ENGINE = 'INNODB' AND REQUESTING_THREAD_ID = ? AND BLOCKING_THREAD_ID = ?)"
		for {
			var waiting bool
			if err := tx.Scan(probe, query, []any{waiterThread, holderThread}, &waiting); err != nil {
				f.t.Fatal("could not observe the MySQL Serializable reader lock")
			}
			if waiting {
				break
			}
			select {
			case <-probe.Done():
				f.t.Fatal("MySQL writer did not exhibit the Serializable reader lock")
			case <-ticker.C:
			}
		}
	}
	if err := tx.Rollback(); err != nil {
		f.t.Fatal("could not release the isolation reader transaction")
	}
	write := <-done
	collected = true
	f.requireWrite(write.result, write.err)
}

// Numeric session IDs may be recycled by SQL Server immediately after a
// disconnect. The existing isolated diagnostic login can see the connection
// GUID, which distinguishes the physical connection from the reused SPID.
func (f *transactionOptionsFixture) sqlServerPhysicalID(session int64) string {
	f.t.Helper()
	const query = "SELECT CONVERT(varchar(36), connection_id) FROM sys.dm_exec_connections WHERE session_id = @p1 AND parent_connection_id IS NULL"
	var id string
	if err := f.diagnostic.QueryRowContext(f.ctx, query, session).Scan(&id); err != nil || len(id) != 36 {
		f.t.Fatal("could not identify the SQL Server physical connection")
	}
	return id
}

func (f *transactionOptionsFixture) checkDefaultAfter(level sql.IsolationLevel, previous int64, physical string) {
	f.t.Helper()
	if f.db.Stats().InUse != 1 {
		f.t.Fatal("completed transaction did not release the worker slot")
	}
	tx, err := transaction.Begin(f.ctx, f.db)
	if err != nil {
		f.t.Fatal("could not start the default transaction after an explicit one")
	}
	defer tx.Close()
	current := f.workerSession(tx)
	if f.provider == "sqlserver" {
		if f.sqlServerPhysicalID(current) == physical {
			f.t.Fatal("explicit SQL Server isolation returned its physical connection to the pool")
		}
		f.session = current
	}
	if f.provider != "sqlserver" && current != previous {
		f.t.Fatalf("isolation %s failed to return the same worker session", level)
	}
	f.checkSettings(tx, sql.LevelReadCommitted, false)
	if f.provider == "mysql" {
		var defaultLevel string
		if err := tx.Scan(f.ctx, "SELECT @@session.transaction_isolation", nil, &defaultLevel); err != nil ||
			defaultLevel != "READ-COMMITTED" {
			f.t.Fatal("MySQL worker session default isolation changed")
		}
		// A next-transaction setting could leak while the session variable stays
		// unchanged. Read twice across an observer commit to prove default RC.
		if f.readCode(tx) != "after" {
			f.t.Fatal("MySQL default transaction did not see the previous commit")
		}
		f.requireWrite(f.observer.ExecContext(f.ctx, f.updateSQL(), "default", transactionOptionsRow))
		if f.readCode(tx) != "default" {
			f.t.Fatal("MySQL default transaction inherited an explicit isolation override")
		}
	}
	if err := tx.Rollback(); err != nil {
		f.t.Fatal("could not roll back the default transaction")
	}
}

func (f *transactionOptionsFixture) checkReadOnly() {
	f.t.Helper()
	f.requireWrite(f.observer.ExecContext(f.ctx, f.updateSQL(), "readonly", transactionOptionsRow))
	tx := f.beginWith(sql.TxOptions{Isolation: sql.LevelReadCommitted, ReadOnly: true})
	defer tx.Close()
	if f.workerSession(tx) != f.session {
		f.t.Fatal("read-only transaction did not use the worker connection")
	}
	f.checkSettings(tx, sql.LevelReadCommitted, true)
	if f.readCode(tx) != "readonly" {
		f.t.Fatal("read-only transaction could not read the committed permanent row")
	}
	result, err := tx.Exec(f.ctx, f.updateSQL(), "invalid", transactionOptionsRow)
	if result != nil || !errors.Is(err, goeferr.ErrUnknownDriver) || f.ctx.Err() != nil {
		f.t.Fatal("read-only transaction did not reject a write to a permanent table")
	}
	if err := tx.Rollback(); err != nil {
		f.t.Fatal("could not roll back the read-only transaction")
	}
	f.requireObserverCode("readonly")
	// The same statement now succeeds in a default, otherwise identical
	// transaction after the worker connection has returned to the pool.
	tx, err = transaction.Begin(f.ctx, f.db)
	if err != nil {
		f.t.Fatal("could not start a transaction after read-only completion")
	}
	defer tx.Close()
	if f.workerSession(tx) != f.session {
		f.t.Fatal("read-only completion did not return the same worker connection")
	}
	f.requireWrite(tx.Exec(f.ctx, f.updateSQL(), "writeok", transactionOptionsRow))
	if err := tx.Rollback(); err != nil {
		f.t.Fatal("could not roll back the confirming writable transaction")
	}
	f.requireObserverCode("readonly")
}

func (f *transactionOptionsFixture) updateSQL() string {
	return "UPDATE " + ownedTxTable + " SET code = " + f.bind(1) + " WHERE id = " + f.bind(2)
}

func (f *transactionOptionsFixture) readCode(tx *transaction.Tx) string {
	f.t.Helper()
	var code string
	query := "SELECT code FROM " + ownedTxTable + " WHERE id = " + f.bind(1)
	if err := tx.Scan(f.ctx, query, []any{transactionOptionsRow}, &code); err != nil {
		f.t.Fatal("could not read the fixture row inside the transaction")
	}
	return code
}

func (f *transactionOptionsFixture) requireObserverCode(want string) {
	f.t.Helper()
	var code string
	query := "SELECT code FROM " + ownedTxTable + " WHERE id = " + f.bind(1)
	if err := f.observer.QueryRowContext(f.ctx, query, transactionOptionsRow).Scan(&code); err != nil || code != want {
		f.t.Fatal("the independent observer did not read the expected committed row")
	}
}
