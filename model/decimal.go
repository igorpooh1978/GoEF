package model

import (
	"errors"
	"strings"
)

// MaxDecimalDigits is the maximum precision of a Decimal. A storage provider
// may impose a smaller precision or scale.
const MaxDecimalDigits = 1000

const maxDecimalTextBytes = MaxDecimalDigits + 3 // optional sign, point, and leading zero

var (
	errDecimalSyntax    = errors.New("model: invalid decimal text")
	errDecimalPrecision = errors.New("model: decimal text exceeds the precision or length limit")
	errNilDecimal       = errors.New("model: cannot unmarshal a decimal into a nil receiver")
)

// Decimal is an exact finite base-10 value with a preserved fractional scale.
// The zero value represents 0 at scale zero. Go equality compares both the
// value and its scale, so 1 and 1.00 are distinct.
type Decimal struct {
	text string
}

// ParseDecimal reads plain ASCII base-10 text with an optional leading sign
// and optional fractional part. Both sides of a decimal point must have
// digits. It discards a leading plus, redundant integer zeros, and the sign
// of zero while retaining all fractional digits. Input is bounded to
// MaxDecimalDigits+3 bytes and the resulting precision to MaxDecimalDigits.
func ParseDecimal(text string) (Decimal, error) {
	if len(text) > maxDecimalTextBytes {
		return Decimal{}, errDecimalPrecision
	}
	if text == "" {
		return Decimal{}, errDecimalSyntax
	}

	negative := text[0] == '-'
	if negative || text[0] == '+' {
		text = text[1:]
	}
	integer, fraction, hasPoint := strings.Cut(text, ".")
	if !decimalDigits(integer) || (hasPoint && !decimalDigits(fraction)) {
		return Decimal{}, errDecimalSyntax
	}
	integer = strings.TrimLeft(integer, "0")
	if len(integer)+len(fraction) > MaxDecimalDigits {
		return Decimal{}, errDecimalPrecision
	}
	if integer == "" {
		if !hasPoint {
			return Decimal{}, nil
		}
		integer = "0"
		if strings.TrimLeft(fraction, "0") == "" {
			negative = false
		}
	}

	var canonical strings.Builder
	canonical.Grow(len(text) + 1)
	if negative {
		canonical.WriteByte('-')
	}
	canonical.WriteString(integer)
	if hasPoint {
		canonical.WriteByte('.')
		canonical.WriteString(fraction)
	}
	return Decimal{text: canonical.String()}, nil
}

// String returns the canonical decimal text, including the preserved scale.
func (d Decimal) String() string {
	if d.text == "" {
		return "0"
	}
	return d.text
}

// Precision returns the number of digits needed by the canonical value at
// its scale. A zero integer part does not take a precision digit when the
// scale is positive. Unscaled zero has precision one.
func (d Decimal) Precision() int {
	integer, fraction, hasPoint := strings.Cut(strings.TrimPrefix(d.String(), "-"), ".")
	if hasPoint && integer == "0" {
		return len(fraction)
	}
	return len(integer) + len(fraction)
}

// Scale returns the number of fractional digits.
func (d Decimal) Scale() int {
	_, fraction, _ := strings.Cut(d.text, ".")
	return len(fraction)
}

// MarshalText returns the canonical decimal representation as caller-owned bytes.
func (d Decimal) MarshalText() ([]byte, error) {
	return []byte(d.String()), nil
}

// UnmarshalText reads a decimal using [ParseDecimal]. On failure d stays unchanged.
func (d *Decimal) UnmarshalText(text []byte) error {
	if d == nil {
		return errNilDecimal
	}
	if len(text) > maxDecimalTextBytes {
		return errDecimalPrecision
	}
	value, err := ParseDecimal(string(text))
	if err != nil {
		return err
	}
	*d = value
	return nil
}

func decimalDigits(text string) bool {
	if text == "" {
		return false
	}
	for i := range len(text) {
		if text[i] < '0' || text[i] > '9' {
			return false
		}
	}
	return true
}
