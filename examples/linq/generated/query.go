package generated

//go:generate go run github.com/igorpooh1978/GoEF/cmd/goef-linqgen -source query.go -output query_gen.go

// Customer is the row type used by the generated binding example.
type Customer struct {
	City string
	Name string
}

//goef:query FindCustomer
func findCustomer(row Customer, city string) (bool, string) {
	return row.City == city, row.Name
}
