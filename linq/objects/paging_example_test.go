package objects_test

import (
	"fmt"
	"slices"

	"github.com/igorpooh1978/GoEF/linq/objects"
)

func ExampleTake() {
	ids := slices.Values([]int{10, 11, 12, 13, 14})
	page := objects.Take(objects.Skip(ids, 2), 2)
	fmt.Println(slices.Collect(page))
	// Output: [12 13]
}
