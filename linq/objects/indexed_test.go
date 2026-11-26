package objects_test

import (
	"fmt"
	"iter"
	"slices"
	"testing"

	"github.com/igorpooh1978/GoEF/linq/objects"
)

type indexedInput struct {
	value int
	index int
}

func TestIndexedWhereSelectStopsAndResetsBothPositions(t *testing.T) {
	starts, cleanups := 0, 0
	var reads []int
	var predicateInputs, selectorInputs []indexedInput
	source := func(yield func(int) bool) {
		starts++
		defer func() { cleanups++ }()
		for _, value := range []int{1, 2, 4, 2, 3} {
			reads = append(reads, value)
			if !yield(value) {
				return
			}
		}
	}
	filtered := objects.WhereIndexed(source, func(value, index int) bool {
		predicateInputs = append(predicateInputs, indexedInput{value: value, index: index})
		return value%2 == 0
	})
	projected := objects.SelectIndexed(filtered, func(value, index int) string {
		selectorInputs = append(selectorInputs, indexedInput{value: value, index: index})
		return fmt.Sprintf("%d:%d", index, value)
	})

	if starts != 0 || cleanups != 0 || len(reads) != 0 || len(predicateInputs) != 0 || len(selectorInputs) != 0 {
		t.Fatal("constructing the pipeline used its source or callbacks")
	}
	var first []string
	for value := range projected {
		first = append(first, value)
		break
	}
	if !slices.Equal(first, []string{"0:2"}) || starts != 1 || cleanups != 1 || !slices.Equal(reads, []int{1, 2}) {
		t.Fatalf("first iteration yielded %v, read %v, started %d times and cleaned up %d times", first, reads, starts, cleanups)
	}
	if !slices.Equal(predicateInputs, []indexedInput{{1, 0}, {2, 1}}) || !slices.Equal(selectorInputs, []indexedInput{{2, 0}}) {
		t.Fatalf("first iteration passed %v into predicate and %v into selector", predicateInputs, selectorInputs)
	}

	reads, predicateInputs, selectorInputs = nil, nil, nil
	all := slices.Collect(projected)
	if !slices.Equal(all, []string{"0:2", "1:4", "2:2"}) || starts != 2 || cleanups != 2 || !slices.Equal(reads, []int{1, 2, 4, 2, 3}) {
		t.Fatalf("replay yielded %v, read %v, started %d times and cleaned up %d times", all, reads, starts, cleanups)
	}
	if !slices.Equal(predicateInputs, []indexedInput{{1, 0}, {2, 1}, {4, 2}, {2, 3}, {3, 4}}) ||
		!slices.Equal(selectorInputs, []indexedInput{{2, 0}, {4, 1}, {2, 2}}) {
		t.Fatalf("replay passed %v into predicate and %v into selector", predicateInputs, selectorInputs)
	}
}

func TestIndexedOperatorsKeepSingleUseInput(t *testing.T) {
	values := make(chan int, 3)
	values <- 7
	values <- 8
	values <- 9
	close(values)
	source := func(yield func(int) bool) {
		for value := range values {
			if !yield(value) {
				return
			}
		}
	}
	var predicateInputs, selectorInputs []indexedInput
	filtered := objects.WhereIndexed(source, func(value, index int) bool {
		predicateInputs = append(predicateInputs, indexedInput{value: value, index: index})
		return true
	})
	projected := objects.SelectIndexed(filtered, func(value, index int) int {
		selectorInputs = append(selectorInputs, indexedInput{value: value, index: index})
		return value
	})

	for value := range projected {
		if value != 7 {
			t.Errorf("first value = %d, want 7", value)
		}
		break
	}
	if remaining := slices.Collect(projected); !slices.Equal(remaining, []int{8, 9}) {
		t.Errorf("second iteration yielded %v, want 8 and 9", remaining)
	}
	if remaining := slices.Collect(projected); len(remaining) != 0 {
		t.Errorf("exhausted input yielded %v", remaining)
	}
	want := []indexedInput{{7, 0}, {8, 0}, {9, 1}}
	if !slices.Equal(predicateInputs, want) || !slices.Equal(selectorInputs, want) {
		t.Errorf("predicate inputs %v and selector inputs %v, want %v in both", predicateInputs, selectorInputs, want)
	}
}

func TestIndexedOperatorsHandleEmptyAndTypedNilValues(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		var empty iter.Seq[*int] = slices.Values([]*int(nil))
		filtered := objects.WhereIndexed(empty, func(*int, int) bool {
			t.Fatal("empty source called predicate")
			return true
		})
		projected := objects.SelectIndexed(filtered, func(*int, int) int {
			t.Fatal("empty source called selector")
			return 0
		})
		if result := slices.Collect(projected); len(result) != 0 {
			t.Errorf("empty source yielded %v", result)
		}
	})
	t.Run("typed nil and duplicates", func(t *testing.T) {
		value := 9
		source := slices.Values([]*int{nil, &value, nil})
		filtered := objects.WhereIndexed(source, func(value *int, index int) bool {
			return value == nil && index%2 == 0
		})
		projected := objects.SelectIndexed(filtered, func(value *int, index int) int {
			if value != nil {
				t.Fatal("typed nil was replaced")
			}
			return index
		})
		if result := slices.Collect(projected); !slices.Equal(result, []int{0, 1}) {
			t.Errorf("typed nil values yielded positions %v, want 0 and 1", result)
		}
	})
}

func ExampleWhereIndexed() {
	source := slices.Values([]string{"oak", "elm", "ash", "fir"})
	alternating := objects.WhereIndexed(source, func(_ string, index int) bool {
		return index%2 == 1
	})
	numbered := objects.SelectIndexed(alternating, func(tree string, index int) string {
		return fmt.Sprintf("%d %s", index, tree)
	})
	for line := range numbered {
		fmt.Println(line)
	}
	// Output:
	// 0 elm
	// 1 fir
}
