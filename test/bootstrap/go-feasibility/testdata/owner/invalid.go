package invalid

import shape "example.invalid/goef/go-feasibility"

type account struct{}
type invoice struct{}

var _ = shape.Context{}.Set[account]().Where(shape.Field[invoice, bool]{})
