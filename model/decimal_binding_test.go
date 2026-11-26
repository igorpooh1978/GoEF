package model_test

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/igorpooh1978/GoEF/goeferr"
	"github.com/igorpooh1978/GoEF/model"
)

func TestBindDecimalProviderDeclarations(t *testing.T) {
	for _, tc := range []struct {
		provider  model.TypeProvider
		precision int
		scale     int
		storeType string
	}{
		{model.TypePostgreSQL, 1000, 1000, "numeric(1000,1000)"},
		{model.TypeSQLite, 1000, 1000, "TEXT"},
		{model.TypeSQLServer, 38, 38, "decimal(38,38)"},
		{model.TypeMySQL, 65, 30, "decimal(65,30)"},
		{model.TypePostgreSQL, 1, 0, "numeric(1,0)"},
	} {
		t.Run(string(tc.provider)+"/"+tc.storeType, func(t *testing.T) {
			binding, err := model.BindDecimal(tc.provider, tc.precision, tc.scale)
			if err != nil || binding.StoreType() != tc.storeType {
				t.Fatalf("binding = %q, %v, want %q", binding.StoreType(), err, tc.storeType)
			}
		})
	}
}

func TestBindDecimalRejectsUnsupportedDeclarations(t *testing.T) {
	for _, tc := range []struct {
		name      string
		provider  model.TypeProvider
		precision int
		scale     int
	}{
		{"unknown provider", "untrusted-provider", 4, 2},
		{"zero precision", model.TypePostgreSQL, 0, 0},
		{"negative precision", model.TypePostgreSQL, -1, 0},
		{"huge precision", model.TypeSQLite, math.MaxInt, 0},
		{"negative scale", model.TypePostgreSQL, 5, -1},
		{"scale exceeds precision", model.TypeSQLServer, 10, 11},
		{"above PostgreSQL precision", model.TypePostgreSQL, 1001, 0},
		{"above SQLite codec precision", model.TypeSQLite, 1001, 0},
		{"above SQL Server precision", model.TypeSQLServer, 39, 0},
		{"above MySQL precision", model.TypeMySQL, 66, 0},
		{"above MySQL scale", model.TypeMySQL, 65, 31},
	} {
		t.Run(tc.name, func(t *testing.T) {
			binding, err := model.BindDecimal(tc.provider, tc.precision, tc.scale)
			var detail *model.TypeBindingError
			if binding.StoreType() != "" || !errors.Is(err, goeferr.ErrInvalidModel) || !errors.As(err, &detail) {
				t.Fatalf("invalid decimal binding = %q, %v", binding.StoreType(), err)
			}
			if strings.Contains(err.Error(), "untrusted-provider") {
				t.Fatal("invalid provider text was exposed")
			}
		})
	}
}

func TestDecimalBindingNormalizesExactValues(t *testing.T) {
	for _, provider := range []model.TypeProvider{model.TypePostgreSQL, model.TypeSQLite, model.TypeSQLServer, model.TypeMySQL} {
		t.Run(string(provider), func(t *testing.T) {
			binding, err := model.BindDecimal(provider, 24, 8)
			if err != nil {
				t.Fatal(err)
			}
			for _, tc := range []struct {
				name  string
				input string
				want  string
			}{
				{"pad whole number", "42", "42.00000000"},
				{"pad fractional value", "-0012.300", "-12.30000000"},
				{"discard excess zero digits", "0.00000001000", "0.00000001"},
				{"canonicalize zero sign", "-000.00000000", "0.00000000"},
				{"above binary floating point precision", "9007199254740993.00000001", "9007199254740993.00000001"},
				{"negative precision boundary", "-9999999999999999.99999999", "-9999999999999999.99999999"},
			} {
				t.Run(tc.name, func(t *testing.T) {
					input, err := model.ParseDecimal(tc.input)
					if err != nil {
						t.Fatal(err)
					}
					encoded, err := binding.Encode(input)
					if err != nil || encoded != tc.want {
						t.Fatalf("Encode = %q, %v, want %q", encoded, err, tc.want)
					}
					for _, text := range []string{encoded, tc.input} {
						decoded, err := binding.Decode(text)
						if err != nil || decoded.String() != tc.want || decoded.Scale() != 8 {
							t.Fatalf("Decode = %s (scale %d), %v, want %s at scale 8", decoded, decoded.Scale(), err, tc.want)
						}
					}
				})
			}
		})
	}
}

func TestDecimalBindingDropsZeroScaleAndUsesFullFractionalPrecision(t *testing.T) {
	integer, err := model.BindDecimal(model.TypePostgreSQL, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{"0.00", "-3.0", "4"} {
		decoded, err := integer.Decode(input)
		if err != nil {
			t.Fatal(err)
		}
		want, _, _ := strings.Cut(input, ".")
		encoded, err := integer.Encode(decoded)
		if err != nil || encoded != want || decoded.Scale() != 0 || decoded.String() != want {
			t.Fatalf("input %s round trip = %q, %v", input, encoded, err)
		}
		if want == "0" && decoded != (model.Decimal{}) {
			t.Fatal("unscaled zero should equal the Go zero value")
		}
	}
	binding, err := model.BindDecimal(model.TypeSQLite, model.MaxDecimalDigits, model.MaxDecimalDigits)
	if err != nil {
		t.Fatal(err)
	}
	want := "-0." + strings.Repeat("8", model.MaxDecimalDigits)
	full, err := binding.Decode(want)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := binding.Encode(full)
	if err != nil || encoded != want {
		t.Fatal("full fractional precision did not round-trip")
	}
	if value, err := binding.Decode("1"); err == nil || value != (model.Decimal{}) {
		t.Fatal("a binding with zero integer digits accepted a nonzero integer part")
	}
}

func TestDecimalBindingRejectsOverflowLossAndInvalidText(t *testing.T) {
	binding, err := model.BindDecimal(model.TypeMySQL, 5, 2)
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{"1000.00", "-1000.00", "1.001", "-0.001"} {
		amount, err := model.ParseDecimal(input)
		if err != nil {
			t.Fatal(err)
		}
		if text, err := binding.Encode(amount); err == nil || text != "" || strings.Contains(err.Error(), input) {
			t.Fatalf("out of range Encode returned %q, %v", text, err)
		}
		if value, err := binding.Decode(input); err == nil || value != (model.Decimal{}) || strings.Contains(err.Error(), input) {
			t.Fatalf("out of range Decode returned %s, %v", value, err)
		}
	}
	for _, input := range []string{"untrusted input", "0.00e1", "NaN"} {
		if value, err := binding.Decode(input); err == nil || value != (model.Decimal{}) || strings.Contains(err.Error(), input) {
			t.Fatalf("invalid Decode returned %s, %v", value, err)
		}
	}
	var zero model.DecimalBinding
	if zero.StoreType() != "" {
		t.Fatal("zero binding has a storage type")
	}
	if text, err := zero.Encode(model.Decimal{}); err == nil || text != "" {
		t.Fatalf("invalid binding Encode = %q, %v", text, err)
	}
	if value, err := zero.Decode("0"); err == nil || value != (model.Decimal{}) {
		t.Fatalf("invalid binding Decode = %s, %v", value, err)
	}
}

func ExampleBindDecimal() {
	binding, err := model.BindDecimal(model.TypePostgreSQL, 12, 4)
	if err != nil {
		panic(err)
	}
	amount, err := model.ParseDecimal("123.45")
	if err != nil {
		panic(err)
	}
	parameter, err := binding.Encode(amount)
	if err != nil {
		panic(err)
	}
	fmt.Println(binding.StoreType(), parameter)
	// Output:
	// numeric(12,4) 123.4500
}
