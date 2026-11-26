// Package objects provides typed in-memory sequence operators.
package objects

import "iter"

// Where yields the values of source that satisfy predicate, in encounter order.
// Each iteration starts source and stops when its consumer stops. A single-use
// source retains its own iteration behavior. Source and predicate must be non-nil.
func Where[T any](source iter.Seq[T], predicate func(T) bool) iter.Seq[T] {
	return func(yield func(T) bool) {
		for value := range source {
			if !predicate(value) {
				continue
			}
			if !yield(value) {
				return
			}
		}
	}
}
