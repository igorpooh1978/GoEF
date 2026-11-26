package transaction_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"testing"

	"github.com/igorpooh1978/GoEF/goeferr"
	"github.com/igorpooh1978/GoEF/transaction"
)

type optionProbe struct {
	*probe
	begins          int
	seen            driver.TxOptions
	readUncommitted int64
	queryErr        error
}

func (p *optionProbe) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := p.probe.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &optionConn{probeConn: conn.(*probeConn), options: p}, nil
}

type optionConn struct {
	*probeConn
	options *optionProbe
}

func (c *optionConn) BeginTx(ctx context.Context, options driver.TxOptions) (driver.Tx, error) {
	c.options.begins++
	c.options.seen = options
	return c.probeConn.BeginTx(ctx, options)
}

func (c *optionConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	if c.options.queryErr != nil {
		return nil, c.options.queryErr
	}
	if query == "PRAGMA read_uncommitted" {
		return &optionRows{value: c.options.readUncommitted}, nil
	}
	return &optionRows{value: 42, eof: query == "empty"}, nil
}

type optionRows struct {
	value int64
	eof   bool
}

func (*optionRows) Columns() []string { return []string{"value"} }
func (*optionRows) Close() error      { return nil }

func (r *optionRows) Next(values []driver.Value) error {
	if r.eof {
		return io.EOF
	}
	values[0] = r.value
	r.eof = true
	return nil
}

func TestTransactionOptionsPreflight(t *testing.T) {
	for _, tc := range []struct {
		dialect transaction.Dialect
		opts    sql.TxOptions
	}{
		{"invalid", sql.TxOptions{}},
		{transaction.DialectPostgreSQL, sql.TxOptions{Isolation: sql.LevelSnapshot}},
		{transaction.DialectMySQL, sql.TxOptions{Isolation: sql.LevelLinearizable}},
		{transaction.DialectSQLServer, sql.TxOptions{ReadOnly: true}},
		{transaction.DialectSQLite, sql.TxOptions{Isolation: sql.LevelRepeatableRead}},
		{transaction.DialectSQLite, sql.TxOptions{ReadOnly: true}},
	} {
		p := &optionProbe{probe: new(probe)}
		db := sql.OpenDB(p)
		tx, err := transaction.BeginWithOptions(t.Context(), db, tc.dialect, tc.opts)
		if tx != nil || !errors.Is(err, goeferr.ErrUnsupportedFeature) || p.open.Load() != 0 {
			t.Fatalf("options %v on %s opened a connection or returned %v", tc.opts, tc.dialect, err)
		}
		_ = db.Close()
	}
}

func TestTransactionOptionsForwardAndFinalize(t *testing.T) {
	for _, dialect := range []transaction.Dialect{transaction.DialectPostgreSQL, transaction.DialectMySQL, transaction.DialectSQLServer, transaction.DialectSQLite} {
		t.Run(string(dialect), func(t *testing.T) {
			p := &optionProbe{probe: new(probe)}
			db := sql.OpenDB(p)
			db.SetMaxOpenConns(1)
			t.Cleanup(func() { _ = db.Close() })
			opts := sql.TxOptions{Isolation: sql.LevelSerializable, ReadOnly: dialect == transaction.DialectPostgreSQL || dialect == transaction.DialectMySQL}
			tx, err := transaction.BeginWithOptions(t.Context(), db, dialect, opts)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = tx.Close() })
			want := driver.TxOptions{Isolation: driver.IsolationLevel(opts.Isolation), ReadOnly: opts.ReadOnly}
			if dialect == transaction.DialectSQLite {
				want = driver.TxOptions{}
			}
			if p.seen != want {
				t.Fatalf("native options = %v, want %v", p.seen, want)
			}
			if _, err := tx.Savepoint(t.Context(), "wrong"); !errors.Is(err, goeferr.ErrUnsupportedFeature) {
				t.Fatalf("wrong dialect savepoint = %v", err)
			}
			var value int
			if err := tx.Scan(t.Context(), "rows", nil, &value); err != nil || value != 42 {
				t.Fatalf("scan returned %d, %v", value, err)
			}
			if err := tx.Scan(t.Context(), "empty", nil, &value); !errors.Is(err, sql.ErrNoRows) {
				t.Fatalf("empty scan = %v", err)
			}
			if err := tx.Commit(); err != nil {
				t.Fatal(err)
			}
			checkState(t, tx, db, transaction.StateCommitted, 0)
			if (p.close.Load() == 1) != (dialect == transaction.DialectSQLServer) {
				t.Fatal("explicit SQL Server isolation requires discarding only its native session")
			}
		})
	}
}

func TestSQLiteSerializableRejectsReadUncommitted(t *testing.T) {
	p := &optionProbe{probe: new(probe), readUncommitted: 1}
	db := sql.OpenDB(p)
	t.Cleanup(func() { _ = db.Close() })
	tx, err := transaction.BeginWithOptions(t.Context(), db, transaction.DialectSQLite, sql.TxOptions{Isolation: sql.LevelSerializable})
	if tx != nil || !errors.Is(err, goeferr.ErrUnsupportedFeature) || p.open.Load() != 1 || p.begins != 0 || db.Stats().InUse != 0 {
		t.Fatalf("pragma rejection returned (%v, %v)", tx, err)
	}
}
