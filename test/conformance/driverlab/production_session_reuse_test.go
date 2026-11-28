package driverlab

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"sync/atomic"
	"testing"

	"github.com/igorpooh1978/GoEF/provider"
)

type sessionReuseConnector struct {
	driver.Connector
	opened atomic.Int64
}

func (c *sessionReuseConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.Connector.Connect(ctx)
	if err == nil {
		c.opened.Add(1)
	}
	return conn, err
}

func openSessionReusePool(s *scenario) *sessionReuseConnector {
	s.t.Helper()
	connector := &sessionReuseConnector{Connector: s.connector}
	db := sql.OpenDB(connector)
	s.t.Cleanup(func() {
		if err := db.Close(); err != nil {
			s.t.Error("could not close the session reuse pool")
		}
	})
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	s.db = db
	s.readBackend = productionBackend(s)
	if err := db.PingContext(s.ctx); err != nil || connector.opened.Load() != 1 {
		s.t.Fatal("could not warm the isolated physical connection")
	}
	return connector
}

type readCommand func(context.Context, provider.Command, provider.Bindings) (provider.Rows, error)

func reviewedQuery(s *scenario, reader *spiScenarioReader) readCommand {
	s.t.Helper()
	capability, ok := reader.lease.(provider.SessionNeutralQuerier)
	if !ok {
		s.t.Fatal("production lease did not expose the reviewed-read capability")
	}
	return capability.QuerySessionNeutral
}

func openSessionReuseRows(s *scenario, reader *spiScenarioReader, ctx context.Context, query readCommand) {
	s.t.Helper()
	command := provider.Command{
		SQL:     "SELECT id FROM goef_driver_parent WHERE id >= " + s.bind(1) + " ORDER BY id",
		Slots:   []provider.Slot{{Kind: provider.KindInteger}},
		Columns: []provider.Column{{Kind: provider.KindInteger}},
	}
	rows, err := query(ctx, command, provider.Bindings{provider.Integer(1)})
	if err != nil || rows == nil {
		s.t.Fatalf("could not open the session reuse control query: %v", err)
	}
	reader.rows = rows
}

func readSessionReuseRows(s *scenario, reader *spiScenarioReader, query readCommand) {
	s.t.Helper()
	openSessionReuseRows(s, reader, s.ctx, query)
	s.checkSPIInteger(reader.rows, 1)
	requireSessionReuseEOF(s.t, reader.rows)
}

func requireSessionReuseEOF(t *testing.T, rows provider.Rows) {
	t.Helper()
	if row, err := rows.Next(); row != nil || err != io.EOF {
		t.Fatalf("expected clean exhaustion, got %v", err)
	}
}

func checkSessionReuseAttempt(s *scenario, connector *sessionReuseConnector, disposition provider.Disposition, wantNew int64, use func(*spiScenarioReader)) {
	s.t.Helper()
	before := connector.opened.Load()
	reader := productionLease(s)
	use(reader)
	if err := reader.finish(disposition); err != nil {
		s.t.Fatalf("could not release the session reuse lease: %v", err)
	}
	s.checkSPIPoolInUse(0)
	if err := s.db.PingContext(s.ctx); err != nil {
		s.t.Fatal("the caller-owned pool could not acquire a working connection")
	}
	if delta := connector.opened.Load() - before; delta != wantNew {
		s.t.Fatalf("opened %d new physical connections, want %d", delta, wantNew)
	}
}

func checkReviewedReadReuse(s *scenario) {
	s.seedParent()
	connector := openSessionReusePool(s)
	checkSessionReuseAttempt(s, connector, provider.Reuse, 0, func(reader *spiScenarioReader) {
		readSessionReuseRows(s, reader, reviewedQuery(s, reader))
	})
}

func checkUnsafeReadDiscard(s *scenario) {
	s.seedParent()
	connector := openSessionReusePool(s)
	for _, tc := range []struct {
		name string
		use  func(*scenario, *spiScenarioReader)
	}{
		{name: "ordinary-read", use: func(s *scenario, reader *spiScenarioReader) {
			readSessionReuseRows(s, reader, reader.lease.Query)
		}},
		{name: "session-mutation", use: checkMutatedSession},
		{name: "incomplete-stream", use: func(s *scenario, reader *spiScenarioReader) {
			openSessionReuseRows(s, reader, s.ctx, reviewedQuery(s, reader))
			s.checkSPIInteger(reader.rows, 1)
		}},
		{name: "canceled-stream", use: func(s *scenario, reader *spiScenarioReader) {
			ctx, cancel := context.WithCancel(s.ctx)
			defer cancel()
			openSessionReuseRows(s, reader, ctx, reviewedQuery(s, reader))
			s.checkSPIInteger(reader.rows, 1)
			cancel()
			if _, err := reader.rows.Next(); !errors.Is(err, context.Canceled) {
				s.t.Fatalf("expected causal cancellation, got %v", err)
			}
		}},
		{name: "driver-error", use: func(s *scenario, reader *spiScenarioReader) {
			command := provider.Command{
				SQL:     "SELECT id FROM goef_driver_absent_reuse_control",
				Columns: []provider.Column{{Kind: provider.KindInteger}},
			}
			rows, err := reviewedQuery(s, reader)(s.ctx, command, nil)
			if err == nil && rows != nil {
				reader.rows = rows
				_, err = rows.Next()
			}
			if err == nil || err == io.EOF {
				s.t.Fatal("expected the invalid query to fail")
			}
		}},
	} {
		s.t.Run(tc.name, func(t *testing.T) {
			sub := *s
			sub.t = t
			checkSessionReuseAttempt(&sub, connector, provider.Reuse, 1, func(reader *spiScenarioReader) {
				tc.use(&sub, reader)
			})
		})
	}
}

type sessionMutation struct {
	setSQL  string
	readSQL string
	value   int64
}

func checkMutatedSession(s *scenario, reader *spiScenarioReader) {
	s.t.Helper()
	mutation := map[string]sessionMutation{
		"postgres": {
			"SET goef.driver_reuse_control TO '4242'",
			"SELECT COALESCE(NULLIF(current_setting('goef.driver_reuse_control', true), ''), '0')::bigint",
			4242,
		},
		"sqlite": {
			"PRAGMA cache_size=4242",
			"PRAGMA cache_size",
			4242,
		},
		"mysql": {
			"SET @goef_driver_reuse_control = 4242",
			"SELECT CAST(COALESCE(@goef_driver_reuse_control, 0) AS SIGNED)",
			4242,
		},
		"sqlserver": {
			"SET CONTEXT_INFO 0x00001092",
			"SELECT CAST(CASE WHEN CONTEXT_INFO() = 0x00001092 THEN 4242 ELSE 0 END AS bigint)",
			4242,
		},
	}[s.provider]
	if _, err := reader.lease.Exec(s.ctx, provider.Command{SQL: mutation.setSQL}, nil); err != nil {
		s.t.Fatalf("could not alter isolated session state: %v", err)
	}
	// Reading the setting also checks that a later reviewed read cannot make
	// earlier arbitrary SQL reusable even when its own stream ends cleanly.
	var err error
	reader.rows, err = reviewedQuery(s, reader)(s.ctx, provider.Command{
		SQL:     mutation.readSQL,
		Columns: []provider.Column{{Kind: provider.KindInteger}},
	}, nil)
	if err != nil || reader.rows == nil {
		s.t.Fatalf("could not read the changed session setting: %v", err)
	}
	s.checkSPIInteger(reader.rows, provider.Integer(mutation.value))
	requireSessionReuseEOF(s.t, reader.rows)
	s.t.Cleanup(func() {
		var observed int64
		if err := s.db.QueryRowContext(s.ctx, mutation.readSQL).Scan(&observed); err != nil || observed == mutation.value {
			s.t.Error("the next physical connection retained the discarded session setting")
		}
	})
}

func checkExplicitReadDiscard(s *scenario) {
	s.seedParent()
	connector := openSessionReusePool(s)
	checkSessionReuseAttempt(s, connector, provider.Discard, 1, func(reader *spiScenarioReader) {
		readSessionReuseRows(s, reader, reviewedQuery(s, reader))
	})
}
