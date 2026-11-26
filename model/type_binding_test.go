package model_test

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/igorpooh1978/GoEF/goeferr"
	"github.com/igorpooh1978/GoEF/model"
)

type textAlias = string

type scalarFixture struct {
	ID     int64
	Active bool
	Tiny   int8
	Small  int16
	Number int32
	Native int
	Text   *textAlias
	Label  *string `goef:"maxLength=16"`
	Bytes  []byte
}

func TestBindTypesMODEL_TYPE_001S02(t *testing.T) {
	t.Parallel()
	mapped, err := model.Map(model.Struct[scalarFixture](
		"Example", []string{"ID"}, model.ToTable("base_scalars"), model.ColumnName("Label", "display_label"),
	))
	if err != nil {
		t.Fatal(err)
	}
	types := []model.ScalarKind{
		model.ScalarInt64, model.ScalarBool, model.ScalarInt8, model.ScalarInt16,
		model.ScalarInt32, model.ScalarInt, model.ScalarString, model.ScalarString, model.ScalarBytes,
	}
	fieldNames := []string{"ID", "Active", "Tiny", "Small", "Number", "Native", "Text", "Label", "Bytes"}
	before := mapped.Snapshot()
	for _, tc := range []struct {
		provider model.TypeProvider
		sqlTypes []string
	}{
		{model.TypePostgreSQL, []string{"bigint", "boolean", "smallint", "smallint", "integer", "bigint", "text", "varchar(16)", "bytea"}},
		{model.TypeSQLite, []string{"INTEGER", "INTEGER", "INTEGER", "INTEGER", "INTEGER", "INTEGER", "TEXT", "TEXT", "BLOB"}},
		{model.TypeSQLServer, []string{"bigint", "bit", "smallint", "smallint", "int", "bigint", "nvarchar(max)", "nvarchar(16)", "varbinary(max)"}},
		{model.TypeMySQL, []string{"bigint", "boolean", "tinyint", "smallint", "int", "bigint", "longtext CHARACTER SET utf8mb4", "varchar(16) CHARACTER SET utf8mb4", "longblob"}},
	} {
		t.Run(string(tc.provider), func(t *testing.T) {
			bound, err := model.BindTypes(mapped, tc.provider)
			if err != nil {
				t.Fatal(err)
			}
			entities := bound.Entities()
			if bound.Provider() != tc.provider || len(entities) != 1 {
				t.Fatalf("provider/model = %v, %v", bound.Provider(), entities)
			}
			entity := entities[0]
			if entity.Name != "Example" || entity.TableName != "base_scalars" || !slices.Equal(entity.PrimaryKey, []string{"ID"}) {
				t.Fatalf("entity = %+v", entity)
			}
			if len(entity.Properties) != len(tc.sqlTypes) {
				t.Fatalf("properties = %+v", entity.Properties)
			}
			for index, p := range entity.Properties {
				if p.Name != fieldNames[index] {
					t.Errorf("field %d name = %q, want %q", index, p.Name, fieldNames[index])
				}
				nullable := p.Name == "Label" || p.Name == "Text"
				length := 0
				if p.Name == "Label" {
					length = 16
					if p.ColumnName != "display_label" {
						t.Errorf("label name = %q", p.ColumnName)
					}
				} else if p.ColumnName != p.Name {
					t.Errorf("default column name = %q for %q", p.ColumnName, p.Name)
				}
				if p.StoreType != tc.sqlTypes[index] || p.Kind != types[index] || p.Nullable != nullable || p.MaxLength != length {
					t.Errorf("field %d = %+v", index, p)
				}
			}
		})
	}
	if !bytes.Equal(before, mapped.Snapshot()) {
		t.Fatal("type binding changed the version 1 logical snapshot")
	}
}

func TestBindTypesMODEL_TYPE_001S03(t *testing.T) {
	t.Parallel()
	type example struct {
		ID     int64
		Secret uint64
	}
	mapped, err := model.Map(model.Struct[example]("private_entity", []string{"ID"}, model.ColumnName("Secret", "private_column")))
	if err != nil {
		t.Fatal(err)
	}
	for _, provider := range []model.TypeProvider{model.TypePostgreSQL, model.TypeSQLite, model.TypeSQLServer, model.TypeMySQL} {
		t.Run(string(provider), func(t *testing.T) {
			bound, err := model.BindTypes(mapped, provider)
			var detail *model.TypeBindingError
			if bound != nil || !errors.Is(err, goeferr.ErrInvalidModel) || !errors.As(err, &detail) {
				t.Fatalf("bound, error = %v, %v", bound, err)
			}
			if detail.Provider != provider || detail.EntityPosition != 1 || detail.PropertyPosition != 2 {
				t.Fatalf("coordinates = %+v", detail)
			}
			for _, private := range []string{"private_entity", "private_column", "Secret"} {
				if strings.Contains(err.Error(), private) {
					t.Fatalf("diagnostic contains private metadata: %q", err.Error())
				}
			}
		})
	}
}

func TestBindTypesRejectsUnsupportedDeclarations(t *testing.T) {
	t.Parallel()
	type domainID int64
	type byteValue uint8
	type namedPointer *string
	for _, tc := range []struct {
		name         string
		registration model.EntityMapping
	}{
		{"nested_pointer", model.Struct[struct{ Value **string }]("Entry", nil)},
		{"named_scalar", model.Struct[struct{ Value domainID }]("Entry", nil)},
		{"named_pointer", model.Struct[struct{ Value namedPointer }]("Entry", nil)},
		{"named_byte", model.Struct[struct{ Value []byteValue }]("Entry", nil)},
		{"other_slice", model.Struct[struct{ Value []int }]("Entry", nil)},
		{"floating_point", model.Struct[struct{ Value float64 }]("Entry", nil)},
		{"compound", model.Struct[struct{ Value struct{ Number int64 } }]("Entry", nil)},
		{"length_on_non_text", model.Struct[struct{ Value []byte }]("Entry", nil, model.MaxLength("Value", 16))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mapped, err := model.Map(tc.registration)
			if err != nil {
				t.Fatal(err)
			}
			bound, err := model.BindTypes(mapped, model.TypeSQLite)
			if bound != nil || !errors.Is(err, goeferr.ErrInvalidModel) {
				t.Fatalf("bound, error = %v, %v", bound, err)
			}
		})
	}
}

func TestBindTypesValidatesNativeLengthLimits(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		provider model.TypeProvider
		max      int
		wantType string
	}{
		{model.TypePostgreSQL, 10_485_760, "varchar(10485760)"},
		{model.TypeSQLServer, 4_000, "nvarchar(4000)"},
		{model.TypeMySQL, 16_383, "varchar(16383) CHARACTER SET utf8mb4"},
	} {
		t.Run(string(tc.provider), func(t *testing.T) {
			register := func(length int) *model.MappedModel {
				t.Helper()
				mapped, err := model.Map(model.Struct[struct{ Text string }]("Entry", nil, model.MaxLength("Text", length)))
				if err != nil {
					t.Fatal(err)
				}
				return mapped
			}
			bound, err := model.BindTypes(register(tc.max), tc.provider)
			if err != nil || bound.Entities()[0].Properties[0].StoreType != tc.wantType {
				t.Fatalf("bound, error = %v, %v", bound, err)
			}
			bound, err = model.BindTypes(register(tc.max+1), tc.provider)
			if bound != nil || !errors.Is(err, goeferr.ErrInvalidModel) {
				t.Fatalf("bound, error = %v, %v", bound, err)
			}
		})
	}
}

func TestBoundModelSnapshotsAndOwnsPhysicalMetadata(t *testing.T) {
	t.Parallel()
	type data struct {
		ID    int64
		Value *[]byte
	}
	alpha := model.Struct[data]("Alpha", []string{"ID"})
	beta := model.Struct[data]("Beta", []string{"ID"})
	bind := func(target model.TypeProvider, items ...model.EntityMapping) *model.BoundModel {
		t.Helper()
		mapped, err := model.Map(items...)
		if err != nil {
			t.Fatal(err)
		}
		bound, err := model.BindTypes(mapped, target)
		if err != nil {
			t.Fatal(err)
		}
		return bound
	}
	bound := bind(model.TypeSQLite, beta, alpha)
	snapshot := bound.Snapshot()
	if !bytes.Contains(snapshot, []byte(`"version":1,"provider":"sqlite"`)) ||
		!bytes.Equal(snapshot, bind(model.TypeSQLite, alpha, beta).Snapshot()) ||
		bytes.Equal(snapshot, bind(model.TypePostgreSQL, alpha, beta).Snapshot()) {
		t.Fatalf("invalid physical snapshot identity: %s", snapshot)
	}
	entities := bound.Entities()
	if entities[0].Name != "Alpha" || !entities[0].Properties[1].Nullable || entities[0].Properties[1].Kind != model.ScalarBytes {
		t.Fatalf("entity order or nullable binary metadata = %+v", entities)
	}
	entities[0].PrimaryKey[0] = "changed"
	entities[0].Properties[0].StoreType = "changed"
	snapshot[0] = '!'
	fresh := bound.Entities()[0]
	if fresh.PrimaryKey[0] != "ID" || fresh.Properties[0].StoreType != "INTEGER" || bound.Snapshot()[0] != '{' {
		t.Fatal("returned metadata aliases a published result")
	}
}

func TestBindTypesValidatesInputsAndAllowsEmptyMapping(t *testing.T) {
	t.Parallel()
	mapped, err := model.Map()
	if err != nil {
		t.Fatal(err)
	}
	bound, err := model.BindTypes(mapped, model.TypeSQLite)
	if err != nil || len(bound.Entities()) != 0 {
		t.Fatalf("empty bound model, error = %v, %v", bound, err)
	}
	for _, tc := range []struct {
		mapped   *model.MappedModel
		provider model.TypeProvider
	}{
		{nil, model.TypeSQLite},
		{new(model.MappedModel), model.TypeSQLite},
		{mapped, "private_target"},
	} {
		bound, err := model.BindTypes(tc.mapped, tc.provider)
		if bound != nil || !errors.Is(err, goeferr.ErrInvalidModel) || strings.Contains(err.Error(), "private_target") {
			t.Fatalf("bound, error = %v, %v", bound, err)
		}
	}
}

func TestTypeBindingErrorFallback(t *testing.T) {
	t.Parallel()
	for _, err := range []*model.TypeBindingError{nil, {}} {
		if err.Error() != "invalid model type binding" || err.Unwrap() != nil {
			t.Fatalf("unexpected uninitialized error: %v", err)
		}
	}
}

func ExampleBindTypes() {
	type Record struct {
		ID    int64
		Label *string `goef:"maxLength=16"`
	}
	mapped, err := model.Map(model.Struct[Record]("Record", []string{"ID"}))
	if err != nil {
		panic(err)
	}
	physical, err := model.BindTypes(mapped, model.TypePostgreSQL)
	if err != nil {
		panic(err)
	}
	for _, field := range physical.Entities()[0].Properties {
		fmt.Println(field.Name, field.StoreType, field.Nullable)
	}
	// Output:
	// ID bigint false
	// Label varchar(16) true
}
