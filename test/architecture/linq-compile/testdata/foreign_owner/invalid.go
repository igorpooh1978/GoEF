package invalid

import f "example.invalid/goef/linq-compile"

type order struct{}
type invoice struct{}

func reject() {
	paid := f.Set[invoice]().Alias().Field[bool]("paid")
	_ = f.Set[order]().Where(f.Equal(paid, f.Slot[bool]("paid")))
}
