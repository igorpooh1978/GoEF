package model_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/igorpooh1978/GoEF/goeferr"
	"github.com/igorpooh1978/GoEF/model"
)

type mappedCustomer struct {
	ID       int
	Name     string  `goef:"column=tag_name,maxLength=60"`
	Region   *string `goef:"maxLength=20"`
	Code     string  `goef:"column=tag_code,maxLength=30"`
	internal int
}

func TestMapMODEL_MAP_001S02(t *testing.T) {
	t.Parallel()
	keys := []string{"Code", "ID"}
	options := []model.MappingOption{
		model.ToTable("customers"),
		model.ColumnName("Name", "customer_name"),
		model.MaxLength("Code", 40),
		model.ColumnName("Name", "customer_name"),
	}
	customer := model.Struct[mappedCustomer]("Customer", keys, options...)
	keys[0] = "internal"
	options[0] = model.ToTable("changed")
	audit := model.Struct[struct{ Event string }]("Audit", nil)

	first, err := model.Map(customer, audit)
	if err != nil {
		t.Fatal(err)
	}
	second, err := model.Map(audit, customer)
	if err != nil {
		t.Fatal(err)
	}

	want := []model.MappedEntity{
		{Name: "Audit", TableName: "Audit", Properties: []model.MappedProperty{{Name: "Event", ColumnName: "Event"}}, PrimaryKey: []string{}},
		{Name: "Customer", TableName: "customers", Properties: []model.MappedProperty{
			{Name: "ID", ColumnName: "ID"},
			{Name: "Name", ColumnName: "customer_name", MaxLength: 60},
			{Name: "Region", ColumnName: "Region", MaxLength: 20, Nullable: true},
			{Name: "Code", ColumnName: "tag_code", MaxLength: 40},
		}, PrimaryKey: []string{"Code", "ID"}},
	}
	if got := first.Entities(); !reflect.DeepEqual(got, want) {
		t.Fatalf("mapping = %#v, want %#v", got, want)
	}

	snapshot := first.Snapshot()
	if !bytes.Equal(snapshot, second.Snapshot()) {
		t.Fatal("reordered registrations changed snapshot bytes")
	}
	var versioned struct {
		Version  int
		Entities []model.MappedEntity
	}
	if err := json.Unmarshal(snapshot, &versioned); err != nil {
		t.Fatal(err)
	}
	if versioned.Version != 1 || !reflect.DeepEqual(want, versioned.Entities) {
		t.Fatalf("unexpected snapshot version or content: %s", snapshot)
	}

	copies := first.Entities()
	copies[1].Name = "Changed"
	copies[1].Properties[0].ColumnName = "Changed"
	copies[1].PrimaryKey[0] = "Changed"
	snapshot[0] = 0
	if !reflect.DeepEqual(first.Entities(), want) || !bytes.Equal(first.Snapshot(), second.Snapshot()) {
		t.Fatal("mutating returned data changed the published mapping")
	}
}

func TestMapMODEL_MAP_001S03(t *testing.T) {
	t.Parallel()
	registration := model.Struct[struct {
		ID   int
		Name string `goef:"column=synthetic_hidden_column"`
	}]("Customer", []string{"ID"}, model.ColumnName("ID", "synthetic_hidden_column"))
	got, err := model.Map(registration)
	if got != nil {
		t.Fatal("invalid mapping was published")
	}
	var detail *model.MappingError
	if !errors.Is(err, goeferr.ErrInvalidModel) || !errors.As(err, &detail) {
		t.Fatalf("missing typed invalid-model mapping error: %v", err)
	}
	if detail.Position != (model.MappingPosition{Entity: 1, Property: 2}) ||
		detail.Previous != (model.MappingPosition{Entity: 1, Property: 1}) {
		t.Fatalf("wrong duplicate column locations: %+v, %+v", detail.Position, detail.Previous)
	}
	if strings.Contains(err.Error(), "synthetic_hidden_column") {
		t.Fatalf("diagnostic exposed a raw column name: %v", err)
	}
}

func TestMapRejectsInvalidInput(t *testing.T) {
	t.Parallel()
	type customer struct {
		ID   int
		Name string
	}
	type conflictingTags struct {
		Name string `goef:"column=first,column=second"`
	}
	type malformedTag struct {
		Name string `goef:"maxLength=not-an-integer"`
	}
	type invalidColumnTag struct {
		Name string `goef:"column="`
	}
	type unknownTag struct {
		Name string `goef:"opaque=sensitive"`
	}
	type missingTagValue struct {
		Name string `goef:"maxLength"`
	}
	type embedded struct{ customer }
	var zero model.MappingOption

	cases := []struct {
		name          string
		registrations []model.EntityMapping
		at, previous  model.MappingPosition
	}{
		{"zero registration", []model.EntityMapping{{}}, model.MappingPosition{Entity: 1}, model.MappingPosition{}},
		{"non-struct registration", []model.EntityMapping{model.Struct[*customer]("Customer", nil)}, model.MappingPosition{Entity: 1}, model.MappingPosition{}},
		{"embedded field", []model.EntityMapping{model.Struct[embedded]("Customer", nil)}, model.MappingPosition{Entity: 1, Property: 1}, model.MappingPosition{}},
		{"zero option", []model.EntityMapping{model.Struct[customer]("Customer", nil, zero)}, model.MappingPosition{Entity: 1, Option: 1}, model.MappingPosition{}},
		{"unknown case-sensitive field", []model.EntityMapping{model.Struct[customer]("Customer", nil, model.ColumnName("id", "private_name"))}, model.MappingPosition{Entity: 1, Option: 1}, model.MappingPosition{}},
		{"conflicting fluent columns", []model.EntityMapping{model.Struct[customer]("Customer", nil, model.ColumnName("Name", "private_name"), model.ColumnName("Name", "other"))}, model.MappingPosition{Entity: 1, Property: 2, Option: 2}, model.MappingPosition{Entity: 1, Property: 2, Option: 1}},
		{"conflicting fluent tables", []model.EntityMapping{model.Struct[customer]("Customer", nil, model.ToTable("private_name"), model.ToTable("other"))}, model.MappingPosition{Entity: 1, Option: 2}, model.MappingPosition{Entity: 1, Option: 1}},
		{"invalid table", []model.EntityMapping{model.Struct[customer]("Customer", nil, model.ToTable("private_name\x00"))}, model.MappingPosition{Entity: 1, Option: 1}, model.MappingPosition{}},
		{"invalid column", []model.EntityMapping{model.Struct[customer]("Customer", nil, model.ColumnName("Name", "private_name\xff"))}, model.MappingPosition{Entity: 1, Property: 2, Option: 1}, model.MappingPosition{}},
		{"non-positive maximum length", []model.EntityMapping{model.Struct[customer]("Customer", nil, model.MaxLength("Name", 0))}, model.MappingPosition{Entity: 1, Property: 2, Option: 1}, model.MappingPosition{}},
		{"overridden conflicting tags", []model.EntityMapping{model.Struct[conflictingTags]("Customer", nil, model.ColumnName("Name", "private_name"))}, model.MappingPosition{Entity: 1, Property: 1, TagOffset: 14}, model.MappingPosition{Entity: 1, Property: 1, TagOffset: 1}},
		{"malformed tag value", []model.EntityMapping{model.Struct[malformedTag]("Customer", nil, model.MaxLength("Name", 3))}, model.MappingPosition{Entity: 1, Property: 1, TagOffset: 1}, model.MappingPosition{}},
		{"invalid tag column", []model.EntityMapping{model.Struct[invalidColumnTag]("Customer", nil)}, model.MappingPosition{Entity: 1, Property: 1, TagOffset: 1}, model.MappingPosition{}},
		{"unknown tag facet", []model.EntityMapping{model.Struct[unknownTag]("Customer", nil)}, model.MappingPosition{Entity: 1, Property: 1, TagOffset: 1}, model.MappingPosition{}},
		{"malformed tag option", []model.EntityMapping{model.Struct[missingTagValue]("Customer", nil)}, model.MappingPosition{Entity: 1, Property: 1, TagOffset: 1}, model.MappingPosition{}},
		{"duplicate effective table", []model.EntityMapping{model.Struct[customer]("A", nil, model.ToTable("private_name")), model.Struct[customer]("B", nil, model.ToTable("private_name"))}, model.MappingPosition{Entity: 2}, model.MappingPosition{Entity: 1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mapped, err := model.Map(tc.registrations...)
			var detail *model.MappingError
			if mapped != nil || !errors.Is(err, goeferr.ErrInvalidModel) || !errors.As(err, &detail) {
				t.Fatalf("got mapping %v and error %v, want no mapping and a typed invalid-model error", mapped, err)
			}
			if detail.Position != tc.at || detail.Previous != tc.previous {
				t.Fatalf("got locations %+v and %+v, want %+v and %+v", detail.Position, detail.Previous, tc.at, tc.previous)
			}
			if !strings.Contains(err.Error(), "mapping") || strings.Contains(err.Error(), "private_name") {
				t.Fatalf("unsafe or incomplete diagnostic: %s", err)
			}
		})
	}
}

func TestMapValidatesLogicalKeys(t *testing.T) {
	t.Parallel()
	type customer struct {
		internal string
		ID       *int
	}
	for _, tc := range []struct {
		key              string
		propertyPosition int
	}{
		{key: "ID", propertyPosition: 1},
		{key: "missing", propertyPosition: 0},
	} {
		t.Run(tc.key, func(t *testing.T) {
			mapped, err := model.Map(model.Struct[customer]("Customer", []string{tc.key}))
			var detail *model.ValidationError
			if mapped != nil || !errors.Is(err, goeferr.ErrInvalidModel) || !errors.As(err, &detail) {
				t.Fatalf("expected no mapping and typed invalid-model error, got mapping %v and error %v", mapped, err)
			}
			if detail.KeyMemberPosition != 1 || detail.PropertyPosition != tc.propertyPosition {
				t.Fatalf("wrong key member or resolved property position: %+v", detail)
			}
		})
	}
	mapped, err := model.Map(model.Struct[customer]("Customer", nil, model.MaxLength("ID", 0)))
	var detail *model.MappingError
	if mapped != nil || !errors.As(err, &detail) || detail.Position.Property != 2 {
		t.Fatalf("expected mapping diagnostic at original struct field 2, got mapping %v and error %v", mapped, err)
	}
}

func TestMapAllowsEmptyModelAndCaseSensitiveColumns(t *testing.T) {
	t.Parallel()
	mapped, err := model.Map()
	if err != nil || len(mapped.Entities()) != 0 || string(mapped.Snapshot()) != `{"version":1,"entities":[]}` {
		t.Fatalf("unexpected empty model: %v, %v", mapped, err)
	}
	_, err = model.Map(model.Struct[struct {
		Code string `goef:"column=code,column=code"`
		Name string `goef:"column=code"`
	}]("Customer", nil, model.ColumnName("Name", "Code")))
	if err != nil {
		t.Fatalf("equal declarations and case-sensitive identifiers should be accepted: %v", err)
	}
}

func TestMappingErrorFallback(t *testing.T) {
	t.Parallel()
	for _, detail := range []*model.MappingError{nil, {}} {
		if detail.Error() != "invalid model mapping" || errors.Unwrap(detail) != nil || errors.Is(detail, goeferr.ErrInvalidModel) {
			t.Fatalf("unexpected uninitialized error detail: %v", detail)
		}
	}
}

func ExampleMap() {
	type Customer struct {
		ID   int
		Name string `goef:"column=tag_name,maxLength=60"`
	}
	mapping, err := model.Map(model.Struct[Customer](
		"Customer", []string{"ID"},
		model.ToTable("customers"),
		model.ColumnName("Name", "customer_name"),
	))
	if err != nil {
		panic(err)
	}
	customer := mapping.Entities()[0]
	name := customer.Properties[1]
	fmt.Println(customer.TableName, name.ColumnName, name.MaxLength)
	// Output: customers customer_name 60
}

func FuzzMapColumnNames(f *testing.F) {
	for _, seed := range []string{"column_name", "служебный код", "a,b", " 	", "", "\x00", "\xff"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, name string) {
		var typed *model.MappingError
		mapped, err := model.Map(model.Struct[struct{ Value string }](
			"Entity", nil, model.ColumnName("Value", name),
		))
		if strings.TrimSpace(name) == "" || strings.ContainsRune(name, 0) || !utf8.ValidString(name) {
			if mapped != nil || !errors.Is(err, goeferr.ErrInvalidModel) || !errors.As(err, &typed) {
				t.Fatalf("invalid name was accepted for publication, or lacked its typed error: %q, %v", name, err)
			}
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		var snapshot struct{ Entities []model.MappedEntity }
		if err := json.Unmarshal(mapped.Snapshot(), &snapshot); err != nil {
			t.Fatal(err)
		}
		if got := snapshot.Entities[0].Properties[0].ColumnName; got != name {
			t.Fatalf("column name changed in snapshot: %q, want %q", got, name)
		}
	})
}
