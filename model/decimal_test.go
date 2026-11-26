package model_test

import (
	"encoding"
	"fmt"
	"strings"
	"testing"

	"github.com/igorpooh1978/GoEF/model"
)

var (
	_ encoding.TextMarshaler   = model.Decimal{}
	_ encoding.TextUnmarshaler = (*model.Decimal)(nil)
)

func TestDecimalCanonicalTextAndScale(t *testing.T) {
	for _, tc := range []struct {
		name      string
		input     string
		want      string
		precision int
		scale     int
	}{
		{"zero", "0", "0", 1, 0},
		{"signed zero", "-000", "0", 1, 0},
		{"scaled zero", "-00.000", "0.000", 3, 3},
		{"positive prefix", "+0012.300", "12.300", 5, 3},
		{"negative integer", "-00035", "-35", 2, 0},
		{"negative fraction", "-000.00450", "-0.00450", 5, 5},
		{"signed fractional unit", "+0.1", "0.1", 1, 1},
		{"exact beyond binary float", "9007199254740993.00000001", "9007199254740993.00000001", 24, 8},
		{"maximum decimal digits", strings.Repeat("9", model.MaxDecimalDigits), strings.Repeat("9", model.MaxDecimalDigits), model.MaxDecimalDigits, 0},
		{"maximum scale and input length", "-0." + strings.Repeat("9", model.MaxDecimalDigits), "-0." + strings.Repeat("9", model.MaxDecimalDigits), model.MaxDecimalDigits, model.MaxDecimalDigits},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := model.ParseDecimal(tc.input)
			if err != nil {
				t.Fatal(err)
			}
			if got.String() != tc.want || got.Precision() != tc.precision || got.Scale() != tc.scale {
				t.Fatalf("got %s with precision %d and scale %d, want %s with precision %d and scale %d", got, got.Precision(), got.Scale(), tc.want, tc.precision, tc.scale)
			}
			text, err := got.MarshalText()
			if err != nil || string(text) != tc.want {
				t.Fatalf("MarshalText = %q, %v, want %q", text, err, tc.want)
			}
			var roundTrip model.Decimal
			if err := roundTrip.UnmarshalText(text); err != nil || roundTrip != got {
				t.Fatalf("round trip = %s, %v, want %s", roundTrip, err, got)
			}
			text[0] = 'x'
			if roundTrip.String() != tc.want || got.String() != tc.want {
				t.Fatal("mutation of caller-owned encoding changed a decimal value")
			}
		})
	}
}

func TestDecimalZeroValueAndScaleIdentity(t *testing.T) {
	var zero model.Decimal
	unscaled, err := model.ParseDecimal("-000")
	if err != nil || unscaled != zero || zero.String() != "0" || zero.Precision() != 1 || zero.Scale() != 0 {
		t.Fatalf("zero value or unscaled representation mismatch: %s, %v", unscaled, err)
	}
	one, err := model.ParseDecimal("1")
	if err != nil {
		t.Fatal(err)
	}
	scaled, err := model.ParseDecimal("1.00")
	if err != nil {
		t.Fatal(err)
	}
	if one == scaled {
		t.Fatal("value equality did not preserve fractional scale")
	}
}

func TestDecimalRejectsInvalidTextWithoutChangingReceiver(t *testing.T) {
	value, err := model.ParseDecimal("12.30")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"sign only", "+"},
		{"double sign", "--1"},
		{"missing integer part", ".5"},
		{"missing fractional part", "1."},
		{"extra point", "1.2.3"},
		{"exponent", "1e9"},
		{"whitespace", " 1.2"},
		{"newline", "1.2\n"},
		{"separator", "1,20"},
		{"underscore", "10_000"},
		{"non-ascii digits", "\u0661.\u0662"},
		{"invalid encoding", "1.\xff"},
		{"float special value", "NaN"},
		{"over digit count", strings.Repeat("9", model.MaxDecimalDigits) + ".0"},
		{"over byte bound", strings.Repeat("0", model.MaxDecimalDigits+4)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parsed, err := model.ParseDecimal(tc.input)
			if err == nil || parsed != (model.Decimal{}) {
				t.Fatalf("invalid input returned %s, %v", parsed, err)
			}
			current := value
			if err := current.UnmarshalText([]byte(tc.input)); err == nil || current != value {
				t.Fatalf("invalid decode changed receiver to %s or returned %v", current, err)
			}
			if len(tc.input) >= 3 && tc.name == "extra point" && strings.Contains(err.Error(), tc.input) {
				t.Fatal("parser error included raw text")
			}
		})
	}
	var nilDecimal *model.Decimal
	if err := nilDecimal.UnmarshalText([]byte("0")); err == nil {
		t.Fatal("unmarshal into a nil receiver reported success")
	}
}

func FuzzDecimalTextRoundTrip(f *testing.F) {
	for _, input := range []string{"", "0", "-000.00200", "9007199254740993.00000001", "1e9", "1.2.3", strings.Repeat("8", model.MaxDecimalDigits)} {
		f.Add(input)
	}
	f.Fuzz(func(t *testing.T, input string) {
		value, err := model.ParseDecimal(input)
		if err != nil {
			return
		}
		text, err := value.MarshalText()
		if err != nil {
			t.Fatal(err)
		}
		var roundTrip model.Decimal
		if err := roundTrip.UnmarshalText(text); err != nil || roundTrip != value {
			t.Fatalf("round trip from canonical decimal failed: %v", err)
		}
		if value.Precision() < 1 || value.Precision() > model.MaxDecimalDigits || value.Scale() > value.Precision() {
			t.Fatalf("invalid precision or scale: %d, %d", value.Precision(), value.Scale())
		}
	})
}

func ExampleParseDecimal() {
	amount, err := model.ParseDecimal("0009007199254740993.1000")
	if err != nil {
		panic(err)
	}
	fmt.Println(amount)
	fmt.Println(amount.Precision(), amount.Scale())
	// Output:
	// 9007199254740993.1000
	// 20 4
}
