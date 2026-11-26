package objects

import "iter"

// Any reports whether source yields an element. It stops at the first value.
// Source must be non-nil.
func Any[T any](source iter.Seq[T]) bool {
	for range source {
		return true
	}
	return false
}

// AnyBy reports whether a value from source satisfies predicate. It tests
// values in encounter order and stops at the first match. Source and predicate
// must be non-nil.
func AnyBy[T any](source iter.Seq[T], predicate func(T) bool) bool {
	for value := range source {
		if predicate(value) {
			return true
		}
	}
	return false
}

// All reports whether every value from source satisfies predicate. It tests
// values in encounter order and stops at the first failure. It returns true
// for an empty source. Source and predicate must be non-nil.
func All[T any](source iter.Seq[T], predicate func(T) bool) bool {
	for value := range source {
		if !predicate(value) {
			return false
		}
	}
	return true
}
