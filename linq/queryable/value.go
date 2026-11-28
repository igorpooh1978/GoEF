package queryable

import (
	"reflect"
	"slices"
	"time"

	"github.com/igorpooh1978/GoEF/model"
	"github.com/igorpooh1978/GoEF/provider"
)

type mappedField struct {
	typ    reflect.Type
	index  int
	column string
	kind   provider.Kind
}

func (f mappedField) readColumn() ReadColumn {
	return ReadColumn{Name: f.column, Column: provider.Column{
		Kind: f.kind, Nullable: f.typ.Kind() == reflect.Pointer,
	}}
}

func findField(row reflect.Type, entity model.MappedEntity, name string) (mappedField, bool) {
	field, ok := row.FieldByName(name)
	if !ok || !field.IsExported() || field.Anonymous || len(field.Index) != 1 {
		return mappedField{}, false
	}
	for _, property := range entity.Properties {
		if property.Name != name {
			continue
		}
		mapped := mappedField{typ: field.Type, index: field.Index[0], column: property.ColumnName}
		nullable := field.Type.Kind() == reflect.Pointer
		if nullable != property.Nullable || (nullable && field.Type.Name() != "") || property.ColumnName == "" {
			break
		}
		mapped.kind = scalarKind(mapped.scalar())
		return mapped, mapped.kind != 0
	}
	return mappedField{}, false
}

func (f mappedField) scalar() reflect.Type {
	if f.typ.Kind() == reflect.Pointer {
		return f.typ.Elem()
	}
	return f.typ
}

func scalarKind(typ reflect.Type) provider.Kind {
	switch typ.Kind() {
	case reflect.Bool:
		return provider.KindBoolean
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return provider.KindInteger
	case reflect.String:
		return provider.KindText
	case reflect.Float64:
		return provider.KindFloat
	case reflect.Slice:
		if typ.Elem() == reflect.TypeFor[byte]() {
			return provider.KindBinary
		}
	case reflect.Struct:
		if typ == reflect.TypeFor[time.Time]() {
			return provider.KindTimestamp
		}
	}
	return 0
}

func argument(value reflect.Value, kind provider.Kind) provider.Value {
	switch kind {
	case provider.KindBoolean:
		return provider.Boolean(value.Bool())
	case provider.KindInteger:
		return provider.Integer(value.Int())
	case provider.KindText:
		return provider.Text(value.String())
	case provider.KindFloat:
		return provider.Float(value.Float())
	case provider.KindBinary:
		return provider.Binary(value.Bytes())
	case provider.KindTimestamp:
		return provider.Timestamp(value.Interface().(time.Time))
	default:
		return nil
	}
}

func (f mappedField) store(target reflect.Value, value provider.Value) bool {
	if _, null := value.(provider.Null); null {
		return f.typ.Kind() == reflect.Pointer
	}
	if f.typ.Kind() == reflect.Pointer {
		target.Set(reflect.New(f.typ.Elem()))
		target = target.Elem()
	}
	switch cell := value.(type) {
	case provider.Integer:
		if f.kind != provider.KindInteger || target.OverflowInt(int64(cell)) {
			return false
		}
		target.SetInt(int64(cell))
	case provider.Text:
		if f.kind != provider.KindText {
			return false
		}
		target.SetString(string(cell))
	case provider.Boolean:
		if f.kind != provider.KindBoolean {
			return false
		}
		target.SetBool(bool(cell))
	case provider.Float:
		if f.kind != provider.KindFloat {
			return false
		}
		target.SetFloat(float64(cell))
	case provider.Binary:
		if f.kind != provider.KindBinary {
			return false
		}
		target.SetBytes(slices.Clone([]byte(cell)))
	case provider.Timestamp:
		if f.kind != provider.KindTimestamp {
			return false
		}
		target.Set(reflect.ValueOf(time.Time(cell)))
	default:
		return false
	}
	return true
}
