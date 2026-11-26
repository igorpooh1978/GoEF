package objects

import (
	"iter"
	"math"
)

// WhereIndexed yields the source values that satisfy predicate, in encounter
// order. Positions count all source values, including rejected values. Each
// enumeration starts from zero and stops when its consumer stops. Source and
// predicate must be non-nil. It panics before calling predicate with a position
// above math.MaxInt.
func WhereIndexed[T any](source iter.Seq[T], predicate func(T, int) bool) iter.Seq[T] {
	return func(yield func(T) bool) {
		index := -1
		for value := range source {
			index = advanceIndex(index)
			if predicate(value, index) && !yield(value) {
				return
			}
		}
	}
}

// SelectIndexed yields one selector result for each source value, in encounter
// order. Positions start at zero on each enumeration and refer to this
// operator's immediate source. Source and selector must be non-nil. It stops
// when its consumer stops and panics before calling selector with a position
// above math.MaxInt.
func SelectIndexed[T, R any](source iter.Seq[T], selector func(T, int) R) iter.Seq[R] {
	return func(yield func(R) bool) {
		index := -1
		for value := range source {
			index = advanceIndex(index)
			if !yield(selector(value, index)) {
				return
			}
		}
	}
}

func advanceIndex(index int) int {
	if index == math.MaxInt {
		panic("objects: index exceeds int capacity")
	}
	return index + 1
}
