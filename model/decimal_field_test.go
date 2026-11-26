package model_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/igorpooh1978/GoEF/goeferr"
	"github.com/igorpooh1978/GoEF/model"
)

type decimalFieldFixture struct {
	ID       int64
	Amount   model.Decimal `goef:"column=amount"`
	Optional *model.Decimal
}

func TestDecimalFieldsMODEL_TYPE_001S05(t *testing.T) {
	t.Parallel()
	mapped, err := model.Map(model.Struct[decimalFieldFixture]("Invoice", []string{"ID"},
		model.DecimalPrecision("Amount", 38, 4),
		model.DecimalPrecision("Optional", 38, 0),
		model.DecimalPrecision("Amount", 38, 4), // Repeating the same facet is harmless.
	))
	if err != nil {
		t.Fatal(err)
	}
	const fixedFacet = `"decimal":{"precision":38,"scale":4}`
	const integerFacet = `"decimal":{"precision":38,"scale":0}`
	logical := string(mapped.Snapshot())
	if !strings.Contains(logical, fixedFacet) || !strings.Contains(logical, integerFacet) {
		t.Fatalf("logical snapshot lacks both decimal facets: %s", logical)
	}

	for _, tc := range []struct {
		provider           model.TypeProvider
		required, optional string
	}{
		{model.TypePostgreSQL, "numeric(38,4)", "numeric(38,0)"},
		{model.TypeSQLite, "TEXT", "TEXT"},
		{model.TypeSQLServer, "decimal(38,4)", "decimal(38,0)"},
		{model.TypeMySQL, "decimal(38,4)", "decimal(38,0)"},
	} {
		t.Run(string(tc.provider), func(t *testing.T) {
			physical, err := model.BindTypes(mapped, tc.provider)
			if err != nil {
				t.Fatal(err)
			}
			props := physical.Entities()[0].Properties
			if codec, ok := props[0].DecimalBinding(); ok || codec.StoreType() != "" {
				t.Fatal("integer field exposed a decimal codec")
			}
			for i, field := range props[1:] {
				want := []struct {
					column, storeType string
					nullable          bool
					facets            model.DecimalFacets
				}{
					{"amount", tc.required, false, model.DecimalFacets{Precision: 38, Scale: 4}},
					{"Optional", tc.optional, true, model.DecimalFacets{Precision: 38}},
				}[i]
				codec, ok := field.DecimalBinding()
				if !ok || field.ColumnName != want.column || field.Kind != model.ScalarDecimal ||
					field.StoreType != want.storeType || codec.StoreType() != want.storeType ||
					field.Nullable != want.nullable || field.Decimal != want.facets {
					t.Fatalf("field %d: %v, codec found = %t", i, field, ok)
				}
			}
			codec, _ := props[1].DecimalBinding()
			const want = "9007199254740993.1200"
			value, err := codec.Decode(want)
			if err != nil {
				t.Fatal(err)
			}
			got, err := codec.Encode(value)
			if err != nil || got != want {
				t.Fatalf("encode = %q, %v, want %q", got, err, want)
			}
			physicalSnapshot := string(physical.Snapshot())
			if !strings.Contains(physicalSnapshot, fixedFacet) || !strings.Contains(physicalSnapshot, integerFacet) {
				t.Fatalf("physical snapshot lacks both decimal facets: %s", physicalSnapshot)
			}
			props[1].Decimal.Scale = 0
			if props := physical.Entities()[0].Properties; props[1].Decimal.Scale != 4 || string(physical.Snapshot()) != physicalSnapshot {
				t.Fatal("changing returned property metadata mutated the physical model")
			}
		})
	}
	logicalProps := mapped.Entities()[0].Properties
	if logicalProps[1].Decimal != (model.DecimalFacets{Precision: 38, Scale: 4}) {
		t.Fatalf("unexpected logical decimal metadata: %+v", logicalProps[1])
	}
	logicalProps[1].Decimal.Precision = 9
	if mapped.Entities()[0].Properties[1].Decimal.Precision != 38 || string(mapped.Snapshot()) != logical {
		t.Fatal("changing returned property metadata mutated the logical model")
	}
}

func TestDecimalFieldsMODEL_TYPE_001S06(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name            string
		options         []model.MappingOption
		field, previous int
	}{
		{"missing field", []model.MappingOption{model.DecimalPrecision("secret_field", 38, 4)}, 0, 0},
		{"zero precision", []model.MappingOption{model.DecimalPrecision("Amount", 0, 0)}, 2, 0},
		{"precision above model maximum", []model.MappingOption{model.DecimalPrecision("Amount", model.MaxDecimalDigits+1, 0)}, 2, 0},
		{"negative scale", []model.MappingOption{model.DecimalPrecision("Amount", 38, -1)}, 2, 0},
		{"scale above precision", []model.MappingOption{model.DecimalPrecision("Amount", 9, 10)}, 2, 0},
		{"contradiction", []model.MappingOption{model.DecimalPrecision("Amount", 38, 4), model.DecimalPrecision("Amount", 38, 5)}, 2, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mapped, err := model.Map(model.Struct[decimalFieldFixture]("SecretInvoice", nil, tc.options...))
			var detail *model.MappingError
			if mapped != nil || !errors.Is(err, goeferr.ErrInvalidModel) || !errors.As(err, &detail) {
				t.Fatalf("Map = %v, %v, want no model and positioned invalid-model error", mapped, err)
			}
			if detail.Position.Entity != 1 || detail.Position.Property != tc.field ||
				detail.Position.Option != len(tc.options) || detail.Previous.Option != tc.previous ||
				strings.Contains(err.Error(), "SecretInvoice") || strings.Contains(err.Error(), "secret_field") {
				t.Fatalf("unexpected mapping diagnostic: %+v", detail)
			}
		})
	}

	type definedDecimal model.Decimal
	type namedPointer *model.Decimal
	for _, tc := range []struct {
		name         string
		registration model.EntityMapping
		provider     model.TypeProvider
	}{
		{"missing facets", model.Struct[struct{ SecretValue model.Decimal }]("SecretInvoice", nil), model.TypePostgreSQL},
		{"precision on another scalar", model.Struct[struct{ SecretValue int }]("SecretInvoice", nil, model.DecimalPrecision("SecretValue", 9, 4)), model.TypeSQLite},
		{"precision on another defined type", model.Struct[struct{ SecretValue definedDecimal }]("SecretInvoice", nil, model.DecimalPrecision("SecretValue", 9, 4)), model.TypeSQLite},
		{"precision on a named pointer", model.Struct[struct{ SecretValue namedPointer }]("SecretInvoice", nil, model.DecimalPrecision("SecretValue", 9, 4)), model.TypeSQLite},
		{"precision on a nested pointer", model.Struct[struct{ SecretValue **model.Decimal }]("SecretInvoice", nil, model.DecimalPrecision("SecretValue", 9, 4)), model.TypeSQLite},
		{"length on a decimal", model.Struct[struct{ SecretValue model.Decimal }]("SecretInvoice", nil,
			model.DecimalPrecision("SecretValue", 9, 4), model.MaxLength("SecretValue", 9)), model.TypeSQLite},
		{"SQL Server precision limit", model.Struct[struct{ SecretValue model.Decimal }]("SecretInvoice", nil,
			model.DecimalPrecision("SecretValue", 39, 4)), model.TypeSQLServer},
		{"MySQL scale limit", model.Struct[struct{ SecretValue *model.Decimal }]("SecretInvoice", nil,
			model.DecimalPrecision("SecretValue", 38, 31)), model.TypeMySQL},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mapped, err := model.Map(tc.registration)
			if err != nil {
				t.Fatal(err)
			}
			physical, err := model.BindTypes(mapped, tc.provider)
			var detail *model.TypeBindingError
			if physical != nil || !errors.Is(err, goeferr.ErrInvalidModel) || !errors.As(err, &detail) {
				t.Fatalf("BindTypes = %v, %v, want no model and positioned invalid-model error", physical, err)
			}
			if detail.Provider != tc.provider || detail.EntityPosition != 1 || detail.PropertyPosition != 1 ||
				strings.Contains(err.Error(), "SecretInvoice") || strings.Contains(err.Error(), "SecretValue") {
				t.Fatalf("unexpected binding diagnostic: %+v", detail)
			}
		})
	}
	mapped, err := model.Map(model.Struct[struct{ Amount model.Decimal }]("Invoice", nil,
		model.DecimalPrecision("Amount", 5, 2),
	))
	if err != nil {
		t.Fatal(err)
	}
	physical, err := model.BindTypes(mapped, model.TypeSQLite)
	if err != nil {
		t.Fatal(err)
	}
	codec, ok := physical.Entities()[0].Properties[0].DecimalBinding()
	if !ok {
		t.Fatal("missing decimal codec")
	}
	for _, text := range []string{"1.001", "1000"} {
		value, err := model.ParseDecimal(text)
		if err != nil {
			t.Fatal(err)
		}
		if encoded, err := codec.Encode(value); err == nil || encoded != "" {
			t.Fatalf("out-of-range or lossy value %s encoded as %q", text, encoded)
		}
	}
}

func TestDecimalFieldSnapshotsRetainVersionOneScalarBytes(t *testing.T) {
	t.Parallel()
	mapped, err := model.Map(model.Struct[struct{ ID int64 }]("Invoice", []string{"ID"}))
	if err != nil {
		t.Fatal(err)
	}
	physical, err := model.BindTypes(mapped, model.TypeSQLite)
	if err != nil {
		t.Fatal(err)
	}
	const logical = `{"version":1,"entities":[{"name":"Invoice","tableName":"Invoice","properties":[{"name":"ID","columnName":"ID","nullable":false,"maxLength":0}],"primaryKey":["ID"]}]}`
	const bound = `{"version":1,"provider":"sqlite","entities":[{"name":"Invoice","tableName":"Invoice","properties":[{"name":"ID","columnName":"ID","storeType":"INTEGER","kind":"int64","nullable":false,"maxLength":0}],"primaryKey":["ID"]}]}`
	if string(mapped.Snapshot()) != logical || string(physical.Snapshot()) != bound {
		t.Fatalf("version 1 snapshots changed: %s, %s", mapped.Snapshot(), physical.Snapshot())
	}
}

func ExampleDecimalPrecision() {
	type Invoice struct {
		Total model.Decimal
	}
	mapped, err := model.Map(model.Struct[Invoice]("Invoice", nil, model.DecimalPrecision("Total", 24, 4)))
	if err != nil {
		panic(err)
	}
	physical, err := model.BindTypes(mapped, model.TypePostgreSQL)
	if err != nil {
		panic(err)
	}
	field := physical.Entities()[0].Properties[0]
	codec, ok := field.DecimalBinding()
	if !ok {
		panic("expected a decimal binding")
	}
	total, err := model.ParseDecimal("9007199254740993.12")
	if err != nil {
		panic(err)
	}
	text, err := codec.Encode(total)
	if err != nil {
		panic(err)
	}
	fmt.Println(field.StoreType, text)
	// Output: numeric(24,4) 9007199254740993.1200
}
