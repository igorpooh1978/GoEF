package objects_test

import (
	"fmt"
	"slices"

	"github.com/igorpooh1978/GoEF/linq/objects"
)

func ExampleFirst() {
	source := slices.Values([]int{3, 0, 8})
	evens := objects.Where(source, func(n int) bool { return n%2 == 0 })

	value, found := objects.First(evens)
	fmt.Println(value, found)
	// Output:
	// 0 true
}

func ExampleFirstOrDefault() {
	source := slices.Values([]int{3, 0, 8})
	large := objects.Where(source, func(n int) bool { return n > 10 })

	fmt.Println(objects.FirstOrDefault(large, -1))
	// Output:
	// -1
}
