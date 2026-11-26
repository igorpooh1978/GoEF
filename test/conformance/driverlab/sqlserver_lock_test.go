package driverlab

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"net/url"
	"os"
	"strconv"
	"time"

	mssql "github.com/microsoft/go-mssqldb"
)

const sqlServerLockObserver = "goef_lock_observer"

func (s *scenario) checkSQLServerLockWait() {
	s.t.Helper()
	s.seedParent()
	diagnostic := s.openSQLServerLockObserver()
	var holderID int
	if err := s.writer.QueryRowContext(s.ctx, "SELECT @@SPID").Scan(&holderID); err != nil {
		s.t.Fatal("could not identify the SQL Server lock holder")
	}
	waiterID := s.sqlServerLockWaiterID(holderID)

	released := false
	defer func() {
		if !released {
			if err := s.discardSQLServerLockWaiter(); err != nil {
				s.t.Error("could not discard the SQL Server lock waiter during cleanup")
			}
		}
	}()

	s.runSQLServerLockWait(diagnostic, holderID, waiterID, 10000, time.Second)
	// go-mssqldb reports the deadline and invalidates the connection because
	// context.DeadlineExceeded implements net.Error.
	if err := s.discardSQLServerLockWaiter(); err != nil {
		s.t.Fatal("could not discard the SQL Server waiter after the request deadline")
	}
	s.observer = s.lease()
	waiterID = s.sqlServerLockWaiterID(holderID)
	s.checkSQLServerLockWaiterState(waiterID)
	s.checkParent()

	s.runSQLServerLockWait(diagnostic, holderID, waiterID, 1000, 10*time.Second)
	cleanupCtx, finishCleanup := context.WithTimeout(s.ctx, 5*time.Second)
	defer finishCleanup()
	if _, err := s.observer.ExecContext(cleanupCtx, "SET LOCK_TIMEOUT -1"); err != nil {
		s.t.Fatal("could not restore SQL Server waiter lock timeout")
	}
	s.checkSQLServerLockWaiterState(waiterID)

	// Check the pool after verifying reuse on the native-timeout session.
	if err := s.observer.Close(); err != nil {
		s.t.Fatal("could not return the recovered SQL Server session")
	}
	released = true
	s.observer = s.lease()
	s.checkParent()

	s.facts["lock_deadline_cause"] = "context.DeadlineExceeded"
	s.facts["lock_timeout_native"] = "1222"
	s.facts["lock_wait_observed"] = "both"
}

func (s *scenario) sqlServerLockWaiterID(holderID int) int {
	s.t.Helper()
	s.verifyMarker(s.ctx, s.observer.QueryContext)
	var waiterID int
	if err := s.observer.QueryRowContext(s.ctx, "SELECT @@SPID").Scan(&waiterID); err != nil || holderID == waiterID {
		s.t.Fatal("SQL Server lock actors do not own separate sessions")
	}
	return waiterID
}

func (s *scenario) discardSQLServerLockWaiter() error {
	err := s.observer.Raw(func(any) error { return driver.ErrBadConn })
	if errors.Is(err, driver.ErrBadConn) || errors.Is(err, sql.ErrConnDone) {
		return nil
	}
	return err
}

func (s *scenario) checkSQLServerLockWaiterState(waiterID int) {
	s.t.Helper()
	cleanupCtx, finishCleanup := context.WithTimeout(s.ctx, 5*time.Second)
	defer finishCleanup()
	var spid, transactionCount, restoredTimeout int
	if err := s.observer.QueryRowContext(cleanupCtx, "SELECT @@SPID, @@TRANCOUNT, @@LOCK_TIMEOUT").
		Scan(&spid, &transactionCount, &restoredTimeout); err != nil ||
		spid != waiterID || transactionCount != 0 || restoredTimeout != -1 {
		s.t.Fatal("SQL Server waiter session did not recover cleanly")
	}
}

func (s *scenario) openSQLServerLockObserver() *sql.Conn {
	s.t.Helper()
	password := os.Getenv("GOEF_DRIVERLAB_SQLSERVER_OBSERVER_PASSWORD")
	if password == "" {
		s.t.Fatal("SQL Server lock observer password is missing")
	}
	dsn := url.URL{
		Scheme: "sqlserver",
		User:   url.UserPassword(sqlServerLockObserver, password),
		Host:   sqlServerAddress,
		RawQuery: url.Values{
			"database":               {sqlServerCatalog},
			"encrypt":                {"true"},
			"protocol":               {"tcp"},
			"TrustServerCertificate": {"true"},
		}.Encode(),
	}
	connector, err := mssql.NewConnector(dsn.String())
	if err != nil {
		s.t.Fatal("could not configure the SQL Server lock observer")
	}
	connector.Dialer = sqlServerDialer{}
	db := sql.OpenDB(connector)
	db.SetMaxOpenConns(1)
	s.t.Cleanup(func() {
		if err := db.Close(); err != nil {
			s.t.Error("could not close the SQL Server lock observer pool")
		}
	})
	conn, err := db.Conn(s.ctx)
	if err != nil {
		s.t.Fatal("could not lease the SQL Server lock observer")
	}
	s.t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			s.t.Error("could not release the SQL Server lock observer")
		}
	})

	var login, catalog string
	if err := conn.QueryRowContext(s.ctx, "SELECT ORIGINAL_LOGIN(), DB_NAME()").Scan(&login, &catalog); err != nil ||
		login != sqlServerLockObserver || catalog != sqlServerCatalog {
		s.t.Fatal("SQL Server lock observer identity differs from the fixture")
	}
	s.verifyMarker(s.ctx, conn.QueryContext)
	return conn
}

func (s *scenario) runSQLServerLockWait(diagnostic *sql.Conn, holderID, waiterID, serverTimeoutMS int, requestTimeout time.Duration) {
	s.t.Helper()
	// SET LOCK_TIMEOUT is session-local and takes an integer literal.
	setTimeout := "SET LOCK_TIMEOUT " + strconv.Itoa(serverTimeoutMS)
	if _, err := s.observer.ExecContext(s.ctx, setTimeout); err != nil {
		s.t.Fatal("could not set the SQL Server waiter lock timeout")
	}
	var activeTimeout int
	if err := s.observer.QueryRowContext(s.ctx, "SELECT @@LOCK_TIMEOUT").Scan(&activeTimeout); err != nil ||
		activeTimeout != serverTimeoutMS {
		s.t.Fatal("SQL Server waiter lock timeout was not set")
	}

	// The holder's transaction lives independently of the blocked request.
	tx, err := s.writer.BeginTx(s.ctx, nil)
	if err != nil {
		s.t.Fatal("could not start the SQL Server lock holder transaction")
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			s.t.Error("could not roll back the SQL Server lock holder")
		}
	}()
	const update = "UPDATE goef_driver_parent SET code = @p1 WHERE id = @p2"
	result, err := tx.ExecContext(s.ctx, update, "held", 1)
	if err != nil {
		s.t.Fatal("could not acquire the SQL Server fixture row lock")
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		s.t.Fatal("SQL Server lock holder did not update one fixture row")
	}

	requestCtx, cancel := context.WithTimeout(s.ctx, requestTimeout)
	defer cancel()
	type outcome struct {
		err        error
		contextErr error
	}
	done := make(chan outcome, 1)
	go func() {
		_, err := s.observer.ExecContext(requestCtx, update, "waiter", 1)
		done <- outcome{err: err, contextErr: requestCtx.Err()}
	}()
	observeCtx, stopObserve := context.WithTimeout(s.ctx, 2*time.Second)
	observeErr := observeSQLServerLockWait(observeCtx, diagnostic, holderID, waiterID)
	requestActiveAtObservation := requestCtx.Err() == nil
	stopObserve()
	if observeErr != nil {
		cancel()
	}
	blocked := <-done
	if errors.Is(observeErr, context.DeadlineExceeded) {
		s.t.Fatal("SQL Server lock wait was not observed before the watchdog")
	}
	if observeErr != nil {
		s.t.Fatal("could not observe the SQL Server lock wait")
	}
	if !requestActiveAtObservation {
		s.t.Fatal("SQL Server lock wait was observed after the request context ended")
	}

	var native mssql.Error
	if requestTimeout.Milliseconds() < int64(serverTimeoutMS) {
		if !errors.Is(blocked.err, context.DeadlineExceeded) || !errors.Is(blocked.contextErr, context.DeadlineExceeded) ||
			errors.As(blocked.err, &native) {
			s.t.Fatal("SQL Server lock waiter did not report the request deadline")
		}
	} else if blocked.contextErr != nil || !errors.As(blocked.err, &native) || native.Number != 1222 || native.Class != 16 {
		s.t.Fatal("SQL Server lock waiter did not report native timeout 1222 under a live request")
	}

	if err := tx.Rollback(); err != nil {
		s.t.Fatal("could not finish the SQL Server lock holder rollback")
	}
}

func observeSQLServerLockWait(ctx context.Context, diagnostic *sql.Conn, holderID, waiterID int) error {
	const query = `
SELECT CAST(CASE WHEN EXISTS (
    SELECT 1
    FROM sys.dm_exec_requests
    WHERE session_id = @p1 AND blocking_session_id = @p2
      AND database_id = DB_ID() AND wait_type LIKE N'LCK[_]M[_]%'
) THEN 1 ELSE 0 END AS bit);`
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var waiting bool
		if err := diagnostic.QueryRowContext(ctx, query, waiterID, holderID).Scan(&waiting); err != nil {
			return err
		}
		if waiting {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
