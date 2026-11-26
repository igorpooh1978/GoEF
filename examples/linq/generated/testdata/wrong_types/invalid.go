package invalid

import "github.com/igorpooh1978/GoEF/examples/linq/generated"

type ForeignRow struct {
	City string
	Name string
}

var _ = generated.FindCustomer(123)
var _ int = generated.FindCustomer("London").Project(generated.Customer{})
var _ = generated.FindCustomer("London").Project(ForeignRow{})
