package objects_test

import (
	"slices"
	"testing"

	"github.com/igorpooh1978/GoEF/linq/objects"
)

// LINQ-OBJECTS-001-S06, issue #4.
func TestWhereSelectCountConsumesAllProjections(t *testing.T) {
	inputs := []int{1, 2, 4, 2, 3}
	var starts, cleanups int
	var pulled, tested, projected []int
	source := func(yield func(int) bool) {
		starts++
		defer func() { cleanups++ }()
		for _, n := range inputs {
			pulled = append(pulled, n)
			if !yield(n) {
				return
			}
		}
	}
	evens := objects.Where(source, func(n int) bool {
		tested = append(tested, n)
		return n%2 == 0
	})
	projections := objects.Select(evens, func(n int) []int {
		projected = append(projected, n)
		return []int{0}
	})
	if starts != 0 || cleanups != 0 || len(pulled)+len(tested)+len(projected) != 0 {
		t.Fatalf("construction started %d times with %d cleanups, pulls %v, predicates %v and projections %v", starts, cleanups, pulled, tested, projected)
	}

	for enumeration := 1; enumeration <= 2; enumeration++ {
		if got := objects.Count(projections); got != 3 {
			t.Fatalf("count %d = %d, want 3", enumeration, got)
		}
		if starts != enumeration || cleanups != enumeration {
			t.Fatalf("count %d started %d times and cleaned up %d times cumulatively", enumeration, starts, cleanups)
		}
		if !slices.Equal(pulled, inputs) || !slices.Equal(tested, inputs) || !slices.Equal(projected, []int{2, 4, 2}) {
			t.Fatalf("count %d pulled %v, tested %v, projected %v; want %v, %v, [2 4 2]", enumeration, pulled, tested, projected, inputs, inputs)
		}
		pulled, tested, projected = nil, nil, nil
	}
}

func TestCountEmptySource(t *testing.T) {
	if got := objects.Count(slices.Values([]int(nil))); got != 0 {
		t.Fatalf("empty count = %d, want 0", got)
	}
}

func TestCountTypedNilAndDuplicateValues(t *testing.T) {
	input := [][]int{nil, {3}, {3}, nil}
	if got := objects.Count(slices.Values(input)); got != len(input) {
		t.Fatalf("count = %d, want %d", got, len(input))
	}
}

func TestCountRetainsSingleUseSource(t *testing.T) {
	next := 0
	source := func(yield func(int) bool) {
		for next < 3 {
			n := next
			next++
			if !yield(n) {
				return
			}
		}
	}
	if got := objects.Count(source); got != 3 {
		t.Fatalf("first count = %d, want 3", got)
	}
	if got := objects.Count(source); got != 0 {
		t.Fatalf("exhausted count = %d, want 0", got)
	}
}
