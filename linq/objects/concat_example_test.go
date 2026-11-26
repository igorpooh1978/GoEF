package objects_test

import (
	"fmt"
	"slices"

	"github.com/igorpooh1978/GoEF/linq/objects"
)

func ExampleConcat() {
	first := slices.Values([]string{"oak", "elm"})
	second := slices.Values([]string{"elm", "ash"})
	trees := objects.Concat(first, second)
	trees = objects.Prepend(trees, "cedar")
	trees = objects.Append(trees, "fir")
	fmt.Println(slices.Collect(trees))
	// Output: [cedar oak elm elm ash fir]
}
