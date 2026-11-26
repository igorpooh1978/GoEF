package model

import (
	"errors"
	"fmt"
	"strings"
)

var (
	errUnboundDecimal = errors.New("model: decimal binding is uninitialized")
	errDecimalRange   = errors.New("model: decimal integer part exceeds the bound precision")
	errDecimalScale   = errors.New("model: decimal value cannot be represented at the bound scale without rounding")
)

// DecimalBinding specifies an exact decimal storage type and its text codec.
// Construct a binding with [BindDecimal] or obtain it from a bound decimal
// field via [BoundProperty.DecimalBinding]. The zero value has no store type
// and cannot convert values.
type DecimalBinding struct {
	storeType string
	precision int
	scale     int
}

// BindDecimal binds an explicitly requested decimal precision and scale. The
// precision must be positive and the scale between zero and precision. The
// provider limits may further restrict either facet. SQLite uses TEXT with the
// same checks in the codec. The text column does not supply decimal ordering
// or arithmetic. Invalid declarations return a *TypeBindingError.
func BindDecimal(provider TypeProvider, precision, scale int) (DecimalBinding, error) {
	return bindDecimal(provider, DecimalFacets{Precision: precision, Scale: scale}, 0, 0)
}

func bindDecimal(provider TypeProvider, facets DecimalFacets, entityPosition, propertyPosition int) (DecimalBinding, error) {
	precision, scale := facets.Precision, facets.Scale
	maxPrecision, maxScale := MaxDecimalDigits, MaxDecimalDigits
	typeName := "decimal"
	switch provider {
	case TypePostgreSQL:
		typeName = "numeric"
	case TypeSQLite:
		typeName = "TEXT"
	case TypeSQLServer:
		maxPrecision, maxScale = 38, 38
	case TypeMySQL:
		maxPrecision, maxScale = 65, 30
	default:
		return DecimalBinding{}, invalidTypeBinding("", 0, 0, "unknown decimal provider")
	}
	if !facets.valid() || precision > maxPrecision || scale > maxScale {
		return DecimalBinding{}, invalidTypeBinding(provider, entityPosition, propertyPosition, "unsupported decimal precision or scale")
	}

	storeType := typeName
	if provider != TypeSQLite {
		storeType = fmt.Sprintf("%s(%d,%d)", typeName, precision, scale)
	}
	return DecimalBinding{storeType: storeType, precision: precision, scale: scale}, nil
}

// StoreType returns the physical column type declaration.
func (b DecimalBinding) StoreType() string { return b.storeType }

// Encode returns exact decimal text at the bound scale for a SQL parameter.
// It pads missing fractional digits with zero and drops excess fractional
// zeros. A value requiring rounding or too many integer digits is rejected.
func (b DecimalBinding) Encode(value Decimal) (string, error) {
	if b.precision == 0 {
		return "", errUnboundDecimal
	}
	text := value.String()
	integer, fraction, _ := strings.Cut(text, ".")
	magnitude := strings.TrimPrefix(integer, "-")
	if magnitude != "0" && len(magnitude) > b.precision-b.scale {
		return "", errDecimalRange
	}
	if len(fraction) == b.scale {
		return text, nil
	}
	if len(fraction) > b.scale {
		if strings.TrimLeft(fraction[b.scale:], "0") != "" {
			return "", errDecimalScale
		}
		fraction = fraction[:b.scale]
	}
	if b.scale == 0 {
		return integer, nil
	}
	return integer + "." + fraction + strings.Repeat("0", b.scale-len(fraction)), nil
}

// Decode reads plain decimal text and returns the exact value at the bound
// scale. It applies the same precision and scale checks as [DecimalBinding.Encode].
func (b DecimalBinding) Decode(text string) (Decimal, error) {
	value, err := ParseDecimal(text)
	if err != nil {
		return Decimal{}, err
	}
	encoded, err := b.Encode(value)
	if err != nil || encoded == "0" {
		return Decimal{}, err
	}
	return Decimal{text: encoded}, nil
}
