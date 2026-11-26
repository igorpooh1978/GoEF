package objects

import "iter"

// Concat yields the values of first followed by the values of second. It starts
// each source only when requested and stops as soon as the consumer stops. Each
// source keeps its own replayability and must be non-nil.
func Concat[T any](first, second iter.Seq[T]) iter.Seq[T] {
	return func(yield func(T) bool) {
		if yieldAll(first, yield) {
			second(yield)
		}
	}
}

// Append yields the values of source followed by value. It yields value only
// after source completes and the consumer is still accepting values. Source
// must be non-nil and retains its own replayability.
func Append[T any](source iter.Seq[T], value T) iter.Seq[T] {
	return func(yield func(T) bool) {
		if yieldAll(source, yield) {
			yield(value) // This is the final value.
		}
	}
}

// Prepend yields value followed by the values of source. It starts source only
// after the consumer accepts value and stops when the consumer stops. Source
// must be non-nil and retains its own replayability.
func Prepend[T any](source iter.Seq[T], value T) iter.Seq[T] {
	return func(yield func(T) bool) {
		if yield(value) {
			source(yield)
		}
	}
}

// yieldAll reports whether the consumer still accepts values after this source.
func yieldAll[T any](source iter.Seq[T], yield func(T) bool) bool {
	more := true
	source(func(value T) bool {
		more = yield(value)
		return more
	})
	return more
}
