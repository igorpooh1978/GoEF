package driverlab

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/stdlib"
)

const secondProjectionNotice = "goef-pg18-native-next-gate"

func TestPostgresNativeNext(t *testing.T) {
	if os.Getenv("GOEF_DRIVERLAB_POSTGRES_NEXT") != "1" {
		t.Skip("select this test with the isolated postgres_next.py runner")
	}
	if os.Getenv("GOEF_DRIVERLAB_PROVIDER") != "postgres" || os.Getenv("GOEF_DRIVERLAB_PROFILE") == "" {
		t.Fatal("the PostgreSQL Next test requires a selected PostgreSQL profile")
	}
	for _, test := range []struct {
		name    string
		control bool
	}{
		{name: "cancel-native-next"},
		{name: "holder-release-control", control: true},
	} {
		var result scenarioResult
		passed := t.Run(test.name, func(t *testing.T) {
			s := scenario{t: t, provider: "postgres"}
			s.setup()
			s.seedParent()
			facts := map[string]string{
				"engine_version":     s.facts["engine_version"],
				"server_version_num": s.facts["server_version_num"],
			}

			holder, err := s.writer.BeginTx(s.ctx, nil)
			if err != nil {
				t.Fatal("could not start the Next-probe holder")
			}
			holderOpen := true
			defer func() {
				if holderOpen && holder.Rollback() != nil {
					t.Error("could not clean up the Next-probe holder")
				}
			}()
			rollbackHolder := func() {
				holderOpen = false
				if holder.Rollback() != nil {
					t.Fatal("could not roll back the Next-probe holder")
				}
			}
			holderPID := s.postgresLockWaitPID(holder)
			const gate int64 = 2539015
			if _, err := holder.ExecContext(s.ctx, "SELECT pg_advisory_xact_lock($1::bigint)", gate); err != nil {
				t.Fatal("could not hold the advisory lock")
			}

			notices := make(chan struct{}, 1)
			pool := openNextProbePool(&s, notices)
			reader, err := pool.Conn(s.ctx)
			if err != nil {
				t.Fatal("could not acquire the Next-probe reader")
			}
			t.Cleanup(func() {
				if err := reader.Close(); err != nil && !errors.Is(err, sql.ErrConnDone) {
					t.Error("could not close the Next-probe reader")
				}
			})
			s.verifyMarker(s.ctx, reader.QueryContext)
			var readerPID int
			if reader.QueryRowContext(s.ctx, "SELECT pg_backend_pid()").Scan(&readerPID) != nil ||
				readerPID <= 0 || readerPID == holderPID {
				t.Fatal("could not identify independent Next-probe reader")
			}
			const function = `CREATE FUNCTION pg_temp.goef_next_probe_value(value integer, gate bigint)
				RETURNS integer LANGUAGE plpgsql VOLATILE PARALLEL UNSAFE AS $body$
				BEGIN
					IF value = 2 THEN
						RAISE NOTICE 'goef-pg18-native-next-gate';
						PERFORM pg_advisory_xact_lock(gate);
					END IF;
					RETURN value;
				END $body$`
			if _, err := reader.ExecContext(s.ctx, function); err != nil {
				t.Fatal("could not create the temporary Next-probe function")
			}

			request, cancel := context.WithTimeout(s.ctx, 10*time.Second)
			defer cancel()
			const query = `SELECT pg_temp.goef_next_probe_value(g.n, $1::bigint)
				FROM generate_series(1, 2) AS g(n) ORDER BY g.n`
			rows, err := reader.QueryContext(request, query, gate)
			if err != nil {
				t.Fatal("could not open the Next-probe query")
			}
			defer func() {
				if err := rows.Close(); err != nil && err != context.Canceled {
					t.Error("could not close the Next-probe rows")
				}
			}()
			var value int
			if !rows.Next() || rows.Scan(&value) != nil || value != 1 {
				t.Fatal("the Next-probe did not deliver its first row")
			}
			select {
			case <-notices:
				t.Fatal("pgx processed the second-row notice before the second Next call")
			default:
			}

			waitNextProbe(t, s.ctx, "advisory lock wait", func(ctx context.Context) (bool, error) {
				var blocked bool
				err := holder.QueryRowContext(ctx, "SELECT $1::integer = ANY(pg_blocking_pids($2::integer))",
					holderPID, readerPID).Scan(&blocked)
				return blocked, err
			})
			type nextResult struct {
				more bool
				err  error
			}
			results := make(chan nextResult, 1)
			workerDone := make(chan struct{})
			go func() {
				defer close(workerDone)
				more := rows.Next()
				if more {
					results <- nextResult{more: true}
					return
				}
				results <- nextResult{err: rows.Err()}
			}()
			defer func() {
				cancel()
				<-workerDone
			}()

			select {
			case <-notices:
			case <-workerDone:
				t.Fatal("native second Next returned without consuming its notice")
			case <-request.Done():
				t.Fatal("native second Next did not consume its notice while live")
			}
			var stillWaiting bool
			if err := holder.QueryRowContext(s.ctx, "SELECT $1::integer = ANY(pg_blocking_pids($2::integer))",
				holderPID, readerPID).Scan(&stillWaiting); err != nil || !stillWaiting || request.Err() != nil {
				t.Fatal("the observed second Next did not retain a live engine wait")
			}
			select {
			case <-workerDone:
				t.Fatal("native second Next returned before the cancellation decision")
			default:
			}
			facts["phase"] = "native-next"
			facts["first"] = "1"
			facts["lock_wait"] = "observed"
			if test.control {
				rollbackHolder()
			} else {
				cancel()
			}
			<-workerDone
			finished := <-results
			if test.control {
				if !finished.more || rows.Scan(&value) != nil || value != 2 || rows.Next() || rows.Err() != nil ||
					request.Err() != nil {
					t.Fatal("holder release did not finish the open reader with a second row and clean EOF")
				}
				facts["second"] = "2"
				facts["terminal"] = "clean-eof"
				result = scenarioResult{Provider: "postgres", Scenario: test.name, Status: "PASS", Facts: facts}
				return
			}
			if finished.more || !errors.Is(finished.err, context.Canceled) || !errors.Is(rows.Err(), context.Canceled) {
				t.Fatal("native second Next did not retain caller cancellation")
			}
			if err := rows.Close(); err != nil && err != context.Canceled {
				t.Fatal("could not close the canceled Next-probe rows")
			}
			err = reader.Raw(func(any) error { return driver.ErrBadConn })
			if !errors.Is(err, driver.ErrBadConn) && !errors.Is(err, sql.ErrConnDone) {
				t.Fatal("could not discard the canceled Next-probe reader")
			}
			if pool.Stats().InUse != 0 || pool.Stats().OpenConnections != 0 {
				t.Fatal("Next-probe reader pool retained a connection after discard")
			}
			waitNextProbe(t, s.ctx, "canceled backend exit", func(ctx context.Context) (bool, error) {
				if _, err := holder.ExecContext(ctx, "SELECT pg_stat_clear_snapshot()"); err != nil {
					return false, err
				}
				var exited bool
				err := holder.QueryRowContext(ctx, "SELECT NOT EXISTS (SELECT 1 FROM pg_stat_activity WHERE pid = $1)",
					readerPID).Scan(&exited)
				return exited, err
			})
			rollbackHolder()

			recovery, cancelRecovery := context.WithTimeout(s.ctx, 5*time.Second)
			defer cancelRecovery()
			fresh, err := pool.Conn(recovery)
			if err != nil {
				t.Fatal("could not acquire a fresh Next-probe reader")
			}
			defer func() {
				if err := fresh.Close(); err != nil && !errors.Is(err, sql.ErrConnDone) {
					t.Error("could not clean up the fresh Next-probe reader")
				}
			}()
			var freshPID, id int
			var code string
			err = fresh.QueryRowContext(recovery, "SELECT pg_backend_pid(), id, code FROM goef_driver_parent WHERE id = $1", 1).
				Scan(&freshPID, &id, &code)
			if err != nil || freshPID <= 0 || freshPID == readerPID || id != 1 || code != "original" {
				t.Fatal("fresh Next-probe reader could not read the committed row")
			}
			if fresh.Close() != nil || pool.Stats().InUse != 0 {
				t.Fatal("could not release the fresh Next-probe reader")
			}
			facts["terminal"] = "context.Canceled"
			facts["backend"] = "exited-before-unlock"
			facts["recovery"] = "original-row"
			result = scenarioResult{Provider: "postgres", Scenario: test.name, Status: "PASS", Facts: facts}
		})
		if passed && result.Facts != nil {
			report, err := json.Marshal(result)
			if err != nil {
				t.Fatal("could not encode the PostgreSQL Next result")
			}
			fmt.Printf("GOEF_POSTGRES_NEXT_RESULT=%s\n", report)
		}
	}
}

func openNextProbePool(s *scenario, notices chan<- struct{}) *sql.DB {
	s.t.Helper()
	var config *pgx.ConnConfig
	err := s.observer.Raw(func(connection any) error {
		owned, ok := connection.(*stdlib.Conn)
		if !ok {
			return errors.New("fixture is not using pgx stdlib")
		}
		config = owned.Conn().Config()
		return driver.ErrBadConn
	})
	if !errors.Is(err, driver.ErrBadConn) || config == nil {
		s.t.Fatal("could not copy the already-validated pgx fixture configuration")
	}
	config.RuntimeParams["client_min_messages"] = "notice"
	config.OnNotice = func(_ *pgconn.PgConn, notice *pgconn.Notice) {
		if notice.Message == secondProjectionNotice {
			select {
			case notices <- struct{}{}:
			default:
			}
		}
	}
	pool := stdlib.OpenDB(*config)
	pool.SetMaxOpenConns(1)
	pool.SetMaxIdleConns(1)
	s.t.Cleanup(func() {
		if pool.Close() != nil {
			s.t.Error("could not close the Next-probe pool")
		}
	})
	return pool
}

func waitNextProbe(t *testing.T, parent context.Context, phase string, check func(context.Context) (bool, error)) {
	t.Helper()
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()
	tick := time.NewTicker(20 * time.Millisecond)
	defer tick.Stop()
	for {
		ready, err := check(ctx)
		if err != nil {
			t.Fatalf("could not inspect Next-probe %s", phase)
		}
		if ready {
			return
		}
		select {
		case <-tick.C:
		case <-ctx.Done():
			t.Fatalf("did not observe Next-probe %s", phase)
		}
	}
}
