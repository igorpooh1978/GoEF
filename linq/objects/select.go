package objects

import "iter"

// Select yields one selector result for each value in source, in encounter
// order. Each iteration starts source and stops when its consumer stops, so
// replayability follows source. Source and selector must be non-nil.
func Select[T, R any](source iter.Seq[T], selector func(T) R) iter.Seq[R] {
	return func(yield func(R) bool) {
		for value := range source {
			if !yield(selector(value)) {
				return
			}
		}
	}
}
