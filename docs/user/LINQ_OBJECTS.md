# Transform an in-memory sequence

Use `objects.Where` to filter a typed `iter.Seq` and `objects.Select` to project each match to a new type. This example requires Go 1.27.4 and prints `even: 2`. [Runnable example](../../linq/objects/select_example_test.go)

```go
package main

import (
	"fmt"
	"slices"

	"github.com/igorpooh1978/GoEF/linq/objects"
)

func main() {
	evens := objects.Where(slices.Values([]int{1, 2, 3, 4}), func(n int) bool {
		return n%2 == 0
	})
	labels := objects.Select(evens, func(n int) string {
		return fmt.Sprintf("even: %d", n)
	})
	for label := range labels {
		fmt.Println(label)
		break
	}
}
```

Constructing the pipeline leaves the iterator and both callbacks idle. The loop inspects `1` and `2`, projects `2`, and forwards the stop signal to the source. If you replay this slice-backed pipeline to completion, it yields `even: 2` and `even: 4` in that order. `Select` yields one result per input, including duplicate and typed nil values. An empty input yields no values and calls no selector.

Supply a non-nil iterator and non-nil callbacks. The source determines replayability and owns any resources it opens. For concurrent enumeration, the source and callbacks must support concurrent use.

## Count projected values

`objects.Count` enumerates a finite sequence once and returns its number of values as a Go `int`. It runs the predicate for every input and the selector for every match in this pipeline. Equal projections each contribute to the count. This example prints `3`. [Runnable Count example](../../linq/objects/count_example_test.go)

```go
package main

import (
	"fmt"
	"slices"

	"github.com/igorpooh1978/GoEF/linq/objects"
)

func main() {
	evens := objects.Where(slices.Values([]int{1, 2, 4, 2, 3}), func(n int) bool {
		return n%2 == 0
	})
	labels := objects.Select(evens, func(n int) string {
		return fmt.Sprintf("even: %d", n)
	})
	fmt.Println(objects.Count(labels))
}
```

An empty sequence returns zero. A second count starts a new enumeration, so a replayable source can produce the same count and an exhausted single-use source returns zero. The counter uses constant space and completes upstream callbacks synchronously. The source must be non-nil. `Count` panics if the source yields more than `math.MaxInt` values.

## Filter or project by position

Use `objects.WhereIndexed` or `objects.SelectIndexed` when a callback needs the element and its zero-based position. The predicate counts every input, including rejected values. The selector numbers its own input, so after a filter it counts the matches. This example prints `0 elm` and `1 fir`. [Runnable indexed example](../../linq/objects/indexed_test.go)

```go
package main

import (
	"fmt"
	"slices"

	"github.com/igorpooh1978/GoEF/linq/objects"
)

func main() {
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
}
```

Both operators start counting at zero on each enumeration and stop with the consumer. On a single-use input the next enumeration numbers the remaining elements. The index is a Go `int`. If the source yields another value after position `math.MaxInt`, the operator panics before invoking the callback for that value. [Indexed behavior and test evidence](../evidence/W008/OBJECTS_INDEXED.md)

## Check whether values match

`objects.Any` checks whether the source yields a value. `objects.AnyBy` checks whether at least one value matches a predicate. `objects.All` checks whether every value matches. The following example prints `true`, `true`, and `false` on separate lines. [Runnable quantifier example](../../linq/objects/quantifiers_example_test.go)

```go
package main

import (
	"fmt"
	"slices"

	"github.com/igorpooh1978/GoEF/linq/objects"
)

func main() {
	values := slices.Values([]int{4, 7, 12})
	evens := objects.Where(values, func(value int) bool { return value%2 == 0 })
	doubled := objects.Select(evens, func(value int) int { return value * 2 })

	fmt.Println(objects.Any(values))
	fmt.Println(objects.AnyBy(doubled, func(value int) bool { return value > 10 }))
	fmt.Println(objects.All(doubled, func(value int) bool { return value < 20 }))
}
```

`Any` stops at the first value, `AnyBy` at the first match, and `All` at the first failure. On an empty source, `Any` and `AnyBy` return `false` and `All` returns `true`. Each call starts a new enumeration. A call on an unbounded source keeps reading until it can decide the answer. [Short-circuit test evidence](../evidence/W008/OBJECTS_QUANTIFIERS.md)

## Select a window of values

`objects.Skip(source, count)` ignores the first `count` elements. A nonpositive count forwards the source from its first value. `objects.Take(source, count)` yields at most `count` values and returns an empty sequence for a nonpositive count without starting the source. Positive `Take` stops on its last yielded value, so a finite window can come from an unbounded sequence. [Runnable paging example](../../linq/objects/paging_example_test.go) · [Paging evidence](../evidence/W008/OBJECTS_SKIP_TAKE.md)

## Join sources and values

`objects.Concat(first, second)` yields the first sequence followed by the second. `objects.Append(source, value)` yields the value after the source ends. `objects.Prepend(source, value)` yields the value before starting the source. They preserve duplicates and encounter order. An early stop reaches the active source and leaves any later source unopened. [Runnable composition example](../../linq/objects/concat_example_test.go) · [Composition evidence](../evidence/W008/OBJECTS_CONCAT.md)

## Read the first value

Use `objects.First` when you need to distinguish a yielded Go zero value from an empty sequence. The first example prints `0 true`. The second calls `objects.FirstOrDefault` with an explicit fallback and prints `-1`. [Runnable examples](../../linq/objects/first_example_test.go)

```go
source := slices.Values([]int{3, 0, 8})
evens := objects.Where(source, func(n int) bool { return n%2 == 0 })
value, found := objects.First(evens)
fmt.Println(value, found)

large := objects.Where(source, func(n int) bool { return n > 10 })
fmt.Println(objects.FirstOrDefault(large, -1))
```

An empty sequence returns the zero value of its element type with `found == false` from `First`. `FirstOrDefault` returns the supplied fallback only when its input yields nothing. A yielded zero or typed nil is a valid first result. Both operators stop after the first result in encounter order, before any later element is read. Use `Where` for predicate-based selection. Both operators return after source cleanup, and the source controls replayability. A filter with no match on an unbounded input keeps reading. [Selection evidence](../evidence/W008/OBJECTS_FIRST.md)

- [LINQ Objects design](../architecture/LINQ_OBJECTS.md)
- [Where test evidence](../evidence/W008/OBJECTS_WHERE.md)
- [Select and pipeline test evidence](../evidence/W008/OBJECTS_SELECT.md)
- [Count and pipeline test evidence](../evidence/W008/OBJECTS_COUNT.md)
