package driverlab

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"

	"github.com/igorpooh1978/GoEF/provider"
)

// spiReadBackend adapts the lab's pinned drivers for the SPI streaming fixture.
// The fixture uses a single non-null integer column and no bindings.
type spiReadBackend struct {
	id string
	db *sql.DB
}

var _ provider.Backend = spiReadBackend{}

func (b spiReadBackend) Descriptor() provider.Descriptor {
	return provider.Descriptor{ID: b.id, Major: provider.SPIMajor}
}

func (b spiReadBackend) Acquire(ctx context.Context) (provider.Lease, error) {
	conn, err := b.db.Conn(ctx)
	if err != nil {
		return nil, spiReadError(ctx, "could not acquire SPI fixture connection")
	}
	return &spiReadLease{conn: conn}, nil
}

type spiReadLease struct {
	conn *sql.Conn
}

func (l *spiReadLease) Query(ctx context.Context, command provider.Command, bindings provider.Bindings) (provider.Rows, error) {
	if l.conn == nil {
		return nil, errors.New("SPI fixture lease was released")
	}
	if len(bindings) != 0 || len(command.Slots) != 0 || len(command.Columns) != 1 ||
		command.Columns[0] != (provider.Column{Kind: provider.KindInteger}) {
		return nil, errors.New("SPI fixture requires an unbound integer query")
	}
	rows, err := l.conn.QueryContext(ctx, command.SQL)
	if err != nil {
		return nil, spiReadError(ctx, "could not open SPI fixture rows")
	}
	return &spiReadRows{rows: rows, queryCtx: ctx}, nil
}

func (*spiReadLease) Exec(context.Context, provider.Command, provider.Bindings) (provider.WriteResult, error) {
	return provider.WriteResult{}, errors.New("SPI read fixture accepts queries only")
}

func (l *spiReadLease) Release(ctx context.Context, disposition provider.Disposition) error {
	conn := l.conn
	if conn == nil {
		return errors.New("SPI fixture lease was released")
	}
	l.conn = nil
	if disposition == provider.Reuse && ctx.Err() == nil {
		if conn.Close() != nil {
			return errors.New("could not return SPI fixture connection")
		}
		return nil
	}
	// Raw retires the physical connection even when the driver has not marked
	// it bad itself. A driver may already have retired it during cancellation.
	err := conn.Raw(func(any) error { return driver.ErrBadConn })
	if !errors.Is(err, driver.ErrBadConn) && !errors.Is(err, sql.ErrConnDone) {
		return errors.New("could not discard SPI fixture connection")
	}
	return nil
}

type spiReadRows struct {
	rows      *sql.Rows
	queryCtx  context.Context
	exhausted bool
}

func (r *spiReadRows) Next() ([]provider.Value, error) {
	if r.exhausted {
		return nil, io.EOF
	}
	if r.rows == nil {
		return nil, errors.New("SPI fixture rows were closed")
	}
	// Drivers may buffer rows before the owner cancels. Check the query
	// context before returning any of that data to the SPI consumer.
	if err := r.queryCtx.Err(); err != nil {
		return nil, err
	}
	if !r.rows.Next() {
		if err := r.queryCtx.Err(); err != nil {
			return nil, err
		}
		if r.rows.Err() != nil {
			return nil, errors.New("SPI fixture row iteration failed")
		}
		r.exhausted = true
		return nil, io.EOF
	}
	var value int64
	if r.rows.Scan(&value) != nil {
		return nil, spiReadError(r.queryCtx, "could not read SPI fixture integer")
	}
	if err := r.queryCtx.Err(); err != nil {
		return nil, err
	}
	return []provider.Value{provider.Integer(value)}, nil
}

func (r *spiReadRows) Close(context.Context) error {
	rows := r.rows
	if rows == nil {
		return nil
	}
	r.rows = nil
	// Some drivers return the already reported operation cause from Close.
	// Preserve distinct cleanup failures for the caller.
	if err := rows.Close(); err != nil && err != r.queryCtx.Err() {
		return errors.New("could not close SPI fixture rows")
	}
	return nil
}

func spiReadError(ctx context.Context, message string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return errors.New(message)
}
