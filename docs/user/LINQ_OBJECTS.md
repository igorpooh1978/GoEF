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

- [LINQ Objects design](../architecture/LINQ_OBJECTS.md)
- [Where test evidence](../evidence/W008/OBJECTS_WHERE.md)
- [Select and pipeline test evidence](../evidence/W008/OBJECTS_SELECT.md)
