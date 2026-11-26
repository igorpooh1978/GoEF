package objects_test

import (
	"fmt"
	"iter"
	"slices"
	"testing"

	"github.com/igorpooh1978/GoEF/linq/objects"
)

func recordCompositionSource(events *[]string, name string, values ...int) iter.Seq[int] {
	return func(yield func(int) bool) {
		*events = append(*events, "start "+name)
		defer func() { *events = append(*events, "close "+name) }()
		for _, value := range values {
			*events = append(*events, fmt.Sprintf("%s %d", name, value))
			if !yield(value) {
				return
			}
		}
	}
}

func recordCompositionPrefix(sequence iter.Seq[int], events *[]string, limit int) {
	seen := 0
	for value := range sequence {
		*events = append(*events, fmt.Sprintf("yield %d", value))
		seen++
		if seen == limit {
			break
		}
	}
}

func TestConcatStartsOnlyDemandedSources(t *testing.T) {
	var events []string
	first := recordCompositionSource(&events, "first", 1, 2)
	second := recordCompositionSource(&events, "second", 2, 3)
	sequence := objects.Concat(first, second)
	if len(events) != 0 {
		t.Fatalf("construction started a source: %v", events)
	}

	recordCompositionPrefix(sequence, &events, 2)
	want := []string{"start first", "first 1", "yield 1", "first 2", "yield 2", "close first"}
	if !slices.Equal(events, want) {
		t.Fatalf("stopping at the end of the first source traced %v, want %v", events, want)
	}

	events = nil
	recordCompositionPrefix(sequence, &events, 3)
	wantSecond := append(slices.Clone(want), "start second", "second 2", "yield 2", "close second")
	if !slices.Equal(events, wantSecond) {
		t.Fatalf("stopping inside the second source traced %v, want %v", events, wantSecond)
	}

	events = nil
	recordCompositionPrefix(sequence, &events, 0)
	wantFull := append(slices.Clone(want), "start second", "second 2", "yield 2", "second 3", "yield 3", "close second")
	if !slices.Equal(events, wantFull) {
		t.Fatalf("full replay traced %v, want %v", events, wantFull)
	}
}

func TestAppendAndPrependStartOnlyDemandedValues(t *testing.T) {
	for _, tc := range []struct {
		name  string
		add   func(iter.Seq[int], int) iter.Seq[int]
		limit int
		want  []string
	}{
		{"stop before appended value", objects.Append[int], 1, []string{"start source", "source 1", "yield 1", "close source"}},
		{"append after source cleanup", objects.Append[int], 0, []string{"start source", "source 1", "yield 1", "source 2", "yield 2", "close source", "yield 9"}},
		{"stop at prepended value", objects.Prepend[int], 1, []string{"yield 9"}},
		{"stop in source after prepend", objects.Prepend[int], 2, []string{"yield 9", "start source", "source 1", "yield 1", "close source"}},
		{"prepend before starting source", objects.Prepend[int], 0, []string{"yield 9", "start source", "source 1", "yield 1", "source 2", "yield 2", "close source"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var events []string
			source := recordCompositionSource(&events, "source", 1, 2)
			sequence := tc.add(source, 9)
			if len(events) != 0 {
				t.Fatalf("construction started a source: %v", events)
			}
			recordCompositionPrefix(sequence, &events, tc.limit)
			if !slices.Equal(events, tc.want) {
				t.Fatalf("trace %v, want %v", events, tc.want)
			}
		})
	}
}

func TestSequenceCompositionPreservesEmptyTypedNilAndDuplicates(t *testing.T) {
	item := 7
	empty := slices.Values([]*int(nil))
	left := slices.Values([]*int{nil, &item})
	right := slices.Values([]*int{&item, nil})
	for _, tc := range []struct {
		name     string
		sequence iter.Seq[*int]
		want     []*int
	}{
		{"both empty", objects.Concat(empty, empty), nil},
		{"empty left", objects.Concat(empty, right), []*int{&item, nil}},
		{"empty right", objects.Concat(left, empty), []*int{nil, &item}},
		{"duplicates and nil", objects.Concat(left, right), []*int{nil, &item, &item, nil}},
		{"append nil to empty", objects.Append(empty, nil), []*int{nil}},
		{"prepend nil to empty", objects.Prepend(empty, nil), []*int{nil}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if actual := slices.Collect(tc.sequence); !slices.Equal(actual, tc.want) {
				t.Errorf("sequence %v, want %v", actual, tc.want)
			}
		})
	}
}

func TestCompositionRetainsSingleUseSource(t *testing.T) {
	sourceValues := make(chan int, 2)
	sourceValues <- 1
	sourceValues <- 2
	close(sourceValues)
	source := func(yield func(int) bool) {
		for value := range sourceValues {
			if !yield(value) {
				return
			}
		}
	}
	replayable := slices.Values([]int{3})
	composed := objects.Prepend(objects.Append(objects.Concat(source, replayable), 4), 0)
	for _, want := range [][]int{{0, 1, 2, 3, 4}, {0, 3, 4}} {
		if actual := slices.Collect(composed); !slices.Equal(actual, want) {
			t.Fatalf("enumeration %v, want %v", actual, want)
		}
	}
}

func TestConcatReadsOnlyRequestedPrefix(t *testing.T) {
	pulls, cleanups := 0, 0
	first := func(yield func(int) bool) {
		defer func() { cleanups++ }()
		for value := 1; ; value++ {
			pulls++
			if pulls > 3 {
				t.Fatal("consumer pulled past its requested prefix")
			}
			if !yield(value) {
				return
			}
		}
	}
	second := func(func(int) bool) { t.Fatal("second source started before the first finished") }
	var got []int
	for value := range objects.Concat(first, second) {
		got = append(got, value)
		if value == 3 {
			break
		}
	}
	if !slices.Equal(got, []int{1, 2, 3}) || pulls != 3 || cleanups != 1 {
		t.Fatalf("sequence %v, pulled %d times, cleaned up %d times", got, pulls, cleanups)
	}
}
