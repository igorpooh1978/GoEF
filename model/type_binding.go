package model

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"

	"github.com/igorpooh1978/GoEF/goeferr"
)

// TypeProvider selects the dialect for physical scalar type binding.
type TypeProvider string

const (
	TypePostgreSQL TypeProvider = "postgres"
	TypeSQLite     TypeProvider = "sqlite"
	TypeSQLServer  TypeProvider = "sqlserver"
	TypeMySQL      TypeProvider = "mysql"
)

// ScalarKind identifies the built-in Go representation before storage binding.
type ScalarKind string

const (
	ScalarBool    ScalarKind = "bool"
	ScalarInt     ScalarKind = "int"
	ScalarInt8    ScalarKind = "int8"
	ScalarInt16   ScalarKind = "int16"
	ScalarInt32   ScalarKind = "int32"
	ScalarInt64   ScalarKind = "int64"
	ScalarString  ScalarKind = "string"
	ScalarBytes   ScalarKind = "bytes"
	ScalarDecimal ScalarKind = "decimal"
)

// BoundModel contains the physical scalar bindings for one dialect.
// Use [BindTypes] to build one.
type BoundModel struct {
	provider TypeProvider
	entities []BoundEntity
	snapshot []byte
}

// BoundEntity describes a table with typed properties and an ordered primary key.
// The key members are named by their Go properties.
type BoundEntity struct {
	Name       string          `json:"name"`
	TableName  string          `json:"tableName"`
	Properties []BoundProperty `json:"properties"`
	PrimaryKey []string        `json:"primaryKey"`
}

// BoundProperty describes a physical column binding. StoreType is the dialect's
// column type declaration, including its charset qualifier where applicable.
// MaxLength and Decimal are the original column facets. Their zero values
// mean unspecified.
type BoundProperty struct {
	Name       string        `json:"name"`
	ColumnName string        `json:"columnName"`
	StoreType  string        `json:"storeType"`
	Kind       ScalarKind    `json:"kind"`
	Nullable   bool          `json:"nullable"`
	MaxLength  int           `json:"maxLength"`
	Decimal    DecimalFacets `json:"decimal,omitzero"`

	decimalBinding DecimalBinding
}

// DecimalBinding returns the text codec for a bound decimal field. For another
// kind, it returns the zero codec and false. SQL NULL is handled by the caller.
func (p BoundProperty) DecimalBinding() (DecimalBinding, bool) {
	return p.decimalBinding, p.decimalBinding.precision != 0
}

// Provider returns the dialect used for the bindings.
func (m *BoundModel) Provider() TypeProvider {
	return m.provider
}

// Entities returns an independent copy in canonical entity order.
// Struct field and key-member orders are preserved.
func (m *BoundModel) Entities() []BoundEntity {
	entities := slices.Clone(m.entities)
	for i := range entities {
		entities[i].Properties = slices.Clone(entities[i].Properties)
		entities[i].PrimaryKey = slices.Clone(entities[i].PrimaryKey)
	}
	return entities
}

// Snapshot returns an independent copy of the canonical version 1 physical
// scalar mapping JSON, including provider identity.
func (m *BoundModel) Snapshot() []byte {
	return slices.Clone(m.snapshot)
}

// TypeBindingError locates an invalid scalar binding. Positions are one-based
// in [MappedModel.Entities] order. Zero means the position is not applicable.
// Provider is empty for an unrecognized target.
type TypeBindingError struct {
	Provider         TypeProvider
	EntityPosition   int
	PropertyPosition int
	reason           string
	cause            error
}

// Error reports the binding failure without raw model names or field types.
func (e *TypeBindingError) Error() string {
	if e == nil || e.reason == "" {
		return "invalid model type binding"
	}
	if e.EntityPosition == 0 {
		return "invalid model type binding: " + e.reason
	}
	return fmt.Sprintf("invalid model type binding at entity %d, property %d (%s)", e.EntityPosition, e.PropertyPosition, e.reason)
}

// Unwrap returns the portable invalid-model cause, if one was recorded.
func (e *TypeBindingError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// BindTypes resolves built-in booleans, signed integers, strings, byte slices
// and [Decimal] fields from Map to one supported dialect. A single pointer
// layer preserves declared nullability. The Go int type maps to signed 64-bit
// storage. Decimal fields require explicit [DecimalPrecision] facets.
//
// String lengths are measured in characters for PostgreSQL and MySQL, in
// UTF-16 units for SQL Server with a supplementary-character collation, and
// retained as metadata for SQLite. The latter uses TEXT even with a length
// facet. MySQL text declarations use utf8mb4.
//
// The result describes storage types and supplies codecs for decimal fields.
// Identifier quoting, DDL, schema capacity and indexability are separate
// concerns. Failures return no partial model and can be classified with
// goeferr.ErrInvalidModel.
func BindTypes(mapped *MappedModel, target TypeProvider) (*BoundModel, error) {
	profile, known := scalarProfiles[target]
	if !known {
		return nil, invalidTypeBinding("", 0, 0, "unknown provider")
	}
	if mapped == nil || mapped.sourceTypes == nil {
		return nil, invalidTypeBinding(target, 0, 0, "expected a mapped model")
	}
	entities := make([]BoundEntity, len(mapped.entities))
	for i, entity := range mapped.entities {
		properties := make([]BoundProperty, len(entity.Properties))
		typ := mapped.sourceTypes[entity.Name]
		for j, property := range entity.Properties {
			// Map derives every property directly from this struct.
			field, _ := typ.FieldByName(property.Name)
			kind := scalarKind(field.Type)
			bound := BoundProperty{
				Name: property.Name, ColumnName: property.ColumnName, Kind: kind,
				Nullable: property.Nullable, MaxLength: property.MaxLength, Decimal: property.Decimal,
			}
			if kind == ScalarDecimal {
				if property.Decimal.Precision == 0 {
					return nil, invalidTypeBinding(target, i+1, j+1, "decimal field requires explicit precision and scale")
				}
				if property.MaxLength != 0 {
					return nil, invalidTypeBinding(target, i+1, j+1, "maximum length requires a text field")
				}
				binding, err := bindDecimal(target, property.Decimal, i+1, j+1)
				if err != nil {
					return nil, err
				}
				bound.StoreType = binding.StoreType()
				bound.decimalBinding = binding
			} else {
				if property.Decimal.Precision != 0 {
					return nil, invalidTypeBinding(target, i+1, j+1, "decimal precision and scale require a decimal field")
				}
				storeType, reason := profile.storeType(kind, property.MaxLength)
				if reason != "" {
					return nil, invalidTypeBinding(target, i+1, j+1, reason)
				}
				bound.StoreType = storeType
			}
			properties[j] = bound
		}
		entities[i] = BoundEntity{
			Name: entity.Name, TableName: entity.TableName,
			Properties: properties, PrimaryKey: slices.Clone(entity.PrimaryKey),
		}
	}
	snapshot, err := json.Marshal(struct {
		Version  int           `json:"version"`
		Provider TypeProvider  `json:"provider"`
		Entities []BoundEntity `json:"entities"`
	}{Version: 1, Provider: target, Entities: entities})
	if err != nil {
		return nil, err
	}
	return &BoundModel{provider: target, entities: entities, snapshot: snapshot}, nil
}

func invalidTypeBinding(provider TypeProvider, entity, property int, reason string) *TypeBindingError {
	return &TypeBindingError{
		Provider: provider, EntityPosition: entity, PropertyPosition: property,
		reason: reason, cause: goeferr.NewInvalidModel(),
	}
}

func scalarKind(typ reflect.Type) ScalarKind {
	if typ.Kind() == reflect.Pointer {
		if typ.PkgPath() != "" {
			return ""
		}
		typ = typ.Elem()
	}
	switch typ {
	case reflect.TypeFor[bool]():
		return ScalarBool
	case reflect.TypeFor[int]():
		return ScalarInt
	case reflect.TypeFor[int8]():
		return ScalarInt8
	case reflect.TypeFor[int16]():
		return ScalarInt16
	case reflect.TypeFor[int32]():
		return ScalarInt32
	case reflect.TypeFor[int64]():
		return ScalarInt64
	case reflect.TypeFor[string]():
		return ScalarString
	case reflect.TypeFor[[]byte]():
		return ScalarBytes
	case reflect.TypeFor[Decimal]():
		return ScalarDecimal
	default:
		return ""
	}
}
