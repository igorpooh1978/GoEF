package fixture_test

import (
	"errors"
	"slices"
	"testing"

	f "example.invalid/goef/linq-compile"
)

type order struct{}
type orderID int
type amount int64

type recorder struct{ plans []f.Plan }

func (r *recorder) Compile(p f.Plan) error {
	r.plans = append(r.plans, p)
	return nil
}

var _ f.Compiler = (*recorder)(nil)

type compilable interface{ Compile(f.Compiler) error }

func TestProjectedBranches(t *testing.T) {
	root := f.Set[order]()
	total := root.Alias().Field[amount]("total")
	id := root.Alias().Field[orderID]("id")
	// The common prefix and each branch remain independently compilable.
	prefix := root.Where(f.Equal(total, f.Slot[amount]("one"))).
		Where(f.Equal(total, f.Slot[amount]("two"))).
		Where(f.Equal(total, f.Slot[amount]("three")))
	leftFilter := prefix.Where(f.Equal(total, f.Slot[amount]("left")))
	rightFilter := prefix.Where(f.Equal(total, f.Slot[amount]("right")))
	var left f.Query[orderID] = leftFilter.Select(id)
	var right f.Query[amount] = rightFilter.Select(total)

	shared := []f.Step{
		{Operator: "where-equal", Field: "total", Slot: "one"},
		{Operator: "where-equal", Field: "total", Slot: "two"},
		{Operator: "where-equal", Field: "total", Slot: "three"},
	}
	leftSteps := slices.Concat(shared, []f.Step{{Operator: "where-equal", Field: "total", Slot: "left"}})
	rightSteps := slices.Concat(shared, []f.Step{{Operator: "where-equal", Field: "total", Slot: "right"}})
	for _, tc := range []struct {
		name  string
		query compilable
		want  []f.Step
	}{
		{"left", left, slices.Concat(leftSteps, []f.Step{{Operator: "select", Field: "id"}})},
		{"right", right, slices.Concat(rightSteps, []f.Step{{Operator: "select", Field: "total"}})},
		{"left_filter", leftFilter, leftSteps},
		{"right_filter", rightFilter, rightSteps},
		{"prefix", prefix, shared},
		{"root", root, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := new(recorder)
			if err := tc.query.Compile(r); err != nil {
				t.Fatal(err)
			}
			if len(r.plans) != 1 || !slices.Equal(r.plans[0].Steps, tc.want) {
				t.Fatalf("compiler received %+v, want one plan with %+v", r.plans, tc.want)
			}
			// The compiler owns its copy. Retaining or editing it cannot alter later dispatch.
			clear(r.plans[0].Steps)
			if err := tc.query.Compile(r); err != nil || len(r.plans) != 2 || !slices.Equal(r.plans[1].Steps, tc.want) {
				t.Fatalf("recompile after editing compiler input: plans %+v, error %v", r.plans, err)
			}
		})
	}
}

func TestForeignAlias(t *testing.T) {
	root, other := f.Set[order](), f.Set[order]()
	total := other.Alias().Field[amount]("total")
	// Selecting the same Go type still introduces a different row scope.
	row := root.Select(root.Alias().Field[order]("related"))
	for _, tc := range []struct {
		name            string
		query           compilable
		operator, field string
	}{
		{"filter", root.Where(f.Equal(total, f.Slot[amount]("min"))).Select(root.Alias().Field[orderID]("id")), "where-equal", "total"},
		{"projection", root.Select(total), "select", "total"},
		{"previous_row", row.Select(root.Alias().Field[amount]("total")), "select", "total"},
		{"zero_query", f.Query[order]{}, "set", ""},
		{"zero_field", root.Select(f.Field[order, amount]{}), "select", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := new(recorder)
			err := tc.query.Compile(r)
			var scopeErr *f.ScopeError
			if !errors.As(err, &scopeErr) || scopeErr.Operator != tc.operator || scopeErr.Field != tc.field || len(r.plans) != 0 {
				t.Fatalf("error %v, compiler received %d plans", err, len(r.plans))
			}
		})
	}
}
