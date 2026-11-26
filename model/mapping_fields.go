package model

import (
	"reflect"
	"strconv"
	"strings"
)

type fieldMapping struct {
	at       MappingPosition
	property MappedProperty
	columns  []facetDeclaration[string]
	lengths  []facetDeclaration[int]
	decimals []facetDeclaration[DecimalFacets]
}

func mapEntity(entityPosition int, registration EntityMapping) (MappedEntity, error) {
	at := MappingPosition{Entity: entityPosition}
	if registration.typ == nil || registration.typ.Kind() != reflect.Struct {
		return MappedEntity{}, invalidMapping(at, MappingPosition{}, "registration requires a struct type")
	}
	if !validMappedName(registration.name) {
		return MappedEntity{}, invalidDeclaration(entityPosition, 0, 0, "invalid entity identity")
	}

	fields, err := collectFields(registration.typ, at)
	if err != nil {
		return MappedEntity{}, err
	}
	tables, err := applyMappingOptions(at, registration, fields)
	if err != nil {
		return MappedEntity{}, err
	}
	table, err := resolveFacet("table name", tables)
	if err != nil {
		return MappedEntity{}, mappingFacetError(err, at)
	}

	entity := MappedEntity{
		Name:       registration.name,
		TableName:  table.value,
		Properties: make([]MappedProperty, len(fields)),
		PrimaryKey: append([]string{}, registration.primaryKey...),
	}
	columns := make(map[string]MappingPosition, len(fields))
	for i, field := range fields {
		facets, err := resolvePropertyFacets(field.columns, field.lengths)
		if err != nil {
			return MappedEntity{}, mappingFacetError(err, field.at)
		}
		decimal, err := resolveFacet("decimal precision and scale", field.decimals)
		if err != nil {
			return MappedEntity{}, mappingFacetError(err, field.at)
		}
		if previous, ok := columns[facets.columnName]; ok {
			return MappedEntity{}, invalidMapping(field.at, previous, "duplicate effective column name")
		}
		columns[facets.columnName] = field.at
		field.property.ColumnName = facets.columnName
		field.property.MaxLength = facets.maxLength
		field.property.Decimal = decimal.value
		entity.Properties[i] = field.property
	}
	return entity, nil
}

func collectFields(typ reflect.Type, at MappingPosition) ([]fieldMapping, error) {
	fields := make([]fieldMapping, 0, typ.NumField())
	for i := range typ.NumField() {
		field := typ.Field(i)
		position := MappingPosition{Entity: at.Entity, Property: i + 1}
		if field.Anonymous {
			return nil, invalidMapping(position, MappingPosition{}, "embedded field is unsupported")
		}
		if !field.IsExported() {
			continue
		}
		mapping := fieldMapping{
			at:       position,
			property: MappedProperty{Name: field.Name, Nullable: field.Type.Kind() == reflect.Pointer},
			columns:  []facetDeclaration[string]{{source: conventionSource, at: facetPosition{input: 1, offset: 1}, value: field.Name}},
		}
		if tag, found := field.Tag.Lookup("goef"); found {
			if err := mapping.applyTag(tag); err != nil {
				return nil, err
			}
		}
		fields = append(fields, mapping)
	}
	return fields, nil
}

func (f *fieldMapping) applyTag(tag string) error {
	offset := 1
	for item := range strings.SplitSeq(tag, ",") {
		at := f.at
		at.TagOffset = offset
		key, value, found := strings.Cut(item, "=")
		if !found {
			return invalidMapping(at, MappingPosition{}, "expected a key=value property tag")
		}
		origin := facetPosition{input: 1, offset: offset}
		switch key {
		case "column":
			if !validMappedName(value) {
				return invalidMapping(at, MappingPosition{}, "invalid column name")
			}
			f.columns = append(f.columns, facetDeclaration[string]{source: tagSource, at: origin, value: value})
		case "maxLength":
			length, err := strconv.Atoi(value)
			if err != nil || length < 1 {
				return invalidMapping(at, MappingPosition{}, "invalid maximum length")
			}
			f.lengths = append(f.lengths, facetDeclaration[int]{source: tagSource, at: origin, value: length})
		default:
			return invalidMapping(at, MappingPosition{}, "unknown property tag facet")
		}
		offset += len(item) + 1
	}
	return nil
}

func applyMappingOptions(at MappingPosition, registration EntityMapping, fields []fieldMapping) ([]facetDeclaration[string], error) {
	tables := []facetDeclaration[string]{{source: conventionSource, at: facetPosition{input: 1, offset: 1}, value: registration.name}}
	fieldByName := make(map[string]*fieldMapping, len(fields))
	for i := range fields {
		fieldByName[fields[i].property.Name] = &fields[i]
	}
	for i, option := range registration.options {
		position := MappingPosition{Entity: at.Entity, Option: i + 1}
		origin := facetPosition{input: i + 1, offset: 1}

		if option.kind == tableOption {
			if !validMappedName(option.name) {
				return nil, invalidMapping(position, MappingPosition{}, "invalid table name")
			}
			tables = append(tables, facetDeclaration[string]{source: fluentSource, at: origin, value: option.name})
			continue
		}
		if option.kind != columnOption && option.kind != maxLengthOption && option.kind != decimalPrecisionOption {
			return nil, invalidMapping(position, MappingPosition{}, "unknown mapping option")
		}
		field, found := fieldByName[option.property]
		if !found {
			return nil, invalidMapping(position, MappingPosition{}, "unknown option field")
		}
		position.Property = field.at.Property
		switch option.kind {
		case columnOption:
			if !validMappedName(option.name) {
				return nil, invalidMapping(position, MappingPosition{}, "invalid column name")
			}
			field.columns = append(field.columns, facetDeclaration[string]{source: fluentSource, at: origin, value: option.name})
		case maxLengthOption:
			if option.length < 1 {
				return nil, invalidMapping(position, MappingPosition{}, "invalid maximum length")
			}
			field.lengths = append(field.lengths, facetDeclaration[int]{source: fluentSource, at: origin, value: option.length})
		case decimalPrecisionOption:
			if !option.decimal.valid() {
				return nil, invalidMapping(position, MappingPosition{}, "invalid decimal precision or scale")
			}
			field.decimals = append(field.decimals, facetDeclaration[DecimalFacets]{source: fluentSource, at: origin, value: option.decimal})
		}
	}
	return tables, nil
}
