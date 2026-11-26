package objects_test

import (
	"fmt"
	"slices"

	"github.com/igorpooh1978/GoEF/linq/objects"
)

func ExampleSelect() {
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

	// Output:
	// even: 2
}
