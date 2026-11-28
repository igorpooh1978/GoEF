package queryable_test

import (
	"context"
	"errors"
	"io"
	"reflect"
	"testing"
	"time"

	"github.com/igorpooh1978/GoEF/goeferr"
	"github.com/igorpooh1978/GoEF/linq/bindings"
	"github.com/igorpooh1978/GoEF/linq/expressions"
	"github.com/igorpooh1978/GoEF/linq/queryable"
	"github.com/igorpooh1978/GoEF/model"
	"github.com/igorpooh1978/GoEF/provider"
)

type revision int16
type label string
type payload []byte

type settings struct {
	Key      string `goef:"column=key_name"`
	Revision *revision
	Note     label
	Enabled  bool
	Score    float64
	Payload  payload
	SavedAt  time.Time
	Complex  complex64
	private  int
}

var dialect = provider.Descriptor{ID: "test-provider", Major: provider.SPIMajor}
var nativeErr = errors.New("synthetic adapter failure")

func settingsEntity(t *testing.T) model.MappedEntity {
	t.Helper()
	mapped, err := model.Map(model.Struct[settings]("Settings", []string{"Key"}))
	if err != nil {
		t.Fatal(err)
	}
	return mapped.Entities()[0]
}

type compiler struct {
	shape      queryable.EqualityRead
	descriptor provider.Descriptor
	sql        string
	err        error
}

func (c *compiler) Descriptor() provider.Descriptor { return c.descriptor }

func (c *compiler) CompileEquality(shape queryable.EqualityRead) (string, error) {
	c.shape = shape
	return c.sql, c.err
}

func newCompiler() *compiler { return &compiler{descriptor: dialect, sql: "test equality command"} }

func TestDecodeOwnedProjection(t *testing.T) {
	entity := settingsEntity(t)
	compiler := newCompiler()
	fields := []string{"Revision", "Note", "Enabled", "Score", "Payload", "SavedAt"}
	plan, err := queryable.Compile(compiler, entity, expressions.EqualField[settings, string]("Key"), fields...)
	if err != nil {
		t.Fatal(err)
	}
	want := queryable.EqualityRead{
		Table: "Settings", Predicate: queryable.ReadColumn{Name: "key_name", Column: provider.Column{Kind: provider.KindText}},
		Projection: []queryable.ReadColumn{
			{Name: "Revision", Column: provider.Column{Kind: provider.KindInteger, Nullable: true}},
			{Name: "Note", Column: provider.Column{Kind: provider.KindText}},
			{Name: "Enabled", Column: provider.Column{Kind: provider.KindBoolean}},
			{Name: "Score", Column: provider.Column{Kind: provider.KindFloat}},
			{Name: "Payload", Column: provider.Column{Kind: provider.KindBinary}},
			{Name: "SavedAt", Column: provider.Column{Kind: provider.KindTimestamp}},
		},
	}
	if !reflect.DeepEqual(compiler.shape, want) {
		t.Fatalf("got IR %#v, want %#v", compiler.shape, want)
	}
	fields[0], entity.Properties[0].ColumnName = "changed", "changed"

	stamp := time.Date(2026, 11, 26, 1, 2, 3, 4, time.UTC)
	borrowed := provider.Binary{3, 5}
	adapter := &adapter{
		rows: &rows{
			values:  [][]provider.Value{{provider.Integer(7), provider.Text("value"), provider.Boolean(true), provider.Float(1.5), borrowed, provider.Timestamp(stamp)}},
			onClose: func() { clear(borrowed) },
		},
	}
	got, found, err := plan.Fetch(t.Context(), adapter, "key")
	if err != nil || !found || got.Revision == nil || *got.Revision != 7 || got.Note != "value" ||
		!got.Enabled || got.Score != 1.5 || !got.SavedAt.Equal(stamp) || !reflect.DeepEqual(got.Payload, payload{3, 5}) || got.Key != "" {
		t.Fatalf("unexpected projection %#v, found=%t, err=%v", got, found, err)
	}
	if adapter.lease.command.SQL != compiler.sql ||
		!reflect.DeepEqual(adapter.lease.bindings, provider.Bindings{provider.Text("key")}) ||
		!reflect.DeepEqual(adapter.lease.command.Columns, []provider.Column{
			{Kind: provider.KindInteger, Nullable: true}, {Kind: provider.KindText}, {Kind: provider.KindBoolean},
			{Kind: provider.KindFloat}, {Kind: provider.KindBinary}, {Kind: provider.KindTimestamp},
		}) {
		t.Fatal("dispatch did not preserve the compiled transport layout")
	}
	if !adapter.rows.closed || adapter.lease.disposition != provider.Reuse || adapter.lease.releases != 1 {
		t.Fatal("clean terminal read did not close and release exactly once")
	}
}

func TestBindScalarParameters(t *testing.T) {
	stamp := time.Now()
	t.Run("named integer and nullable predicate", func(t *testing.T) {
		checkArgument(t, "Revision", revision(9), provider.Integer(9))
	})
	t.Run("named string", func(t *testing.T) {
		checkArgument(t, "Note", label("name"), provider.Text("name"))
	})
	t.Run("boolean", func(t *testing.T) {
		checkArgument(t, "Enabled", true, provider.Boolean(true))
	})
	t.Run("float", func(t *testing.T) {
		checkArgument(t, "Score", 1.5, provider.Float(1.5))
	})
	t.Run("bytes", func(t *testing.T) {
		checkArgument(t, "Payload", payload{5, 7}, provider.Binary{5, 7})
	})
	t.Run("nil bytes", func(t *testing.T) {
		checkArgument(t, "Payload", payload(nil), provider.Binary(nil))
	})
	t.Run("timestamp", func(t *testing.T) {
		checkArgument(t, "SavedAt", stamp, provider.Timestamp(stamp))
	})
}

func checkArgument[P expressions.Scalar](t *testing.T, field string, argument P, want provider.Value) {
	t.Helper()
	plan, err := queryable.Compile(newCompiler(), settingsEntity(t), expressions.EqualField[settings, P](field), "Revision")
	if err != nil {
		t.Fatal(err)
	}
	adapter := &adapter{rows: &rows{values: [][]provider.Value{{provider.Null{}}}}}
	got, found, err := plan.Fetch(t.Context(), adapter, argument)
	if err != nil || !found || got.Revision != nil || !reflect.DeepEqual(adapter.lease.bindings, provider.Bindings{want}) {
		t.Fatalf("bind and decode failed, found=%t, err=%v", found, err)
	}
}

func TestCompileFailures(t *testing.T) {
	entity := settingsEntity(t)
	predicate := expressions.EqualField[settings, string]("Key")
	var typedNil *compiler
	for _, tc := range []struct {
		name       string
		compiler   queryable.EqualityCompiler
		entity     model.MappedEntity
		predicate  expressions.Equality[settings, string]
		projection []string
		want       error
	}{
		{"nil compiler", nil, entity, predicate, []string{"Revision"}, goeferr.ErrQueryTranslation},
		{"typed nil compiler", typedNil, entity, predicate, []string{"Revision"}, goeferr.ErrQueryTranslation},
		{"empty table", newCompiler(), model.MappedEntity{}, predicate, []string{"Revision"}, goeferr.ErrQueryTranslation},
		{"empty projection", newCompiler(), entity, predicate, nil, goeferr.ErrQueryTranslation},
		{"zero expression", newCompiler(), entity, expressions.Equality[settings, string]{}, []string{"Revision"}, goeferr.ErrQueryTranslation},
		{"field mismatch", newCompiler(), entity, expressions.EqualField[settings, string]("Revision"), []string{"Revision"}, goeferr.ErrQueryTranslation},
		{"unknown projection", newCompiler(), entity, predicate, []string{"missing"}, goeferr.ErrQueryTranslation},
		{"duplicate projection", newCompiler(), entity, predicate, []string{"Revision", "Revision"}, goeferr.ErrQueryTranslation},
		{"unsupported scalar", newCompiler(), entity, predicate, []string{"Complex"}, goeferr.ErrQueryTranslation},
		{"unexported field", newCompiler(), entity, predicate, []string{"private"}, goeferr.ErrQueryTranslation},
		{"incompatible compiler", &compiler{}, entity, predicate, []string{"Revision"}, goeferr.ErrUnsupportedFeature},
		{"empty command", &compiler{descriptor: dialect}, entity, predicate, []string{"Revision"}, goeferr.ErrQueryTranslation},
		{"compiler error redacted", &compiler{descriptor: dialect, err: nativeErr}, entity, predicate, []string{"Revision"}, goeferr.ErrQueryTranslation},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := queryable.Compile(tc.compiler, tc.entity, tc.predicate, tc.projection...)
			if !errors.Is(err, tc.want) || errors.Is(err, nativeErr) {
				t.Fatalf("got %v, want %v without native details", err, tc.want)
			}
		})
	}

	if _, err := queryable.Compile(newCompiler(), entity, expressions.EqualField[*settings, string]("Key"), "Revision"); !errors.Is(err, goeferr.ErrQueryTranslation) {
		t.Fatal("a pointer was accepted as the projected struct")
	}
	nullableMapping := settingsEntity(t)
	nullableMapping.Properties[0].Nullable = true
	if _, err := queryable.Compile(newCompiler(), nullableMapping, predicate, "Key"); !errors.Is(err, goeferr.ErrQueryTranslation) {
		t.Fatal("a mismatched nullable mapping was accepted")
	}
	unmapped := settingsEntity(t)
	unmapped.Properties = unmapped.Properties[1:]
	if _, err := queryable.Compile(newCompiler(), unmapped, predicate, "Revision"); !errors.Is(err, goeferr.ErrQueryTranslation) {
		t.Fatal("an unmapped predicate was accepted")
	}
}

func TestGeneratedProjection(t *testing.T) {
	entity := settingsEntity(t)
	binding := bindings.BindEquality(
		bindings.EqualityShape{Field: "Key", Parameter: "key", Projection: "Note"}, "key",
		func(settings) label { panic("the client accessor must not run") },
	)
	plan, err := queryable.CompileGenerated(newCompiler(), entity, binding)
	if err != nil {
		t.Fatal(err)
	}
	got, found, err := plan.Fetch(t.Context(), &adapter{rows: &rows{values: [][]provider.Value{{provider.Text("from server")}}}}, binding.Value())
	if err != nil || !found || got != label("from server") {
		t.Fatalf("unexpected projection %q, found=%t, err=%v", got, found, err)
	}

	bad := bindings.BindEquality[settings, string, int](binding.Shape(), "key", nil)
	if _, err := queryable.CompileGenerated(newCompiler(), entity, bad); !errors.Is(err, goeferr.ErrQueryTranslation) {
		t.Fatal("a mismatched generated projection type was accepted")
	}
	if _, err := queryable.CompileGenerated(newCompiler(), entity, bindings.Equality[settings, string, label]{}); !errors.Is(err, goeferr.ErrQueryTranslation) {
		t.Fatal("a zero generated binding was accepted")
	}
}

func TestFetchErrors(t *testing.T) {
	plan, err := queryable.Compile(newCompiler(), settingsEntity(t), expressions.EqualField[settings, string]("Key"), "Revision")
	if err != nil {
		t.Fatal(err)
	}
	projection := []provider.Value{provider.Integer(1)}
	for _, tc := range []struct {
		name       string
		rows       *rows
		queryErr   error
		releaseErr error
		want       error
	}{
		{"query error", nil, nativeErr, nil, nativeErr},
		{"first read error", &rows{terminal: nativeErr}, nil, nil, nativeErr},
		{"late read error", &rows{values: [][]provider.Value{projection}, terminal: nativeErr}, nil, nil, nativeErr},
		{"duplicate", &rows{values: [][]provider.Value{projection, projection}}, nil, nil, goeferr.ErrMultipleResults},
		{"invalid row length", &rows{values: [][]provider.Value{{}}}, nil, nil, goeferr.ErrMaterialization},
		{"wrong value kind", &rows{values: [][]provider.Value{{provider.Text("private data")}}}, nil, nil, goeferr.ErrMaterialization},
		{"nil value", &rows{values: [][]provider.Value{{nil}}}, nil, nil, goeferr.ErrMaterialization},
		{"overflow", &rows{values: [][]provider.Value{{provider.Integer(32768)}}}, nil, nil, goeferr.ErrMaterialization},
		{"row close", &rows{values: [][]provider.Value{projection}, closeErr: nativeErr}, nil, nil, nativeErr},
		{"lease release", &rows{values: [][]provider.Value{projection}}, nil, nativeErr, nativeErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			adapter := &adapter{rows: tc.rows, queryErr: tc.queryErr, releaseErr: tc.releaseErr}
			got, found, err := plan.Fetch(t.Context(), adapter, "key")
			if got.Revision != nil || found || !errors.Is(err, tc.want) {
				t.Fatalf("unexpected result on failure: %#v, found=%t, error=%v", got, found, err)
			}
			wantDisposition := provider.Discard
			if tc.releaseErr != nil {
				wantDisposition = provider.Reuse
			}
			if adapter.lease.releases != 1 || adapter.lease.disposition != wantDisposition ||
				(tc.rows != nil && !tc.rows.closed) {
				t.Fatal("failed read did not finish rows and lease correctly")
			}
		})
	}

	noRows := &adapter{rows: &rows{}}
	if got, found, err := plan.Fetch(t.Context(), noRows, "key"); got.Revision != nil || found || err != nil || !noRows.rows.closed {
		t.Fatal("empty result did not complete cleanly")
	}
	invalid := &adapter{acquireErr: nativeErr}
	if _, _, err := plan.Fetch(t.Context(), invalid, "key"); !errors.Is(err, nativeErr) {
		t.Fatal("acquisition error was not preserved")
	}

	var typedNil *adapter
	if _, _, err := plan.Fetch(t.Context(), typedNil, "key"); !errors.Is(err, goeferr.ErrInvalidState) {
		t.Fatal("a typed nil backend was accepted")
	}
	if _, _, err := plan.Fetch(nil, invalid, "key"); !errors.Is(err, goeferr.ErrInvalidState) {
		t.Fatal("a nil context was accepted")
	}
	incompatible := &adapter{wrongDialect: true}
	if _, _, err := plan.Fetch(t.Context(), incompatible, "key"); !errors.Is(err, goeferr.ErrUnsupportedFeature) || incompatible.acquires != 0 {
		t.Fatal("an incompatible dialect acquired a lease")
	}
	unsupported := &adapter{queryErr: nativeErr}
	if _, _, err := plan.FetchSessionNeutral(t.Context(), unsupported, "key"); !errors.Is(err, goeferr.ErrUnsupportedFeature) ||
		errors.Is(err, nativeErr) || unsupported.lease.releases != 1 || unsupported.lease.disposition != provider.Reuse {
		t.Fatal("an unsupported lease should be returned without executing a query")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	invalid.acquires = 0
	if _, _, err := plan.Fetch(ctx, invalid, "key"); !errors.Is(err, context.Canceled) || !errors.Is(err, goeferr.ErrCanceled) || invalid.acquires != 0 {
		t.Fatal("pre-canceled fetch acquired a lease")
	}
	ctx, expire := context.WithDeadline(t.Context(), time.Time{})
	defer expire()
	if _, _, err := plan.Fetch(ctx, invalid, "key"); !errors.Is(err, context.DeadlineExceeded) || !errors.Is(err, goeferr.ErrDeadlineExceeded) || invalid.acquires != 0 {
		t.Fatal("expired fetch acquired a lease")
	}
}

type adapter struct {
	rows         *rows
	acquireErr   error
	queryErr     error
	releaseErr   error
	wrongDialect bool
	acquires     int
	lease        *lease
}

func (a *adapter) Descriptor() provider.Descriptor {
	if a.wrongDialect {
		return provider.Descriptor{ID: "different", Major: provider.SPIMajor}
	}
	return dialect
}

func (a *adapter) Acquire(context.Context) (provider.Lease, error) {
	a.acquires++
	if a.acquireErr != nil {
		return nil, a.acquireErr
	}
	a.lease = &lease{adapter: a}
	return a.lease, nil
}

type lease struct {
	adapter     *adapter
	command     provider.Command
	bindings    provider.Bindings
	disposition provider.Disposition
	releases    int
}

func (l *lease) Query(_ context.Context, command provider.Command, bindings provider.Bindings) (provider.Rows, error) {
	l.command, l.bindings = command, bindings
	if l.adapter.queryErr != nil {
		return nil, l.adapter.queryErr
	}
	return l.adapter.rows, nil
}

func (l *lease) Exec(context.Context, provider.Command, provider.Bindings) (provider.WriteResult, error) {
	panic("a read plan must not dispatch writes")
}

func (l *lease) Release(ctx context.Context, disposition provider.Disposition) error {
	if _, bounded := ctx.Deadline(); !bounded {
		panic("cleanup must be bounded")
	}
	if l.adapter.rows != nil && !l.adapter.rows.closed {
		panic("close rows before releasing the lease")
	}
	l.releases++
	l.disposition = disposition
	return l.adapter.releaseErr
}

type rows struct {
	values   [][]provider.Value
	terminal error
	closeErr error
	onClose  func()
	closed   bool
}

func (r *rows) Next() ([]provider.Value, error) {
	if len(r.values) != 0 {
		first := r.values[0]
		r.values = r.values[1:]
		return first, nil
	}
	if r.terminal != nil {
		return nil, r.terminal
	}
	return nil, io.EOF
}

func (r *rows) Close(context.Context) error {
	r.closed = true
	if r.onClose != nil {
		r.onClose()
	}
	return r.closeErr
}
