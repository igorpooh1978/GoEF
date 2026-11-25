package invalid

import f "example.invalid/goef/linq-compile"

type order struct{}

func reject() {
	_ = f.Set[order]().Where(func(order) bool { return true })
}
