// Package shape is a compiler fixture. Its plan records only the two field IDs
// that cross the generic facade in the experiment.
package shape

// Field is a field descriptor with an entity owner and a result type.
type Field[Owner, Value any] struct{ ID int }

// Plan is the ordinary value received by the test compiler.
type Plan struct{ Filter, Projection int }

// Compiler is the complete non-generic interface used in the fixture.
type Compiler interface{ Compile(Plan) Plan }

// Context starts a typed chain and performs no I/O.
type Context struct{}

// Set starts with the caller's chosen entity type.
func (Context) Set[T any]() Query[T] { return Query[T]{} }

// Query retains an immutable plan value across chained operations.
type Query[T any] struct{ plan Plan }

// Where accepts only a boolean field belonging to the current entity.
func (q Query[T]) Where(predicate Field[T, bool]) Query[T] {
	q.plan.Filter = predicate.ID
	return q
}

// Select changes the result type while preserving the existing filter.
func (q Query[T]) Select[R any](field Field[T, R]) Query[R] {
	q.plan.Projection = field.ID
	return Query[R]{plan: q.plan}
}

// Compile dispatches to an ordinary interface method.
func (q Query[T]) Compile(c Compiler) Plan { return c.Compile(q.plan) }

// Nest represents the result type of a nesting operator without adding a
// recursively expanding method to every instantiated Query.
func Nest[T any](q Query[T]) Query[[]T] { return Query[[]T]{plan: q.plan} }
