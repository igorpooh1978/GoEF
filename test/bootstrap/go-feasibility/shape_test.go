package shape_test

import (
	"testing"

	shape "example.invalid/goef/go-feasibility"
)

type account struct{}
type accountID int
type recorder struct{ calls int }

func (r *recorder) Compile(p shape.Plan) shape.Plan {
	r.calls++
	return p
}

var _ shape.Compiler = (*recorder)(nil)

func TestFacade(t *testing.T) {
	root := shape.Context{}.Set[account]()
	active := shape.Field[account, bool]{ID: 3}
	id := shape.Field[account, accountID]{ID: 7}
	var projected shape.Query[accountID] = root.Where(active).Select(id)
	var nested shape.Query[[]accountID] = shape.Nest(projected)
	r := new(recorder)
	if got := nested.Compile(r); got != (shape.Plan{Filter: 3, Projection: 7}) {
		t.Fatalf("received plan %+v", got)
	}
	if got := root.Compile(r); got != (shape.Plan{}) || r.calls != 2 {
		t.Fatalf("root plan %+v, calls %d", got, r.calls)
	}
}
