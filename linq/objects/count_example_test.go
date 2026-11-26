package objects_test

import (
	"fmt"
	"slices"

	"github.com/igorpooh1978/GoEF/linq/objects"
)

func ExampleCount() {
	evens := objects.Where(slices.Values([]int{1, 2, 4, 2, 3}), func(n int) bool {
		return n%2 == 0
	})
	labels := objects.Select(evens, func(n int) string {
		return fmt.Sprintf("even: %d", n)
	})
	fmt.Println(objects.Count(labels))

	// Output:
	// 3
}
