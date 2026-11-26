package objects_test

import (
	"math"
	"slices"
	"testing"

	"github.com/igorpooh1978/GoEF/linq/objects"
)

func TestSkipTakeCountBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name       string
		count      int
		skip, take []int
	}{
		{name: "minimum int", count: math.MinInt, skip: []int{7, 7, 3}},
		{name: "negative", count: -1, skip: []int{7, 7, 3}},
		{name: "zero", count: 0, skip: []int{7, 7, 3}},
		{name: "one", count: 1, skip: []int{7, 3}, take: []int{7}},
		{name: "exact", count: 3, take: []int{7, 7, 3}},
		{name: "maximum int", count: math.MaxInt, take: []int{7, 7, 3}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := slices.Values([]int{7, 7, 3})
			if got := slices.Collect(objects.Skip(source, tc.count)); !slices.Equal(got, tc.skip) {
				t.Fatalf("Skip(%d) = %v, want %v", tc.count, got, tc.skip)
			}
			if got := slices.Collect(objects.Take(source, tc.count)); !slices.Equal(got, tc.take) {
				t.Fatalf("Take(%d) = %v, want %v", tc.count, got, tc.take)
			}
		})
	}
	empty := slices.Values([]int(nil))
	if got := slices.Collect(objects.Skip(empty, 2)); len(got) > 0 {
		t.Fatalf("Skip on an empty source yielded %v", got)
	}
	if got := slices.Collect(objects.Take(empty, 2)); len(got) > 0 {
		t.Fatalf("Take on an empty source yielded %v", got)
	}

	typedNil := slices.Values([][]int{{1}, nil})
	tail := objects.Skip(typedNil, 1)
	if got := slices.Collect(objects.Take(tail, 1)); len(got) != 1 || got[0] != nil {
		t.Fatalf("the one typed-nil value should survive Skip and Take, got %v", got)
	}
}

func TestTakeNonPositiveDoesNotStartSource(t *testing.T) {
	source := func(func(int) bool) {
		t.Fatal("Take with a nonpositive count started the source")
	}
	for _, count := range []int{math.MinInt, -1, 0} {
		if result := slices.Collect(objects.Take(source, count)); len(result) > 0 {
			t.Fatalf("Take(%d) yielded %v", count, result)
		}
	}
}

func TestSkipTakeStopsAndReplays(t *testing.T) {
	var starts, cleanups, pulls int
	source := func(yield func(int) bool) {
		starts++
		defer func() { cleanups++ }()
		for value := 0; ; value++ {
			pulls++
			if value >= 5 {
				t.Fatal("source read beyond the finite window")
			}
			if !yield(value) {
				return
			}
		}
	}
	page := objects.Take(objects.Skip(source, 2), 3)
	if starts != 0 || cleanups != 0 || pulls != 0 {
		t.Fatal("window construction started the source")
	}

	if got := slices.Collect(page); !slices.Equal(got, []int{2, 3, 4}) {
		t.Fatalf("first enumeration = %v, want [2 3 4]", got)
	}
	if starts != 1 || cleanups != 1 || pulls != 5 {
		t.Fatalf("first pass: starts %d, cleanups %d, pulls %d", starts, cleanups, pulls)
	}
	var first []int
	for value := range page {
		first = append(first, value)
		break
	}
	if !slices.Equal(first, []int{2}) || starts != 2 || cleanups != 2 || pulls != 8 {
		t.Fatalf("early second pass: values %v, starts %d, cleanups %d, pulls %d", first, starts, cleanups, pulls)
	}
	if got := slices.Collect(page); !slices.Equal(got, []int{2, 3, 4}) {
		t.Fatalf("third enumeration = %v, want [2 3 4]", got)
	}
	if starts != 3 || cleanups != 3 || pulls != 13 {
		t.Fatalf("third pass: starts %d, cleanups %d, pulls %d", starts, cleanups, pulls)
	}
}

func TestSkipTakeRetainsSingleUseSource(t *testing.T) {
	next := 0
	source := func(yield func(int) bool) {
		for next < 6 {
			value := next
			next++
			if !yield(value) {
				return
			}
		}
	}
	page := objects.Take(objects.Skip(source, 1), 2)
	for _, want := range [][]int{{1, 2}, {4, 5}, nil} {
		if got := slices.Collect(page); !slices.Equal(got, want) {
			t.Fatalf("single-use source yielded %v, want %v", got, want)
		}
	}
}
