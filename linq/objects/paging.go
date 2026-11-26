package objects

import "iter"

// Skip yields values after the first count items, in encounter order. A
// nonpositive count yields the whole source. The count starts over on each
// enumeration. When the consumer stops, Skip stops the source.
// Source must be non-nil.
func Skip[T any](source iter.Seq[T], count int) iter.Seq[T] {
	if count <= 0 {
		return source
	}
	return func(yield func(T) bool) {
		remaining := count
		for value := range source {
			if remaining > 0 {
				remaining--
				continue
			}
			if !yield(value) {
				return
			}
		}
	}
}

// Take yields up to count items in encounter order. A nonpositive count
// produces an empty sequence without starting the source. Each enumeration
// stops after the last requested value or when the consumer stops earlier.
// Source must be non-nil.
func Take[T any](source iter.Seq[T], count int) iter.Seq[T] {
	if count <= 0 {
		return func(func(T) bool) {}
	}
	return func(yield func(T) bool) {
		remaining := count
		for value := range source {
			if !yield(value) {
				return
			}
			remaining--
			if remaining == 0 {
				return
			}
		}
	}
}
