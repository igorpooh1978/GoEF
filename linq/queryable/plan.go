// Package queryable compiles and executes typed database projections.
package queryable

import (
	"reflect"

	"github.com/igorpooh1978/GoEF/goeferr"
	"github.com/igorpooh1978/GoEF/linq/bindings"
	"github.com/igorpooh1978/GoEF/linq/expressions"
	"github.com/igorpooh1978/GoEF/model"
	"github.com/igorpooh1978/GoEF/provider"
)

// EqualityRead is the provider-neutral shape of a single-result read. It
// identifies mapped database columns in projection order. A compiler must bind
// Predicate to one non-null argument and limit the result to at most two rows
// so the consumer can distinguish one match from multiple matches.
type EqualityRead struct {
	Table      string
	Predicate  ReadColumn
	Projection []ReadColumn
}

// ReadColumn holds one resolved database column and its transport layout.
type ReadColumn struct {
	Name string
	provider.Column
}

// EqualityCompiler translates [EqualityRead] for one trusted provider. Its
// command uses one parameter slot and returns Projection in order. It must
// quote mapped identifiers and keep parameter values outside the SQL.
type EqualityCompiler interface {
	Descriptor() provider.Descriptor
	CompileEquality(EqualityRead) (string, error)
}

// Plan holds an immutable, reusable compiled projection and its decoder. It
// stores no parameter values or database handles. Its zero value is invalid.
type Plan[Result any, Parameter expressions.Scalar] struct {
	dialect provider.Descriptor
	command provider.Command
	decode  func([]provider.Value, *Result) bool
}

// Compile builds a detached struct projection from Code First metadata and a
// typed equality field. Projection names are direct exported Go fields of Row.
// Omitted fields retain their zero values. At least one projection is required.
// Pass metadata assembled by the application for Row.
func Compile[Row any, Parameter expressions.Scalar](compiler EqualityCompiler, entity model.MappedEntity, predicate expressions.Equality[Row, Parameter], projection ...string) (Plan[Row, Parameter], error) {
	dialect, command, fields, err := compile[Row, Parameter](compiler, entity, predicate.Field(), projection)
	if err != nil {
		return Plan[Row, Parameter]{}, err
	}
	return Plan[Row, Parameter]{dialect: dialect, command: command, decode: func(cells []provider.Value, result *Row) bool {
		row := reflect.ValueOf(result).Elem()
		for i, field := range fields {
			if !field.store(row.Field(field.index), cells[i]) {
				return false
			}
		}
		return true
	}}, nil
}

// CompileGenerated lowers the generated equality and scalar projection into
// the same shape as [Compile]. Fetch with the invocation's Value or another
// parameter of the same type. The generated local accessor is never invoked.
func CompileGenerated[Row any, Parameter, Result expressions.GeneratedScalar](compiler EqualityCompiler, entity model.MappedEntity, invocation bindings.Equality[Row, Parameter, Result]) (Plan[Result, Parameter], error) {
	shape := invocation.Shape()
	dialect, command, fields, err := compile[Row, Parameter](compiler, entity, shape.Field, []string{shape.Projection})
	if err != nil {
		return Plan[Result, Parameter]{}, err
	}
	field := fields[0]
	if field.typ != reflect.TypeFor[Result]() {
		return Plan[Result, Parameter]{}, goeferr.NewQueryTranslation()
	}
	return Plan[Result, Parameter]{dialect: dialect, command: command, decode: func(cells []provider.Value, result *Result) bool {
		return field.store(reflect.ValueOf(result).Elem(), cells[0])
	}}, nil
}

func compile[Row any, Parameter expressions.Scalar](compiler EqualityCompiler, entity model.MappedEntity, predicate string, projection []string) (provider.Descriptor, provider.Command, []mappedField, error) {
	var dialect provider.Descriptor
	var command provider.Command
	row := reflect.TypeFor[Row]()
	if isNil(compiler) || row.Kind() != reflect.Struct || entity.TableName == "" || len(projection) == 0 {
		return dialect, command, nil, goeferr.NewQueryTranslation()
	}
	dialect = compiler.Descriptor()
	if !dialect.Compatible() {
		return dialect, command, nil, goeferr.NewUnsupportedFeature()
	}

	filter, ok := findField(row, entity, predicate)
	if !ok || filter.scalar() != reflect.TypeFor[Parameter]() {
		return dialect, command, nil, goeferr.NewQueryTranslation()
	}
	ir := EqualityRead{Table: entity.TableName, Predicate: filter.readColumn(), Projection: make([]ReadColumn, len(projection))}
	fields := make([]mappedField, len(projection))
	command.Slots = []provider.Slot{{Kind: filter.kind}}
	command.Columns = make([]provider.Column, len(projection))
	seen := make(map[string]bool, len(projection))
	for i, name := range projection {
		field, ok := findField(row, entity, name)
		if !ok || seen[name] {
			return dialect, provider.Command{}, nil, goeferr.NewQueryTranslation()
		}
		seen[name] = true
		ir.Projection[i] = field.readColumn()
		fields[i] = field
		command.Columns[i] = ir.Projection[i].Column
	}
	sql, err := compiler.CompileEquality(ir)
	if err != nil || sql == "" {
		// Provider-compiler diagnostics can contain input identifiers. Expose
		// only the registered portable translation detail at this boundary.
		return dialect, provider.Command{}, nil, goeferr.NewQueryTranslation()
	}
	command.SQL = sql
	return dialect, command, fields, nil
}

func isNil(value any) bool {
	v := reflect.ValueOf(value)
	return !v.IsValid() || (v.Kind() == reflect.Pointer && v.IsNil())
}
