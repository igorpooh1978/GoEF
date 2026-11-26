package objects_test

import (
	"fmt"
	"slices"

	"github.com/igorpooh1978/GoEF/linq/objects"
)

func ExampleAnyBy() {
	values := slices.Values([]int{4, 7, 12})
	evens := objects.Where(values, func(value int) bool { return value%2 == 0 })
	doubled := objects.Select(evens, func(value int) int { return value * 2 })

	fmt.Println(objects.Any(values))
	fmt.Println(objects.AnyBy(doubled, func(value int) bool { return value > 10 }))
	fmt.Println(objects.All(doubled, func(value int) bool { return value < 20 }))
	// Output:
	// true
	// true
	// false
}
