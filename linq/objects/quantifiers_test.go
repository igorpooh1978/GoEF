package objects_test

import (
	"iter"
	"slices"
	"testing"

	"github.com/igorpooh1978/GoEF/linq/objects"
)

func TestQuantifiersFiniteAndEmpty(t *testing.T) {
	for _, tc := range []struct {
		name       string
		values     []int
		any, anyBy bool
		all        bool
	}{
		{name: "empty", all: true},
		{name: "zero is an element", values: []int{0}, any: true},
		{name: "no match", values: []int{1, 3}, any: true},
		{name: "all match", values: []int{2, 4}, any: true, anyBy: true, all: true},
		{name: "mixed", values: []int{1, 2}, any: true, anyBy: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := slices.Values(tc.values)
			evenPositive := func(value int) bool {
				if len(tc.values) == 0 {
					t.Fatal("predicate called for empty input")
				}
				return value > 0 && value%2 == 0
			}
			if got := objects.Any(source); got != tc.any {
				t.Fatalf("Any = %v, want %v", got, tc.any)
			}
			if got := objects.AnyBy(source, evenPositive); got != tc.anyBy {
				t.Fatalf("AnyBy = %v, want %v", got, tc.anyBy)
			}
			if got := objects.All(source, evenPositive); got != tc.all {
				t.Fatalf("All = %v, want %v", got, tc.all)
			}
		})
	}

	source := slices.Values([][]int{nil})
	if !objects.Any(source) || !objects.AnyBy(source, func(value []int) bool { return value == nil }) ||
		!objects.All(source, func(value []int) bool { return value == nil }) {
		t.Fatal("typed nil must be treated as a yielded element")
	}
}

// An early terminal result must stop every upstream callback and complete cleanup.
func TestQuantifiersShortCircuitThroughPipeline(t *testing.T) {
	for _, tc := range []struct {
		name      string
		terminal  func(iter.Seq[int], func(int) bool) bool
		want      bool
		pulled    []int
		projected []int
		checks    []int
	}{
		{
			name:     "Any stops at the first projected element",
			terminal: func(seq iter.Seq[int], _ func(int) bool) bool { return objects.Any(seq) },
			want:     true, pulled: []int{1, 2}, projected: []int{2},
		},
		{
			name:     "AnyBy stops at the first match",
			terminal: objects.AnyBy[int],
			want:     true, pulled: []int{1, 2, 3, 4}, projected: []int{2, 4}, checks: []int{20, 40},
		},
		{
			name:     "All stops at the first failure",
			terminal: objects.All[int],
			pulled:   []int{1, 2}, projected: []int{2}, checks: []int{20},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var pulled, filtered, projected, checked []int
			var starts, cleanups int
			source := func(yield func(int) bool) {
				starts++
				defer func() { cleanups++ }()
				for value := 1; value <= 6; value++ {
					pulled = append(pulled, value)
					if !yield(value) {
						return
					}
				}
			}
			evens := objects.Where(source, func(value int) bool {
				filtered = append(filtered, value)
				return value%2 == 0
			})
			projections := objects.Select(evens, func(value int) int {
				projected = append(projected, value)
				return value * 10
			})
			if starts != 0 {
				t.Fatal("pipeline construction started the source")
			}

			for pass := 1; pass <= 2; pass++ {
				if got := tc.terminal(projections, func(value int) bool {
					checked = append(checked, value)
					return value >= 40
				}); got != tc.want {
					t.Fatalf("pass %d returned %v, want %v", pass, got, tc.want)
				}
				if starts != pass || cleanups != pass {
					t.Fatalf("pass %d: starts %d and cleanups %d", pass, starts, cleanups)
				}
				if !slices.Equal(pulled, tc.pulled) || !slices.Equal(filtered, tc.pulled) ||
					!slices.Equal(projected, tc.projected) || !slices.Equal(checked, tc.checks) {
					t.Fatalf("pass %d: pulls %v, filter %v, projection %v, checks %v", pass, pulled, filtered, projected, checked)
				}
				pulled, filtered, projected, checked = nil, nil, nil, nil
			}
		})
	}
}

func TestQuantifiersInfinitePrefix(t *testing.T) {
	for _, tc := range []struct {
		name  string
		check func(iter.Seq[int]) bool
		pulls int
		want  bool
	}{
		{name: "Any", check: objects.Any[int], pulls: 1, want: true},
		{name: "AnyBy", check: func(seq iter.Seq[int]) bool {
			return objects.AnyBy(seq, func(value int) bool { return value == 4 })
		}, pulls: 5, want: true},
		{name: "All", check: func(seq iter.Seq[int]) bool {
			return objects.All(seq, func(value int) bool { return value < 4 })
		}, pulls: 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pulls := 0
			source := func(yield func(int) bool) {
				for value := 0; ; value++ {
					pulls++
					if pulls > tc.pulls {
						t.Fatal("source read beyond the decisive element")
					}
					if !yield(value) {
						return
					}
				}
			}
			if got := tc.check(source); got != tc.want || pulls != tc.pulls {
				t.Fatalf("result %v after %d pulls, want %v after %d", got, pulls, tc.want, tc.pulls)
			}
		})
	}
}

func TestQuantifiersOneShotSource(t *testing.T) {
	values := []int{1, 2, 3, 4}
	next := 0
	source := func(yield func(int) bool) {
		for next < len(values) {
			value := values[next]
			next++
			if !yield(value) {
				return
			}
		}
	}
	if !objects.Any(source) || next != 1 {
		t.Fatalf("Any did not stop after the first item, consumed %d", next)
	}
	if objects.All(source, func(value int) bool { return value < 3 }) || next != 3 {
		t.Fatalf("All did not stop at the first failure, consumed %d", next)
	}
	if !objects.AnyBy(source, func(value int) bool { return value == 4 }) || next != 4 {
		t.Fatalf("AnyBy did not observe the remaining item, consumed %d", next)
	}
	if objects.Any(source) {
		t.Fatal("consumed source unexpectedly replayed")
	}
}
