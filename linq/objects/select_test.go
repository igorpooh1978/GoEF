package objects_test

import (
	"slices"
	"strconv"
	"testing"

	"github.com/igorpooh1978/GoEF/linq/objects"
)

// LINQ-OBJECTS-001-S05, issue #4.
func TestWhereSelectStopsAtFirstProjectionAndReplays(t *testing.T) {
	var starts, stops int
	var pulled, filtered, projected []int
	source := func(yield func(int) bool) {
		starts++
		defer func() { stops++ }()
		for _, n := range []int{1, 2, 3, 4} {
			pulled = append(pulled, n)
			if !yield(n) {
				return
			}
		}
	}
	evens := objects.Where(source, func(n int) bool {
		filtered = append(filtered, n)
		return n%2 == 0
	})
	labels := objects.Select(evens, func(n int) string {
		projected = append(projected, n)
		return strconv.Itoa(n) + "!"
	})
	if starts != 0 || stops != 0 || len(pulled)+len(filtered)+len(projected) != 0 {
		t.Fatalf("construction started %d times and cleaned up %d times; pulled %v, filtered %v, projected %v", starts, stops, pulled, filtered, projected)
	}

	var first string
	for label := range labels {
		first = label
		break
	}
	if first != "2!" || starts != 1 || stops != 1 {
		t.Fatalf("first projection = %q, starts = %d, cleanups = %d; want %q, 1, 1", first, starts, stops, "2!")
	}
	if !slices.Equal(pulled, []int{1, 2}) || !slices.Equal(filtered, []int{1, 2}) || !slices.Equal(projected, []int{2}) {
		t.Fatalf("first projection pulled %v, filtered %v, projected %v; want [1 2], [1 2], [2]", pulled, filtered, projected)
	}

	pulled, filtered, projected = nil, nil, nil
	if got := slices.Collect(labels); !slices.Equal(got, []string{"2!", "4!"}) {
		t.Fatalf("replayed projections = %q, want [%q %q]", got, "2!", "4!")
	}
	if starts != 2 || stops != 2 {
		t.Fatalf("after replay: starts = %d, cleanups = %d; want 2 each", starts, stops)
	}
	if !slices.Equal(pulled, []int{1, 2, 3, 4}) || !slices.Equal(filtered, []int{1, 2, 3, 4}) || !slices.Equal(projected, []int{2, 4}) {
		t.Fatalf("replay pulled %v, filtered %v, projected %v; want [1 2 3 4], [1 2 3 4], [2 4]", pulled, filtered, projected)
	}
}

func TestSelectPreservesOrderDuplicatesAndTypedNil(t *testing.T) {
	source := slices.Values([]int{3, 2, 3, 0})
	sequence := objects.Select(source, func(n int) []int {
		if n == 0 {
			return nil
		}
		return []int{n}
	})
	got := slices.Collect(sequence)
	want := [][]int{{3}, {2}, {3}, nil}
	if !slices.EqualFunc(got, want, slices.Equal[[]int]) {
		t.Fatalf("projections = %v, want %v", got, want)
	}
	if last := got[len(got)-1]; last != nil {
		t.Fatalf("last projection = %#v, want a typed nil slice", last)
	}
}

func TestSelectEmptySource(t *testing.T) {
	for range objects.Select(slices.Values([]int(nil)), func(int) string {
		t.Fatal("selector called on an empty source")
		return ""
	}) {
		t.Fatal("empty source yielded a value")
	}
}

func TestSelectRetainsSingleUseSource(t *testing.T) {
	next := 1
	source := func(yield func(int) bool) {
		for next <= 3 {
			value := next
			next++
			if !yield(value) {
				return
			}
		}
	}
	sequence := objects.Select(source, strconv.Itoa)
	for first := range sequence {
		if first != "1" {
			t.Fatalf("first projection = %q, want %q", first, "1")
		}
		break
	}
	if got := slices.Collect(sequence); !slices.Equal(got, []string{"2", "3"}) {
		t.Fatalf("remaining projections = %q, want [%q %q]", got, "2", "3")
	}
	if got := slices.Collect(sequence); len(got) != 0 {
		t.Fatalf("exhausted source yielded %q", got)
	}
}
