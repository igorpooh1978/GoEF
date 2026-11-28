package queryable

import (
	"context"
	"errors"
	"io"
	"reflect"
	"time"

	"github.com/igorpooh1978/GoEF/goeferr"
	"github.com/igorpooh1978/GoEF/provider"
)

// Fetch executes the plan once over a lease from the caller-owned backend.
// Zero matches return the zero result, false and nil. A second match returns
// [goeferr.ErrMultipleResults]. On all errors Fetch returns no partial result.
// An already canceled context stops before acquisition. Fetch closes its rows
// and releases its lease before returning. The caller owns the pool.
func (p Plan[Result, Parameter]) Fetch(ctx context.Context, backend provider.Backend, value Parameter) (Result, bool, error) {
	return p.fetch(ctx, backend, value, false)
}

// FetchSessionNeutral executes Fetch with explicit permission to reuse the
// physical session. Review the entire database read, including the objects it
// invokes, against the [provider.SessionNeutralQuerier] contract. If the lease
// lacks that capability, FetchSessionNeutral returns [goeferr.ErrUnsupportedFeature]
// without executing the plan.
func (p Plan[Result, Parameter]) FetchSessionNeutral(ctx context.Context, backend provider.Backend, value Parameter) (Result, bool, error) {
	return p.fetch(ctx, backend, value, true)
}

func (p Plan[Result, Parameter]) fetch(ctx context.Context, backend provider.Backend, value Parameter, sessionNeutral bool) (result Result, found bool, err error) {
	if ctx == nil || isNil(backend) || p.decode == nil {
		return result, false, goeferr.NewInvalidState()
	}
	if err := ctx.Err(); err != nil {
		detail := goeferr.NewCanceled()
		if err == context.DeadlineExceeded {
			detail = goeferr.NewDeadlineExceeded()
		}
		return result, false, &canceledFetch{error: detail, status: err, cause: context.Cause(ctx)}
	}
	if backend.Descriptor() != p.dialect {
		return result, false, goeferr.NewUnsupportedFeature()
	}
	args := provider.Bindings{argument(reflect.ValueOf(value), p.command.Slots[0].Kind)}
	lease, err := backend.Acquire(ctx)
	if err != nil {
		return result, false, err
	}
	var rows provider.Rows
	dispatched := false
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if rows != nil {
			err = errors.Join(err, rows.Close(cleanup))
		}
		disposition := provider.Reuse
		if ctx.Err() != nil || (dispatched && err != nil) {
			disposition = provider.Discard
		}
		err = errors.Join(err, lease.Release(cleanup, disposition))
		if err != nil {
			var zero Result
			result, found = zero, false
		}
	}()

	query := lease.Query
	if sessionNeutral {
		capability, ok := lease.(provider.SessionNeutralQuerier)
		if !ok {
			return result, false, goeferr.NewUnsupportedFeature()
		}
		query = capability.QuerySessionNeutral
	}
	dispatched = true
	rows, err = query(ctx, p.command, args)
	if err != nil {
		return result, false, err
	}
	return readSingle(rows, p.dialect.ID, len(p.command.Columns), p.decode)
}

// Match a private caller cause without including its text or exposing its
// concrete type through the public error chain.
type canceledFetch struct {
	error
	status, cause error
}

func (e *canceledFetch) Is(target error) bool {
	return errors.Is(e.status, target) || errors.Is(e.cause, target)
}

func (e *canceledFetch) Unwrap() error { return e.error }

func readSingle[Result any](rows provider.Rows, dialect string, columns int, decode func([]provider.Value, *Result) bool) (Result, bool, error) {
	var result, zero Result
	first, err := rows.Next()
	if err == io.EOF {
		return zero, false, nil
	}
	if err != nil {
		return zero, false, err
	}
	if len(first) != columns || !decode(first, &result) {
		return zero, false, goeferr.NewProviderFailure(goeferr.ErrMaterialization, dialect, goeferr.PhaseMaterialize, "")
	}
	_, err = rows.Next()
	if err == io.EOF {
		return result, true, nil
	}
	if err != nil {
		return zero, false, err
	}
	return zero, false, goeferr.NewMultipleResults()
}
