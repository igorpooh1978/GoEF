// Package fixture exercises a possible GoLINQ facade. It has no execution layer.
package fixture

import (
	"fmt"
	"slices"
)

// A nonzero-sized allocation distinguishes two roots even when their Go types match.
type scope struct{ marker byte }

// Alias identifies the current row of a particular query.
type Alias[Owner any] struct{ token *scope }

// Field is a symbolic member with a declared owner and value type.
type Field[Owner, Value any] struct {
	scope *scope
	name  string
}

// Field stands in for a model-generated, typed member descriptor.
func (a Alias[Owner]) Field[Value any](name string) Field[Owner, Value] {
	return Field[Owner, Value]{scope: a.token, name: name}
}

// Slot names a typed parameter. The fixture does not bind values.
type Slot[Value comparable] string

// Predicate preserves the entity type until the facade validates the alias.
type Predicate[Owner any] struct {
	scope *scope
	step  Step
}

// Equal accepts a slot of the field's value type.
func Equal[Owner any, Value comparable](f Field[Owner, Value], s Slot[Value]) Predicate[Owner] {
	return Predicate[Owner]{f.scope, Step{Operator: "where-equal", Field: f.name, Slot: string(s)}}
}

// Step is the reduced operation observed by the fixture compiler.
type Step struct{ Operator, Field, Slot string }

// Plan owns the copy of the steps delivered to one compiler invocation.
type Plan struct{ Steps []Step }

// Compiler is the ordinary, externally implementable interface in this fixture.
type Compiler interface{ Compile(Plan) error }

// ScopeError identifies the attempted use of a field outside its query.
type ScopeError struct{ Operator, Field string }

func (e *ScopeError) Error() string {
	return fmt.Sprintf("%s: field %q is outside the query scope", e.Operator, e.Field)
}

type chain struct {
	steps []Step
	err   error
}

func (p chain) append(current, owner *scope, step Step) chain {
	if p.err != nil {
		return p
	}
	if current == nil || current != owner {
		p.err = &ScopeError{Operator: step.Operator, Field: step.Field}
		return p
	}
	p.steps = append(slices.Clip(p.steps), step)
	return p
}

// Query retains a typed row and the prior symbolic steps.
type Query[Row any] struct {
	alias Alias[Row]
	plan  chain
}

// Set opens a fresh row scope for this entity.
func Set[Row any]() Query[Row] { return Query[Row]{alias: Alias[Row]{new(scope)}} }

// Alias supplies the identity shared by branches with the same current row.
func (q Query[Row]) Alias() Alias[Row] { return q.alias }

// Where adds a symbolic predicate to a new branch.
func (q Query[Row]) Where(p Predicate[Row]) Query[Row] {
	q.plan = q.plan.append(q.alias.token, p.scope, p.step)
	return q
}

// Select infers the projected field's type and starts a fresh row scope.
func (q Query[Row]) Select[Result any](f Field[Row, Result]) Query[Result] {
	result := Set[Result]()
	result.plan = q.plan.append(q.alias.token, f.scope, Step{Operator: "select", Field: f.name})
	return result
}

// Compile returns the first binding error or dispatches an owned plan to c.
func (q Query[Row]) Compile(c Compiler) error {
	if q.plan.err != nil {
		return q.plan.err
	}
	if q.alias.token == nil {
		return &ScopeError{Operator: "set"}
	}
	return c.Compile(Plan{Steps: slices.Clone(q.plan.steps)})
}
