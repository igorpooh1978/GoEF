// Package sqladapter implements the driver-free database/sql transport shared by
// GoEF's separately distributed provider modules.
package sqladapter

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"

	"github.com/igorpooh1978/GoEF/goeferr"
	"github.com/igorpooh1978/GoEF/provider"
)

// Config describes a reviewed driver integration. Classify receives the native
// error privately and returns a registry sentinel and a structured native code.
// ClosePhysical can terminate a connection held by an underlying native pool.
type Config struct {
	ID            string
	IntegerBool   bool
	Classify      func(error) (error, string)
	ClosePhysical func(context.Context, any) error
}

// Backend implements the GoEF execution SPI over an application-owned pool.
type Backend struct {
	db     *sql.DB
	config Config
}

var _ provider.Backend = (*Backend)(nil)

// New binds a pool already validated by the concrete provider.
func New(db *sql.DB, config Config) (*Backend, error) {
	if db == nil || config.ID == "" || config.Classify == nil {
		return nil, goeferr.NewInvalidState()
	}
	return &Backend{db: db, config: config}, nil
}

// DB returns the application-owned pool for explicit database/sql operations.
func (b *Backend) DB() *sql.DB { return b.db }

// Descriptor reports static provider metadata without acquiring a connection.
func (b *Backend) Descriptor() provider.Descriptor {
	return provider.Descriptor{ID: b.config.ID, Major: provider.SPIMajor}
}

// Acquire transfers a lease on one physical connection. No user command is
// dispatched during acquisition.
func (b *Backend) Acquire(ctx context.Context) (provider.Lease, error) {
	if ctx == nil {
		return nil, goeferr.NewInvalidState()
	}
	conn, err := b.db.Conn(ctx)
	if err != nil {
		return nil, b.failure(ctx, goeferr.PhaseAcquire, err)
	}
	return &lease{backend: b, conn: conn}, nil
}

type lease struct {
	backend *Backend
	conn    *sql.Conn
	rows    *stream
	used    bool
}

func (l *lease) Query(ctx context.Context, command provider.Command, bindings provider.Bindings) (provider.Rows, error) {
	args, err := l.preflight(ctx, command, bindings, goeferr.PhaseRead)
	if err != nil {
		return nil, err
	}
	for _, column := range command.Columns {
		if column.Kind < provider.KindInteger || column.Kind > provider.KindTimestamp {
			return nil, goeferr.NewInvalidState()
		}
	}
	l.used = true
	rows, err := l.conn.QueryContext(ctx, command.SQL, args...)
	if err != nil {
		return nil, l.backend.failure(ctx, goeferr.PhaseRead, err)
	}
	names, err := rows.Columns()
	if err != nil {
		primary := l.backend.failure(ctx, goeferr.PhaseRead, err)
		return nil, errors.Join(primary, l.backend.failure(ctx, goeferr.PhaseRelease, rows.Close()))
	}
	if len(names) != len(command.Columns) {
		primary := l.backend.materialization()
		return nil, errors.Join(primary, l.backend.failure(ctx, goeferr.PhaseRelease, rows.Close()))
	}

	result := newStream(l, rows, ctx, command.Columns)
	l.rows = result
	return result, nil
}

func (l *lease) Exec(ctx context.Context, command provider.Command, bindings provider.Bindings) (provider.WriteResult, error) {
	args, err := l.preflight(ctx, command, bindings, goeferr.PhaseWrite)
	if err != nil {
		return provider.WriteResult{}, err
	}
	if len(command.Columns) != 0 {
		return provider.WriteResult{}, goeferr.NewInvalidState()
	}
	l.used = true
	result, err := l.conn.ExecContext(ctx, command.SQL, args...)
	if err != nil {
		return provider.WriteResult{}, l.backend.failure(ctx, goeferr.PhaseWrite, err)
	}
	count, err := result.RowsAffected()
	if err != nil || count < 0 {
		return provider.WriteResult{}, nil
	}
	return provider.WriteResult{RowsAffected: count, RowsAffectedKnown: true}, nil
}

func (l *lease) preflight(ctx context.Context, command provider.Command, bindings provider.Bindings, phase goeferr.Phase) ([]any, error) {
	if ctx == nil || l.conn == nil || command.SQL == "" {
		return nil, goeferr.NewInvalidState()
	}
	if l.rows != nil {
		return nil, goeferr.NewContextBusy()
	}
	if err := ctx.Err(); err != nil {
		return nil, l.backend.failure(ctx, phase, err)
	}
	if err := command.ValidateBindings(bindings); err != nil {
		return nil, errors.Join(goeferr.NewInvalidState(), err)
	}
	return arguments(command.Slots, bindings), nil
}

func (l *lease) Release(ctx context.Context, disposition provider.Disposition) error {
	conn := l.conn
	if conn == nil {
		return goeferr.NewInvalidState()
	}
	l.conn = nil

	var cleanup error
	if l.rows != nil {
		// Clean up even when the caller violated the serial lease contract.
		cleanup = errors.Join(goeferr.NewContextBusy(), l.rows.Close(ctx))
	}
	if ctx == nil {
		cleanup = errors.Join(cleanup, goeferr.NewInvalidState())
		ctx = context.Background()
		disposition = provider.Discard
	}
	if disposition == provider.Reuse && !l.used && ctx.Err() == nil {
		return errors.Join(cleanup, l.backend.failure(ctx, goeferr.PhaseRelease, conn.Close()))
	}

	// Raw marks the database/sql connection bad before closing it. An explicit
	// physical close is needed when the driver wraps an independent native pool.
	var physicalErr error
	discardErr := conn.Raw(func(native any) error {
		if closePhysical := l.backend.config.ClosePhysical; closePhysical != nil {
			physicalErr = closePhysical(ctx, native)
		}
		return driver.ErrBadConn
	})
	if errors.Is(discardErr, driver.ErrBadConn) || errors.Is(discardErr, sql.ErrConnDone) {
		discardErr = nil
	}
	return errors.Join(cleanup,
		l.backend.failure(ctx, goeferr.PhaseRelease, physicalErr),
		l.backend.failure(ctx, goeferr.PhaseRelease, discardErr))
}

func (b *Backend) materialization() error {
	return goeferr.NewProviderFailure(goeferr.ErrMaterialization, b.config.ID, goeferr.PhaseMaterialize, "")
}

func (b *Backend) failure(ctx context.Context, phase goeferr.Phase, err error) error {
	if err == nil {
		return nil
	}
	// A caller deadline and a separate server timeout can occur concurrently.
	// Only report cancellation when the driver returned its causal signal.
	if status := ctx.Err(); status != nil && errors.Is(err, status) {
		kind := goeferr.ErrCanceled
		if status == context.DeadlineExceeded {
			kind = goeferr.ErrDeadlineExceeded
		}
		return &cancellation{
			detail: goeferr.NewProviderFailure(kind, b.config.ID, phase, ""),
			status: status,
			cause:  context.Cause(ctx),
		}
	}
	kind, code := b.config.Classify(err)
	if kind == nil && (errors.Is(err, driver.ErrBadConn) || errors.Is(err, sql.ErrConnDone)) {
		kind = goeferr.ErrConnectionUnavailable
	}
	return goeferr.NewProviderFailure(kind, b.config.ID, phase, code)
}

// Keep the caller-defined cause observable to errors.Is without printing it or
// exposing arbitrary application or native errors through the unwrap chain.
type cancellation struct {
	detail *goeferr.ErrorDetail
	status error
	cause  error
}

func (e *cancellation) Error() string { return e.detail.Error() }
func (e *cancellation) Unwrap() error { return e.detail }
func (e *cancellation) Is(target error) bool {
	return errors.Is(e.status, target) || errors.Is(e.cause, target)
}
