package invalid

type query[T any] struct{}

func (query[T]) Nest() query[[]T] { return query[[]T]{} }

var _ = query[int]{}
