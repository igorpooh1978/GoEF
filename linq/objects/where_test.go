package objects_test

import (
	"slices"
	"testing"

	"github.com/igorpooh1978/GoEF/linq/objects"
)

// LINQ-OBJECTS-001-S01, issue #4.
func TestWhereConsumesOnlyNecessaryPrefix(t *testing.T) {
	var starts, reads, calls, stops int
	source := func(yield func(int) bool) {
		starts++
		defer func() { stops++ }()
		for _, v := range []int{1, 2, 3, 4} {
			reads++
			if !yield(v) {
				return
			}
		}
	}
	evens := objects.Where(source, func(v int) bool {
		calls++
		return v%2 == 0
	})
	if starts != 0 || reads != 0 || calls != 0 {
		t.Fatalf("constructing Where started the source %d times, read %d values and called the predicate %d times", starts, reads, calls)
	}
	var first int
	for v := range evens {
		first = v
		break
	}
	if first != 2 || reads != 2 || calls != 2 || starts != 1 || stops != 1 {
		t.Fatalf("first=%d reads=%d predicate calls=%d starts=%d stops=%d; want 2, 2, 2, 1, 1", first, reads, calls, starts, stops)
	}
}

func TestWhereEnumeratesSourceAgain(t *testing.T) {
	values := []int{4, 1, 2, 4}
	evens := objects.Where(slices.Values(values), func(v int) bool { return v%2 == 0 })
	if got := slices.Collect(evens); !slices.Equal(got, []int{4, 2, 4}) {
		t.Fatalf("first enumeration = %v, want [4 2 4]", got)
	}
	values[0] = 3
	if got := slices.Collect(evens); !slices.Equal(got, []int{2, 4}) {
		t.Fatalf("second enumeration = %v, want [2 4]", got)
	}
}

func TestWhereEmptySource(t *testing.T) {
	for range objects.Where(slices.Values([]string(nil)), func(string) bool {
		t.Fatal("predicate called on an empty source")
		return true
	}) {
		t.Fatal("empty source yielded a value")
	}
}

func TestWhereAllRejected(t *testing.T) {
	source := slices.Values([]int{1, 3, 5})
	calls := 0
	for range objects.Where(source, func(v int) bool {
		calls++
		return v%2 == 0
	}) {
		t.Fatal("filter yielded a rejected value")
	}
	if calls != 3 {
		t.Fatalf("predicate calls = %d, want 3", calls)
	}
}

func TestWhereResumesSingleUseSource(t *testing.T) {
	next := 1
	source := func(yield func(int) bool) {
		for next <= 4 {
			value := next
			next++
			if !yield(value) {
				return
			}
		}
	}
	evens := objects.Where(source, func(v int) bool { return v%2 == 0 })
	for first := range evens {
		if first != 2 {
			t.Fatalf("first match = %d, want 2", first)
		}
		break
	}
	if got := slices.Collect(evens); !slices.Equal(got, []int{4}) {
		t.Fatalf("remaining matches = %v, want [4]", got)
	}
	if got := slices.Collect(evens); len(got) != 0 {
		t.Fatalf("exhausted source yielded %v", got)
	}
}
