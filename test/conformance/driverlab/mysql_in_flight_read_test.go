package driverlab

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/igorpooh1978/GoEF/provider"
)

var mysqlInFlightRead = provider.Command{
	SQL:     "SELECT id FROM goef_driver_parent WHERE id = 1 FOR SHARE",
	Columns: []provider.Column{{Kind: provider.KindInteger}},
}

type mysqlPendingRead struct {
	rows  provider.Rows
	value []provider.Value
	err   error
	phase string
}

func TestDriverMySQLInFlightRead(t *testing.T) {
	if os.Getenv("GOEF_MYSQL_IN_FLIGHT_READ") != "1" {
		t.Skip("MySQL in-flight read fixture is opt-in")
	}
	profile := os.Getenv("GOEF_DRIVERLAB_PROFILE")
	if os.Getenv("GOEF_DRIVERLAB_PROVIDER") != "mysql" || (profile != "mysql-8.0" && profile != "mysql-8.4") {
		t.Fatal("MySQL in-flight read fixture requires a pinned MySQL profile")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 40*time.Second)
	t.Cleanup(cancel)
	s := scenario{fixture: openMySQL(t), t: t, provider: "mysql", ctx: ctx}
	s.writer = s.lease()
	s.verifyMarker(ctx, s.writer.QueryContext)
	s.createTable("CREATE TABLE goef_driver_parent ("+
		"id INTEGER NOT NULL PRIMARY KEY, code VARCHAR(40) NOT NULL) ENGINE=InnoDB",
		"DROP TABLE goef_driver_parent")
	s.seedParent()

	// The holder uses a different pool so zero in-use connections is an
	// observable invariant for the single-slot SPI reader pool.
	readerPool := openMySQL(t).db
	readerPool.SetMaxOpenConns(1)
	readerPool.SetMaxIdleConns(1)
	backend := spiReadBackend{id: "mysql", db: readerPool}
	holder := s.mysqlLockIdentity(s.writer.QueryRowContext)
	s.mysqlReadRound(backend, holder, false)
	waiter, phase := s.mysqlReadRound(backend, holder, true)

	ctx, cancelRecovery := context.WithTimeout(ctx, 5*time.Second)
	defer cancelRecovery()
	lease, err := backend.Acquire(ctx)
	if err != nil {
		t.Fatal("could not acquire a fresh SPI reader lease")
	}
	reader := &spiScenarioReader{lease: lease}
	defer s.finishMySQLRead(reader, provider.Discard)
	freshConn := lease.(*spiReadLease).conn
	fresh := s.mysqlLockIdentity(freshConn.QueryRowContext)
	if fresh.connection == waiter.connection || fresh.thread == waiter.thread ||
		fresh.connection == holder.connection || fresh.thread == holder.thread {
		t.Fatal("MySQL recovery did not use a fresh physical session")
	}
	s.verifyMarker(ctx, freshConn.QueryContext)
	s.checkMySQLLockRow(freshConn.QueryRowContext, 1, "original")
	reader.rows, err = lease.Query(ctx, spiTwoRows, nil)
	if err != nil || reader.rows == nil {
		t.Fatal("fresh SPI reader query failed")
	}
	s.checkSPIInteger(reader.rows, 1)
	s.checkSPIInteger(reader.rows, 2)
	if row, err := reader.rows.Next(); row != nil || err != io.EOF {
		t.Fatal("fresh SPI reader did not reach clean EOF")
	}
	s.finishMySQLRead(reader, provider.Reuse)
	if stats := readerPool.Stats(); stats.InUse != 0 || stats.OpenConnections != 1 || stats.Idle != 1 {
		t.Fatal("fresh MySQL reader connection did not return to the pool")
	}

	record, err := json.Marshal(map[string]string{
		"profile":           profile,
		"server_version":    s.facts["server_version"],
		"status":            "PASS",
		"wait_observed":     "exact_innodb_threads",
		"release_control":   "row_then_eof",
		"cancel_cause":      "context.Canceled",
		"cancel_phase":      phase,
		"dispatch_count":    "1_each",
		"pool_after_cancel": "0_open_0_in_use",
		"recovery":          "fresh_session_row_unchanged",
	})
	if err != nil {
		t.Fatal("could not encode MySQL read evidence")
	}
	// Keep this detached from testing.Log's file-and-line prefix.
	fmt.Printf("GOEF_MYSQL_IN_FLIGHT_READ_RESULT=%s\n", record)
}

func (s *scenario) mysqlReadRound(backend spiReadBackend, holder mysqlLockSession, cancelReader bool) (mysqlLockSession, string) {
	s.t.Helper()

	holderTx, err := s.writer.BeginTx(s.ctx, nil)
	if err != nil {
		s.t.Fatal("could not begin the MySQL lock holder")
	}
	defer s.rollbackMySQLLockTx(holderTx)
	s.updateMySQLLockRow(holderTx, 1, "held")

	lease, err := backend.Acquire(s.ctx)
	if err != nil {
		s.t.Fatal("could not acquire the SPI reader lease")
	}
	reader := &spiScenarioReader{lease: lease}
	defer s.finishMySQLRead(reader, provider.Discard)
	nativeLease := lease.(*spiReadLease)
	waiter := s.mysqlLockIdentity(nativeLease.conn.QueryRowContext)
	if waiter.connection == holder.connection || waiter.thread == holder.thread {
		s.t.Fatal("MySQL lock actors share a physical session")
	}
	if stats := backend.db.Stats(); stats.InUse != 1 || stats.OpenConnections != 1 {
		s.t.Fatal("MySQL SPI reader did not occupy its single reader pool slot")
	}
	// Locking reads require an explicit transaction under this autocommit
	// profile. A canceled reader's transaction is retired with its connection.
	if _, err := nativeLease.conn.ExecContext(s.ctx, "START TRANSACTION"); err != nil {
		s.t.Fatal("could not begin the MySQL locking read")
	}

	request, cancel := context.WithTimeout(s.ctx, 8*time.Second)
	completed := make(chan mysqlPendingRead, 1)
	go func() {
		rows, err := lease.Query(request, mysqlInFlightRead, nil)
		if err != nil || rows == nil {
			completed <- mysqlPendingRead{rows: rows, err: err, phase: "query"}
			return
		}
		value, err := rows.Next()
		completed <- mysqlPendingRead{rows: rows, value: value, err: err, phase: "first_next"}
	}()
	joined := false
	receive := func(result mysqlPendingRead) {
		joined = true
		reader.rows = result.rows
	}
	defer func() {
		cancel()
		if !joined {
			receive(<-completed)
		}
	}()

	poll := time.NewTicker(10 * time.Millisecond)
	defer poll.Stop()
	for {
		var waiting bool
		if err := holderTx.QueryRowContext(request, `
			SELECT EXISTS (
				SELECT 1 FROM performance_schema.data_lock_waits
				WHERE ENGINE = 'INNODB' AND REQUESTING_THREAD_ID = ? AND BLOCKING_THREAD_ID = ?
			)`, waiter.thread, holder.thread).Scan(&waiting); err != nil {
			s.t.Fatal("could not observe the MySQL locking read")
		}
		select {
		case result := <-completed:
			receive(result)
			s.t.Fatal("SPI read finished before the exact MySQL lock wait was observed")
		default:
		}
		if waiting && request.Err() == nil {
			break
		}
		select {
		case result := <-completed:
			receive(result)
			s.t.Fatal("SPI read finished before the exact MySQL lock wait was observed")
		case <-request.Done():
			s.t.Fatal("timed out observing the MySQL SPI read")
		case <-poll.C:
		}
	}

	if cancelReader {
		cancel()
	} else if err := holderTx.Rollback(); err != nil {
		s.t.Fatal("could not release the holder for the live-context control")
	}
	result := <-completed
	receive(result)
	if nativeLease.queryDispatches != 1 {
		s.t.Fatal("MySQL locking read did not dispatch once through the SPI")
	}
	if cancelReader {
		if result.value != nil || !errors.Is(request.Err(), context.Canceled) ||
			!errors.Is(result.err, context.Canceled) || errors.Is(result.err, context.DeadlineExceeded) {
			s.t.Fatal("in-flight SPI read did not return caller cancellation before a row")
		}
		s.finishMySQLRead(reader, provider.Discard)
		if stats := backend.db.Stats(); stats.InUse != 0 || stats.OpenConnections != 0 {
			s.t.Fatal("canceled MySQL SPI read left a connection in its pool")
		}
		if err := holderTx.Rollback(); err != nil {
			s.t.Fatal("could not roll back the MySQL holder after reader discard")
		}
		return waiter, result.phase
	}
	if request.Err() != nil || result.err != nil || len(result.value) != 1 {
		s.t.Fatal("live locking read did not return the expected row")
	}
	if value, ok := result.value[0].(provider.Integer); !ok || value != 1 {
		s.t.Fatal("live locking read did not return integer 1")
	}
	if row, err := reader.rows.Next(); row != nil || err != io.EOF {
		s.t.Fatal("live locking read did not reach clean EOF")
	}
	if _, err := nativeLease.conn.ExecContext(request, "ROLLBACK"); err != nil {
		s.t.Fatal("could not end the successful MySQL locking read")
	}
	s.finishMySQLRead(reader, provider.Reuse)
	if stats := backend.db.Stats(); stats.InUse != 0 || stats.Idle != 1 {
		s.t.Fatal("live MySQL reader did not return its connection to the pool")
	}
	return waiter, result.phase
}

func (s *scenario) finishMySQLRead(reader *spiScenarioReader, disposition provider.Disposition) {
	s.t.Helper()
	if err := reader.finish(disposition); err != nil {
		s.t.Error("could not close and release the MySQL SPI reader")
	}
}
