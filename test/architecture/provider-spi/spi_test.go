package spi_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"slices"
	"testing"
	"time"

	"github.com/igorpooh1978/GoEF/provider"
)

type backend struct{ major uint16 }

func (b backend) Descriptor() provider.Descriptor {
	return provider.Descriptor{ID: "fixture", Major: b.major}
}

func (backend) Acquire(context.Context) (provider.Lease, error) {
	return &lease{}, nil
}

type lease struct{ dispatches int }

func (l *lease) Query(ctx context.Context, command provider.Command, args provider.Bindings) (provider.Rows, error) {
	if err := command.ValidateBindings(args); err != nil {
		return nil, err
	}
	l.dispatches++
	copied := slices.Clone(args)
	for i, value := range copied {
		if binary, ok := value.(provider.Binary); ok {
			copied[i] = slices.Clone(binary)
		}
	}
	return &rows{ctx: ctx, row: copied}, nil
}

func (l *lease) Exec(_ context.Context, command provider.Command, args provider.Bindings) (provider.WriteResult, error) {
	if err := command.ValidateBindings(args); err != nil {
		return provider.WriteResult{}, err
	}
	l.dispatches++
	return provider.WriteResult{RowsAffectedKnown: len(args) == 1}, nil
}

func (*lease) Release(context.Context, provider.Disposition) error { return nil }

type rows struct {
	ctx context.Context
	row []provider.Value
}

func (r *rows) Next() ([]provider.Value, error) {
	if err := r.ctx.Err(); err != nil {
		return nil, err
	}
	if r.row == nil {
		return nil, io.EOF
	}
	value := r.row
	r.row = nil
	return value, nil
}

func (r *rows) Close(context.Context) error {
	r.row = nil
	return nil
}

var (
	_ provider.Backend = backend{}
	_ provider.Lease   = (*lease)(nil)
	_ provider.Rows    = (*rows)(nil)
)

func TestExternalQueryBoundary(t *testing.T) {
	t.Parallel()
	var b provider.Backend = backend{major: provider.SPIMajor}
	if !b.Descriptor().Compatible() || (backend{major: provider.SPIMajor + 1}).Descriptor().Compatible() {
		t.Fatal("descriptor did not distinguish the supported contract")
	}
	ctx := t.Context()
	connection, err := b.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := connection.Release(ctx, provider.Discard); err != nil {
			t.Fatal(err)
		}
	}()

	command := provider.Command{
		SQL:     "select ?",
		Slots:   []provider.Slot{{Kind: provider.KindText}},
		Columns: []provider.Column{{Kind: provider.KindText}},
	}
	result, err := connection.Query(ctx, command, provider.Bindings{provider.Text("fixture")})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := result.Close(ctx); err != nil {
			t.Fatal(err)
		}
	}()
	row, err := result.Next()
	if err != nil || !slices.Equal(row, []provider.Value{provider.Text("fixture")}) {
		t.Fatalf("Next() = %v, %v", row, err)
	}
	if row, err := result.Next(); err != io.EOF || row != nil {
		t.Fatalf("Next() after exhaustion = %v, %v; want nil, io.EOF", row, err)
	}
}

func TestExternalWriteBoundary(t *testing.T) {
	t.Parallel()
	var executor provider.Executor = &lease{}
	ctx := t.Context()
	known, err := executor.Exec(ctx, provider.Command{
		SQL:   "delete from fixture where id = ?",
		Slots: []provider.Slot{{Kind: provider.KindInteger}},
	}, provider.Bindings{provider.Integer(7)})
	if err != nil || known.RowsAffected != 0 || !known.RowsAffectedKnown {
		t.Fatalf("Exec with affected-count support = %+v, %v; want known zero", known, err)
	}
	unknown, err := executor.Exec(ctx, provider.Command{SQL: "delete from fixture"}, nil)
	if err != nil || unknown.RowsAffectedKnown {
		t.Fatalf("Exec without affected-count support = %+v, %v; want unknown", unknown, err)
	}
}

func TestExternalBindingValidation(t *testing.T) {
	t.Parallel()
	text := provider.Text("abc")
	tests := []struct {
		name string
		slot provider.Slot
		args provider.Bindings
	}{
		{name: "missing", slot: provider.Slot{Kind: provider.KindText}},
		{name: "wrong kind", slot: provider.Slot{Kind: provider.KindText}, args: provider.Bindings{provider.Integer(1)}},
		{name: "not nullable", slot: provider.Slot{Kind: provider.KindText}, args: provider.Bindings{provider.Null{}}},
		{name: "nil interface", slot: provider.Slot{Kind: provider.KindText, Nullable: true}, args: provider.Bindings{nil}},
		{name: "typed nil", slot: provider.Slot{Kind: provider.KindText}, args: provider.Bindings{(*provider.Text)(nil)}},
		{name: "pointer value", slot: provider.Slot{Kind: provider.KindText}, args: provider.Bindings{&text}},
		{name: "invalid slot", slot: provider.Slot{Kind: provider.KindNull, Nullable: true}, args: provider.Bindings{provider.Null{}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			connection := &lease{}
			ctx := t.Context()
			command := provider.Command{SQL: "select ?", Slots: []provider.Slot{tc.slot}}
			if result, err := connection.Query(ctx, command, tc.args); err == nil || result != nil {
				t.Errorf("Query() = %v, %v; want nil rows and error", result, err)
			}
			if _, err := connection.Exec(ctx, command, tc.args); err == nil {
				t.Error("Exec() accepted an invalid binding")
			}
			if connection.dispatches != 0 {
				t.Errorf("dispatches = %d; want 0", connection.dispatches)
			}
		})
	}
	nullable := provider.Command{Slots: []provider.Slot{{Kind: provider.KindText, Nullable: true}}}
	if err := nullable.ValidateBindings(provider.Bindings{provider.Null{}}); err != nil {
		t.Errorf("ValidateBindings() rejected a permitted null: %v", err)
	}
}

func TestExternalBinaryOwnership(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	connection := &lease{}
	binding := provider.Binary{1, 2, 3}
	command := provider.Command{
		SQL: "select ?", Slots: []provider.Slot{{Kind: provider.KindBinary}},
		Columns: []provider.Column{{Kind: provider.KindBinary}},
	}
	result, err := connection.Query(ctx, command, provider.Bindings{binding})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := result.Close(ctx); err != nil {
			t.Fatal(err)
		}
	}()
	binding[0] = 9
	row, err := result.Next()
	if err != nil || len(row) != 1 {
		t.Fatalf("Next() = %v, %v", row, err)
	}
	binary, ok := row[0].(provider.Binary)
	if !ok || !bytes.Equal(binary, []byte{1, 2, 3}) {
		t.Fatalf("Next() retained the mutable binding: %v", row)
	}
}

func TestExternalStreamCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var connection provider.Lease = &lease{}
	result, err := connection.Query(ctx, provider.Command{
		SQL: "select ?", Slots: []provider.Slot{{Kind: provider.KindText}},
		Columns: []provider.Column{{Kind: provider.KindText}},
	}, provider.Bindings{provider.Text("fixture")})
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	if row, err := result.Next(); row != nil || !errors.Is(err, context.Canceled) {
		t.Errorf("Next() after cancellation = %v, %v; want nil, context.Canceled", row, err)
	}
	cleanup, stop := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
	defer stop()
	if err := errors.Join(result.Close(cleanup), connection.Release(cleanup, provider.Discard)); err != nil {
		t.Errorf("cleanup after cancellation: %v", err)
	}
}
