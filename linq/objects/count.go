package objects

import (
	"iter"
	"math"
)

// Count returns the number of values yielded by a finite source as a Go int.
// It enumerates source once, completing upstream callbacks before returning.
// Source must be non-nil. Count panics if source yields more than math.MaxInt values.
func Count[T any](source iter.Seq[T]) int {
	count := 0
	for range source {
		if count == math.MaxInt {
			panic("objects: count exceeds int capacity")
		}
		count++
	}
	return count
}
