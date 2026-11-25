package invalid

import shape "example.invalid/goef/go-feasibility"

type account struct{}

var _ shape.Query[int] = shape.Context{}.Set[account]().Select(shape.Field[account, string]{})
