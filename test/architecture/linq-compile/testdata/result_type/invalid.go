package invalid

import f "example.invalid/goef/linq-compile"

type order struct{}
type orderID int

func reject() {
	root := f.Set[order]()
	var _ f.Query[int] = root.Select(root.Alias().Field[orderID]("id"))
}
