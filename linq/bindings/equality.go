// Package bindings holds typed data emitted by the GoLINQ source generator.
package bindings

// EqualityShape identifies one direct row field, one invocation parameter and
// one projected row field by their Go declaration names.
type EqualityShape struct {
	Field      string
	Parameter  string
	Projection string
}

type scalar interface {
	~bool | ~int | ~int8 | ~int16 | ~int32 | ~int64 | ~string
}

// Equality holds a generated equality-and-projection shape and its argument.
// It has no execution methods. A zero Equality has no projection.
type Equality[Row any, Parameter, Result scalar] struct {
	shape   EqualityShape
	value   Parameter
	project func(Row) Result
}

// BindEquality constructs an invocation of the given shape. The source
// generator supplies a direct field accessor as project. A caller that
// supplies its own accessor owns its behavior and concurrency guarantees.
func BindEquality[Row any, Parameter, Result scalar](shape EqualityShape, value Parameter, project func(Row) Result) Equality[Row, Parameter, Result] {
	return Equality[Row, Parameter, Result]{shape: shape, value: value, project: project}
}

// Shape returns a copy of the declaration names, independent of the argument.
func (b Equality[Row, Parameter, Result]) Shape() EqualityShape {
	return b.shape
}

// Value returns the argument captured by this invocation.
func (b Equality[Row, Parameter, Result]) Value() Parameter {
	return b.value
}

// Project reads the projection from a row the caller already owns. It panics
// if the binding was constructed without an accessor.
func (b Equality[Row, Parameter, Result]) Project(row Row) Result {
	return b.project(row)
}
