package sqladapter

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/igorpooh1978/GoEF/goeferr"
	"github.com/igorpooh1978/GoEF/provider"
)

type driverFixture struct {
	connects, closes int
	dispatches       int
	args             []driver.NamedValue
	connectErr       error
	execErr          error
	queryErr         error
	closeErr         error
	nextErr          error
	sets             [][][]driver.Value
	columnNames      []string
	affected         int64
	affectedErr      error
}

func (f *driverFixture) Connect(context.Context) (driver.Conn, error) {
	if f.connectErr != nil {
		return nil, f.connectErr
	}
	f.connects++
	return f, nil
}

func (f *driverFixture) Driver() driver.Driver            { return f }
func (f *driverFixture) Open(string) (driver.Conn, error) { return f.Connect(context.Background()) }
func (f *driverFixture) Close() error {
	f.closes++
	return nil
}
func (*driverFixture) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (*driverFixture) Begin() (driver.Tx, error)           { return nil, driver.ErrSkip }

func (f *driverFixture) ExecContext(_ context.Context, _ string, args []driver.NamedValue) (driver.Result, error) {
	f.dispatches++
	f.args = args
	if f.execErr != nil {
		return nil, f.execErr
	}
	return f, nil
}

func (f *driverFixture) LastInsertId() (int64, error) { return 0, driver.ErrSkip }
func (f *driverFixture) RowsAffected() (int64, error) { return f.affected, f.affectedErr }

func (f *driverFixture) QueryContext(_ context.Context, _ string, args []driver.NamedValue) (driver.Rows, error) {
	f.dispatches++
	f.args = args
	if f.queryErr != nil {
		return nil, f.queryErr
	}
	return &fakeRows{driver: f}, nil
}

type fakeRows struct {
	driver *driverFixture
	set    int
	row    int
}

func (r *fakeRows) Columns() []string { return r.driver.columnNames }
func (r *fakeRows) Close() error      { return r.driver.closeErr }
func (r *fakeRows) Next(dest []driver.Value) error {
	if r.set < len(r.driver.sets) && r.row < len(r.driver.sets[r.set]) {
		copy(dest, r.driver.sets[r.set][r.row])
		r.row++
		return nil
	}
	if r.driver.nextErr != nil {
		return r.driver.nextErr
	}
	return io.EOF
}
func (r *fakeRows) HasNextResultSet() bool { return r.set+1 < len(r.driver.sets) }
func (r *fakeRows) NextResultSet() error {
	r.set++
	r.row = 0
	return nil
}

var private = errors.New("private driver value")

func newFixture(t *testing.T, cfg Config) (*Backend, *driverFixture) {
	t.Helper()
	f := &driverFixture{}
	db := sql.OpenDB(f)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	cfg.ID = "sqlite"
	if cfg.Classify == nil {
		cfg.Classify = func(error) (error, string) { return nil, "" }
	}
	b, err := New(db, cfg)
	if err != nil {
		t.Fatal(err)
	}
	return b, f
}

func acquire(t *testing.T, b *Backend) provider.Lease {
	t.Helper()
	l, err := b.Acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func TestLeaseReleaseAndPoolOwnership(t *testing.T) {
	b, f := newFixture(t, Config{})
	if d := b.Descriptor(); d.ID != "sqlite" || !d.Compatible() || b.DB().Driver() != f {
		t.Fatal("unexpected provider descriptor or pool")
	}
	l := acquire(t, b)
	if err := l.Release(t.Context(), provider.Reuse); err != nil || f.closes != 0 {
		t.Fatalf("unused lease was not reusable: %v, %d closes", err, f.closes)
	}
	if err := l.Release(t.Context(), provider.Reuse); !errors.Is(err, goeferr.ErrInvalidState) {
		t.Fatalf("released twice: %v", err)
	}
	l = acquire(t, b)
	if _, err := l.Exec(t.Context(), provider.Command{SQL: "write"}, nil); err != nil {
		t.Fatal(err)
	}
	if err := l.Release(t.Context(), provider.Reuse); err != nil || f.closes != 1 {
		t.Fatalf("used connection was not discarded: %v, %d closes", err, f.closes)
	}
	l = acquire(t, b)
	if f.connects != 2 {
		t.Fatalf("same physical connection returned, opens: %d", f.connects)
	}
	if err := l.Release(nil, provider.Reuse); !errors.Is(err, goeferr.ErrInvalidState) || f.closes != 2 {
		t.Fatalf("invalid release failed to consume lease: %v, %d closes", err, f.closes)
	}
}

func TestAcquireRedactionAndCausalCancellation(t *testing.T) {
	b, f := newFixture(t, Config{})
	for _, cfg := range []Config{{}, {ID: "sqlite"}} {
		if _, err := New(b.DB(), cfg); !errors.Is(err, goeferr.ErrInvalidState) {
			t.Fatalf("accepted bad config: %v", err)
		}
	}
	f.connectErr = private
	if l, err := b.Acquire(t.Context()); l != nil || !errors.Is(err, goeferr.ErrUnknownDriver) ||
		errors.Is(err, private) || strings.Contains(err.Error(), private.Error()) {
		t.Fatalf("connection failure escaped as %v, %v", l, err)
	}
	if _, err := b.Acquire(nil); !errors.Is(err, goeferr.ErrInvalidState) {
		t.Fatalf("invalid context was accepted: %v", err)
	}
	f.connectErr = nil
	l := acquire(t, b)
	defer l.Release(t.Context(), provider.Discard)
	ctx, cancel := context.WithCancelCause(t.Context())
	cancel(private)
	if replacement, err := b.Acquire(ctx); replacement != nil || !errors.Is(err, goeferr.ErrCanceled) ||
		!errors.Is(err, context.Canceled) || !errors.Is(err, private) || strings.Contains(err.Error(), private.Error()) {
		t.Fatalf("causal cancellation: %v, %v", replacement, err)
	}
	timed, stop := context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
	defer stop()
	_, err := l.Exec(timed, provider.Command{SQL: "write"}, nil)
	if !errors.Is(err, goeferr.ErrDeadlineExceeded) || !errors.Is(err, context.DeadlineExceeded) || f.dispatches != 0 {
		t.Fatalf("deadline allowed dispatch: %v, %d", err, f.dispatches)
	}
	// Cancellation of an unrelated context does not turn a native timeout into caller cancellation.
	b.config.Classify = func(error) (error, string) { return goeferr.ErrTimeout, "1205" }
	err = b.failure(ctx, goeferr.PhaseRead, fmt.Errorf("wrapped: %w", private))
	if !errors.Is(err, goeferr.ErrTimeout) || errors.Is(err, context.Canceled) || errors.Is(err, private) {
		t.Fatalf("native timeout lost its identity: %v", err)
	}
}

func TestExecValidatesAndSeparatesValues(t *testing.T) {
	b, f := newFixture(t, Config{})
	l := acquire(t, b)
	defer l.Release(t.Context(), provider.Discard)
	timeValue := time.Date(2026, 11, 26, 1, 2, 3, 0, time.UTC)
	command := provider.Command{SQL: "write", Slots: []provider.Slot{
		{Name: "who", Kind: provider.KindText},
		{Kind: provider.KindInteger},
		{Kind: provider.KindFloat},
		{Kind: provider.KindBoolean},
		{Kind: provider.KindBinary},
		{Kind: provider.KindBinary},
		{Kind: provider.KindTimestamp},
		{Kind: provider.KindText, Nullable: true},
	}}
	bindings := provider.Bindings{
		provider.Text("bound string"), provider.Integer(3), provider.Float(3.5), provider.Boolean(true),
		provider.Binary(nil), provider.Binary{1}, provider.Timestamp(timeValue), provider.Null{},
	}
	if _, err := l.Exec(t.Context(), command, bindings[:7]); !errors.Is(err, goeferr.ErrInvalidState) {
		t.Fatalf("wrong value count was accepted: %v", err)
	}
	if _, err := l.Exec(t.Context(), provider.Command{}, nil); !errors.Is(err, goeferr.ErrInvalidState) {
		t.Fatalf("empty SQL: %v", err)
	}
	if _, err := l.Exec(t.Context(), provider.Command{SQL: "write", Columns: []provider.Column{{Kind: provider.KindText}}}, nil); !errors.Is(err, goeferr.ErrInvalidState) {
		t.Fatalf("write layout was accepted: %v", err)
	}
	if f.dispatches != 0 {
		t.Fatalf("invalid input dispatched %d times", f.dispatches)
	}
	result, err := l.Exec(t.Context(), command, bindings)
	if err != nil || result != (provider.WriteResult{RowsAffectedKnown: true}) {
		t.Fatalf("known zero result: %+v, %v", result, err)
	}
	if f.args[0].Name != "who" || f.args[1].Name != "" {
		t.Fatalf("named and positional parameters not preserved: %v", f.args)
	}
	want := []any{"bound string", int64(3), 3.5, true, []byte{}, []byte{1}, timeValue, nil}
	for i, arg := range f.args {
		if !reflect.DeepEqual(arg.Value, want[i]) {
			t.Fatalf("argument %d: got %#v, want %#v", i+1, arg.Value, want[i])
		}
	}
	f.affected = -1
	result, err = l.Exec(t.Context(), command, bindings)
	if err != nil || result.RowsAffectedKnown {
		t.Fatalf("unknown result: %+v, %v", result, err)
	}
	f.execErr = driver.ErrBadConn
	_, err = l.Exec(t.Context(), command, bindings)
	if !errors.Is(err, goeferr.ErrConnectionUnavailable) || f.dispatches != 3 || f.connects != 1 {
		t.Fatalf("bound write replayed or not classified: %v, dispatches %d, connections %d", err, f.dispatches, f.connects)
	}
}

func TestRowsUseDeclaredKindsAndBlockOtherCommands(t *testing.T) {
	b, f := newFixture(t, Config{IntegerBool: true})
	l := acquire(t, b)
	defer l.Release(t.Context(), provider.Discard)
	timeValue := time.Date(2026, 11, 26, 1, 2, 3, 0, time.UTC)
	f.columnNames = []string{"i", "f", "flag", "text", "data", "time", "nullable"}
	f.sets = [][][]driver.Value{{
		{int64(5), float64(2.5), true, "first", []byte{0}, timeValue, nil},
		{uint64(7), float32(1.5), int64(0), []byte("second"), []byte{2}, timeValue, []byte("nonnull")},
	}}
	command := provider.Command{SQL: "query", Columns: []provider.Column{
		{Kind: provider.KindInteger}, {Kind: provider.KindFloat}, {Kind: provider.KindBoolean},
		{Kind: provider.KindText}, {Kind: provider.KindBinary}, {Kind: provider.KindTimestamp},
		{Kind: provider.KindText, Nullable: true},
	}}
	rows, err := l.Query(t.Context(), command, nil)
	if err != nil {
		t.Fatal(err)
	}
	command.Columns[0].Kind = provider.KindText // retained layout must be copied
	if _, err := l.Query(t.Context(), command, nil); !errors.Is(err, goeferr.ErrContextBusy) {
		t.Fatalf("simultaneous stream allowed: %v", err)
	}
	want := [][]provider.Value{
		{provider.Integer(5), provider.Float(2.5), provider.Boolean(true), provider.Text("first"),
			provider.Binary{0}, provider.Timestamp(timeValue), provider.Null{}},
		{provider.Integer(7), provider.Float(1.5), provider.Boolean(false), provider.Text("second"),
			provider.Binary{2}, provider.Timestamp(timeValue), provider.Text("nonnull")},
	}
	for _, expected := range want {
		row, err := rows.Next()
		if err != nil || !reflect.DeepEqual(row, expected) {
			t.Fatalf("row: got %v / %v, want %v", row, err, expected)
		}
	}
	for range 2 {
		if row, err := rows.Next(); row != nil || !errors.Is(err, io.EOF) {
			t.Fatalf("unclean exhaustion: %v, %v", row, err)
		}
	}
	for range 2 {
		if err := rows.Close(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := rows.Next(); !errors.Is(err, goeferr.ErrInvalidState) {
		t.Fatalf("closed rows still readable: %v", err)
	}
}

func TestQueryFailureLifecycle(t *testing.T) {
	b, f := newFixture(t, Config{})
	l := acquire(t, b)
	command := provider.Command{SQL: "query", Columns: []provider.Column{{Kind: provider.KindInteger}}}
	f.columnNames = []string{"i"}
	f.sets = [][][]driver.Value{{{int64(2)}}}
	f.queryErr = private
	if rows, err := l.Query(t.Context(), command, nil); rows != nil || !errors.Is(err, goeferr.ErrUnknownDriver) || errors.Is(err, private) {
		t.Fatalf("opening error: rows %v, err %v", rows, err)
	}
	f.queryErr = nil
	if _, err := l.Query(t.Context(), provider.Command{SQL: "query", Columns: []provider.Column{{Kind: provider.KindNull}}}, nil); !errors.Is(err, goeferr.ErrInvalidState) {
		t.Fatalf("invalid layout accepted: %v", err)
	}
	f.closeErr = private
	if _, err := l.Query(t.Context(), provider.Command{SQL: "query"}, nil); !errors.Is(err, goeferr.ErrMaterialization) || !errors.Is(err, goeferr.ErrUnknownDriver) {
		t.Fatalf("missing primary or cleanup error: %v", err)
	}
	f.closeErr = nil
	f.nextErr = private
	rows, err := l.Query(t.Context(), command, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rows.Next(); err != nil {
		t.Fatal(err)
	}
	if _, err := rows.Next(); !errors.Is(err, goeferr.ErrUnknownDriver) || errors.Is(err, private) {
		t.Fatalf("late error not preserved safely: %v", err)
	}
	if err := rows.Close(t.Context()); err != nil {
		t.Fatal(err)
	}

	f.nextErr = nil
	f.sets = [][][]driver.Value{{{nil}}}
	rows, err = l.Query(t.Context(), command, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rows.Next(); !errors.Is(err, goeferr.ErrMaterialization) {
		t.Fatalf("non-nullable layout accepted null: %v", err)
	}
	if err := rows.Close(t.Context()); err != nil {
		t.Fatal(err)
	}

	f.sets = [][][]driver.Value{{}, {{int64(2)}}}
	rows, err = l.Query(t.Context(), command, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rows.Next(); !errors.Is(err, goeferr.ErrUnsupportedFeature) {
		t.Fatalf("ignored unexpected result set: %v", err)
	}
	if err := l.Release(t.Context(), provider.Reuse); !errors.Is(err, goeferr.ErrContextBusy) || f.closes != 1 {
		t.Fatalf("release did not close leaked rows and discard: %v, %d closes", err, f.closes)
	}
	if err := rows.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestCanceledStreamAndForcedPhysicalClose(t *testing.T) {
	var forceCloseCount int
	b, f := newFixture(t, Config{ClosePhysical: func(_ context.Context, native any) error {
		if _, ok := native.(*driverFixture); !ok {
			t.Errorf("unexpected native connection: %T", native)
		}
		forceCloseCount++
		return private
	}})
	l := acquire(t, b)
	f.columnNames = []string{"i"}
	f.sets = [][][]driver.Value{{{int64(1)}, {int64(2)}}}
	ctx, cancel := context.WithCancelCause(t.Context())
	defer cancel(nil)
	rows, err := l.Query(ctx, provider.Command{SQL: "query", Columns: []provider.Column{{Kind: provider.KindInteger}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rows.Next(); err != nil {
		t.Fatal(err)
	}
	cancel(private)
	if row, err := rows.Next(); row != nil || !errors.Is(err, goeferr.ErrCanceled) || !errors.Is(err, private) {
		t.Fatalf("canceled stream produced data or lost the cause: %v, %v", row, err)
	}
	if err := rows.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := l.Release(t.Context(), provider.Discard); !errors.Is(err, goeferr.ErrUnknownDriver) ||
		errors.Is(err, private) || f.closes != 1 || forceCloseCount != 1 {
		t.Fatalf("physical close failure did not discard safely: %v, closes %d/%d", err, f.closes, forceCloseCount)
	}
}

func TestCellValueRejectsOverflowAndOutOfDomainConversions(t *testing.T) {
	for _, tc := range []struct {
		raw  any
		kind provider.Kind
	}{
		{uint64(math.MaxInt64) + 1, provider.KindInteger},
		{"1", provider.KindInteger},
		{int64(1), provider.KindFloat},
		{int64(2), provider.KindBoolean},
		{uint64(2), provider.KindBoolean},
		{"true", provider.KindBoolean},
		{int64(1), provider.KindText},
		{"bytes", provider.KindBinary},
		{"2026-11-26", provider.KindTimestamp},
	} {
		if got, ok := cellValue(tc.raw, provider.Column{Kind: tc.kind}, true); got != nil || ok {
			t.Fatalf("accepted %v as kind %v: value %v", tc.raw, tc.kind, got)
		}
	}
	if got, ok := cellValue(int64(1), provider.Column{Kind: provider.KindBoolean}, false); got != nil || ok {
		t.Fatalf("accepted integer boolean in strict mode: %v", got)
	}
	if got, ok := cellValue(uint64(1), provider.Column{Kind: provider.KindBoolean}, true); !ok || got != provider.Boolean(true) {
		t.Fatalf("unsigned tinyint bool: %v, %t", got, ok)
	}
}
