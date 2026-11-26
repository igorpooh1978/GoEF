# Transform an in-memory sequence

The current `objects.Where` filters an `iter.Seq` with a typed predicate. The projection and terminal count APIs in the wider example remain planned.

Compose a local filter, a typed projection and a terminal count. The planned example measures enumeration and early stop, repeats a replayable source, and checks the behavior of an empty sequence. It needs no provider configuration.

## Stop after the first match

Creating the filter leaves the source idle. The loop prints `2` and stops after inspecting `1` and `2`. Supply a non-nil source and predicate. The filtered sequence retains the replayability of its source.

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
	for n := range evens {
		fmt.Println(n)
		break
	}
}
```

Feature IDs: LINQ-OBJECTS-001, LINQ-CONFORMANCE-001.

- [Design or source evidence](../architecture/LINQ_OBJECTS.md)
- [Where tests and integration evidence](../evidence/W008/OBJECTS_WHERE.md)
- [Planned examples and evidence requirements](../../examples/README.md)
