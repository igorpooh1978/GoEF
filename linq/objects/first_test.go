package objects_test

import (
	"iter"
	"slices"
	"testing"

	"github.com/igorpooh1978/GoEF/linq/objects"
)

func TestFirstDistinguishesMissingFromPresent(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		values []int
		want   int
		found  bool
	}{
		{name: "empty"},
		{name: "present zero", values: []int{0, 9}, found: true},
		{name: "first in encounter order", values: []int{4, 1}, want: 4, found: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			value, found := objects.First(slices.Values(tc.values))
			if value != tc.want || found != tc.found {
				t.Fatalf("First(%v) = (%v, %v), want (%v, %v)", tc.values, value, found, tc.want, tc.found)
			}

			want := tc.want
			if !tc.found {
				want = -1
			}
			if got := objects.FirstOrDefault(slices.Values(tc.values), -1); got != want {
				t.Fatalf("FirstOrDefault(%v, -1) = %v, want %v", tc.values, got, want)
			}
		})
	}

	nilElement := slices.Values([][]int{nil})
	value, found := objects.First(nilElement)
	if value != nil || !found {
		t.Fatalf("First(nil element) = (%v, %v), want (nil, true)", value, found)
	}
	if got := objects.FirstOrDefault(nilElement, []int{1}); got != nil {
		t.Fatalf("FirstOrDefault(nil element) = %v, want nil", got)
	}
	if got := objects.FirstOrDefault(slices.Values([][]int(nil)), []int{1}); !slices.Equal(got, []int{1}) {
		t.Fatalf("FirstOrDefault(empty) = %v, want [1]", got)
	}
}

func TestFirstStopsAndClosesFilteredSource(t *testing.T) {
	t.Parallel()
	var reads, checks []int
	var starts, cleanups int
	source := func(yield func(int) bool) {
		starts++
		defer func() { cleanups++ }()
		for _, value := range []int{3, 5, 0, 2} {
			reads = append(reads, value)
			if !yield(value) {
				return
			}
		}
	}
	evens := objects.Where(source, func(value int) bool {
		checks = append(checks, value)
		return value%2 == 0
	})
	if starts != 0 || len(reads) != 0 || len(checks) != 0 {
		t.Fatal("constructing Where read the source or called its predicate")
	}

	value, ok := objects.First(evens)
	if value != 0 || !ok {
		t.Fatalf("First(evens) = (%d, %v), want (0, true)", value, ok)
	}
	if starts != 1 || cleanups != 1 || !slices.Equal(reads, []int{3, 5, 0}) || !slices.Equal(checks, reads) {
		t.Fatalf("First started %d and closed %d times, read %v and tested %v", starts, cleanups, reads, checks)
	}

	reads, checks = nil, nil
	if value := objects.FirstOrDefault(evens, -1); value != 0 {
		t.Fatalf("FirstOrDefault(evens, -1) = %d, want 0", value)
	}
	if starts != 2 || cleanups != 2 || !slices.Equal(reads, []int{3, 5, 0}) || !slices.Equal(checks, reads) {
		t.Fatalf("replay started %d and closed %d times, read %v and tested %v", starts, cleanups, reads, checks)
	}

	reads = nil
	never := objects.Where(source, func(int) bool { return false })
	if value := objects.FirstOrDefault(never, -1); value != -1 {
		t.Fatalf("FirstOrDefault(no match, -1) = %d, want -1", value)
	}
	if starts != 3 || cleanups != 3 || !slices.Equal(reads, []int{3, 5, 0, 2}) {
		t.Fatalf("no-match scan started %d and closed %d times, read %v", starts, cleanups, reads)
	}
}

func TestFirstPreservesSingleUseAndUnboundedInputs(t *testing.T) {
	t.Parallel()
	remaining := []int{7, 8}
	singleUse := func(yield func(int) bool) {
		for len(remaining) > 0 {
			value := remaining[0]
			remaining = remaining[1:]
			if !yield(value) {
				return
			}
		}
	}
	for _, want := range []int{7, 8, -1} {
		if got := objects.FirstOrDefault(singleUse, -1); got != want {
			t.Fatalf("FirstOrDefault(single use, -1) = %d, want %d", got, want)
		}
	}

	infinite := iter.Seq[int](func(yield func(int) bool) {
		for value := 0; ; value++ {
			if value > 3 {
				t.Fatal("read beyond the first matching value")
			}
			if !yield(value) {
				return
			}
		}
	})
	if value, found := objects.First(objects.Where(infinite, func(n int) bool { return n == 3 })); value != 3 || !found {
		t.Fatalf("First(unbounded match at 3) = (%d, %v), want (3, true)", value, found)
	}
}
