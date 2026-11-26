package model

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
)

// EntityMapping registers a Go struct and its Code First options.
// Create a registration with [Struct].
type EntityMapping struct {
	typ        reflect.Type
	name       string
	primaryKey []string
	options    []MappingOption
}

// Struct registers a struct type under an explicit logical entity name.
// primaryKey lists exact field names in key order. An empty list registers no key.
// The registration owns copies of its key and option lists.
func Struct[T any](name string, primaryKey []string, options ...MappingOption) EntityMapping {
	return EntityMapping{
		typ:        reflect.TypeFor[T](),
		name:       name,
		primaryKey: slices.Clone(primaryKey),
		options:    slices.Clone(options),
	}
}

// MappingOption supplies one fluent table or property facet.
type MappingOption struct {
	kind     mappingOptionKind
	property string
	name     string
	length   int
}

type mappingOptionKind uint8

const (
	tableOption mappingOptionKind = iota + 1
	columnOption
	maxLengthOption
)

// ToTable sets the table name for an entity. The default is the entity name.
func ToTable(name string) MappingOption {
	return MappingOption{kind: tableOption, name: name}
}

// ColumnName sets the column name for a property identified by its Go field name.
func ColumnName(property, name string) MappingOption {
	return MappingOption{kind: columnOption, property: property, name: name}
}

// MaxLength sets a positive length facet for a property identified by its Go
// field name. Interpretation and enforcement of the limit belong to the provider.
func MaxLength(property string, length int) MappingOption {
	return MappingOption{kind: maxLengthOption, property: property, length: length}
}

// MappedModel contains validated, provider-neutral Code First metadata.
// Use [Map] to build a model.
type MappedModel struct {
	entities []MappedEntity
	snapshot []byte
}

// MappedEntity describes an entity's table, properties and ordered primary key.
type MappedEntity struct {
	Name       string           `json:"name"`
	TableName  string           `json:"tableName"`
	Properties []MappedProperty `json:"properties"`
	PrimaryKey []string         `json:"primaryKey"`
}

// MappedProperty describes a Go field and its resolved column facets.
// A zero MaxLength means that no maximum length was declared.
type MappedProperty struct {
	Name       string `json:"name"`
	ColumnName string `json:"columnName"`
	Nullable   bool   `json:"nullable"`
	MaxLength  int    `json:"maxLength"`
}

// Entities returns an independent copy of the metadata. Entities are sorted by
// exact name. Properties and key members retain their respective declared orders.
func (m *MappedModel) Entities() []MappedEntity {
	entities := slices.Clone(m.entities)
	for i := range entities {
		entities[i].Properties = slices.Clone(entities[i].Properties)
		entities[i].PrimaryKey = slices.Clone(entities[i].PrimaryKey)
	}
	return entities
}

// Snapshot returns an independent copy of the canonical version 1 JSON mapping.
// It describes table and column names, maximum lengths, direct field
// nullability, and primary keys, without encoding Go or database types.
func (m *MappedModel) Snapshot() []byte {
	return slices.Clone(m.snapshot)
}

// Map builds Code First metadata from explicitly registered structs. Direct
// exported fields become properties. Pointer fields are nullable. Embedded
// fields are unsupported. Field order and explicit primary-key order are kept.
//
// Names are exact and case-sensitive. Table and column names default to their
// entity and field names. Property tags of the form
// `goef:"column=name,maxLength=60"` override conventions. Each field may
// have one goef key in a well-formed Go struct tag. Fluent options
// override individual tag facets. Contradictions at one precedence are errors.
//
// Map publishes no metadata if a declaration is invalid. A [MappingError]
// reports original struct field positions. A [ValidationError] reports
// resolved property positions, which count only mapped fields. Map performs no I/O.
func Map(registrations ...EntityMapping) (*MappedModel, error) {
	declaration := Declaration{Entities: make([]EntityDecl, len(registrations))}
	entities := make([]MappedEntity, len(registrations))
	for i, registration := range registrations {
		entity, err := mapEntity(i+1, registration)
		if err != nil {
			return nil, err
		}
		declaration.Entities[i] = entity.logical()
		entities[i] = entity
	}
	if err := Validate(declaration); err != nil {
		return nil, err
	}
	tables := make(map[string]int, len(entities))
	for i, entity := range entities {
		if previous := tables[entity.TableName]; previous != 0 {
			return nil, invalidMapping(MappingPosition{Entity: i + 1}, MappingPosition{Entity: previous}, "duplicate effective table name")
		}
		tables[entity.TableName] = i + 1
	}

	slices.SortFunc(entities, func(a, b MappedEntity) int { return strings.Compare(a.Name, b.Name) })
	snapshot, err := json.Marshal(struct {
		Version  int            `json:"version"`
		Entities []MappedEntity `json:"entities"`
	}{Version: 1, Entities: entities})
	if err != nil {
		return nil, err
	}
	return &MappedModel{entities: entities, snapshot: snapshot}, nil
}

func (e MappedEntity) logical() EntityDecl {
	properties := make([]PropertyDecl, len(e.Properties))
	for i, property := range e.Properties {
		properties[i] = PropertyDecl{Name: property.Name, Nullable: property.Nullable}
	}
	return EntityDecl{Name: e.Name, Properties: properties, PrimaryKey: e.PrimaryKey}
}
