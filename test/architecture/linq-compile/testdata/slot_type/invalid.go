package invalid

import f "example.invalid/goef/linq-compile"

type order struct{}
type amount int64
type quantity int64

func reject() {
	total := f.Set[order]().Alias().Field[amount]("total")
	_ = f.Equal(total, f.Slot[quantity]("units"))
}
