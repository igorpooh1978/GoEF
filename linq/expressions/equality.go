// Package expressions constructs typed symbolic expressions for database queries.
package expressions

import "time"

// GeneratedScalar is the scalar subset accepted by existing generated equality
// and projection bindings.
type GeneratedScalar interface {
	~bool | ~int | ~int8 | ~int16 | ~int32 | ~int64 | ~string
}

// Scalar is the set of non-null equality parameter types.
type Scalar interface {
	GeneratedScalar | ~float64 | ~[]byte | time.Time
}

// Equality describes one direct field and the type of its non-null parameter.
// The field's owner and value type are validated against the model at compilation.
type Equality[Row any, Parameter Scalar] struct {
	field string
}

// EqualField describes equality on a direct exported Go field. The name is a
// Go field name. The query compiler resolves its mapped database column.
func EqualField[Row any, Parameter Scalar](name string) Equality[Row, Parameter] {
	return Equality[Row, Parameter]{field: name}
}

// Field returns the Go field name carried by the expression.
func (e Equality[Row, Parameter]) Field() string { return e.field }
