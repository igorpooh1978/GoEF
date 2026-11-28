package sqladapter

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"slices"

	"github.com/igorpooh1978/GoEF/goeferr"
	"github.com/igorpooh1978/GoEF/provider"
)

type stream struct {
	lease     *lease
	rows      *sql.Rows
	ctx       context.Context
	columns   []provider.Column
	raw       []any
	targets   []any
	values    []provider.Value
	terminal  error
	nativeErr error
	mayReuse  bool
}

func newStream(lease *lease, rows *sql.Rows, ctx context.Context, columns []provider.Column) *stream {
	result := &stream{
		lease: lease, rows: rows, ctx: ctx, columns: slices.Clone(columns),
		raw: make([]any, len(columns)), targets: make([]any, len(columns)),
		values: make([]provider.Value, len(columns)),
	}
	for i := range result.raw {
		result.targets[i] = &result.raw[i]
	}
	return result
}

func (s *stream) Next() ([]provider.Value, error) {
	if s.rows == nil {
		return nil, goeferr.NewInvalidState()
	}
	if s.terminal != nil {
		return nil, s.terminal
	}
	if err := s.ctx.Err(); err != nil {
		return s.fail(err)
	}
	if !s.rows.Next() {
		if err := s.rows.Err(); err != nil {
			return s.fail(err)
		}
		if err := s.ctx.Err(); err != nil {
			return s.fail(err)
		}
		more := s.rows.NextResultSet()
		if err := s.rows.Err(); err != nil {
			return s.fail(err)
		}
		if err := s.ctx.Err(); err != nil {
			return s.fail(err)
		}
		s.terminal = io.EOF
		if more {
			s.terminal = goeferr.NewUnsupportedFeature()
		}
		return nil, s.terminal
	}
	if err := s.rows.Scan(s.targets...); err != nil {
		if status := s.ctx.Err(); status != nil && errors.Is(err, status) {
			return s.fail(err)
		}
		s.terminal = s.lease.backend.materialization()
		return nil, s.terminal
	}
	if err := s.ctx.Err(); err != nil {
		return s.fail(err)
	}
	for i, raw := range s.raw {
		value, ok := cellValue(raw, s.columns[i], s.lease.backend.config.IntegerBool)
		if !ok {
			s.terminal = s.lease.backend.materialization()
			return nil, s.terminal
		}
		s.values[i] = value
	}
	return s.values, nil
}

func (s *stream) fail(err error) ([]provider.Value, error) {
	s.nativeErr = err
	s.terminal = s.lease.backend.failure(s.ctx, goeferr.PhaseRead, err)
	return nil, s.terminal
}

// database/sql has no context-aware Rows.Close. Close synchronously so the
// leased connection cannot escape while native result cleanup is in progress.
func (s *stream) Close(_ context.Context) error {
	if s.rows == nil {
		return nil
	}
	rows := s.rows
	s.rows = nil
	s.lease.rows = nil
	clear(s.raw)
	clear(s.values)
	err := rows.Close()
	if s.mayReuse && s.terminal == io.EOF && err == nil && s.ctx.Err() == nil {
		s.lease.discard = false
	}
	if s.nativeErr != nil && errors.Is(err, s.nativeErr) {
		return nil
	}
	return s.lease.backend.failure(s.ctx, goeferr.PhaseRelease, err)
}
