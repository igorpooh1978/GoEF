package objects

import "iter"

// First returns the first value from source and true. An empty source returns
// the zero value of T and false. It stops the source after the first value and
// returns after the source unwinds. Each call starts a new enumeration whose
// replayability follows source. Source must be non-nil.
func First[T any](source iter.Seq[T]) (T, bool) {
	for value := range source {
		return value, true
	}
	var zero T
	return zero, false
}

// FirstOrDefault returns the first yielded value or fallback when source is
// empty. A yielded zero value is returned as is. It enumerates source with
// the same early-stop behavior as First. Source must be non-nil.
func FirstOrDefault[T any](source iter.Seq[T], fallback T) T {
	if value, found := First(source); found {
		return value
	}
	return fallback
}
