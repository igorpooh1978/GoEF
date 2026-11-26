package generated_test

import (
	"fmt"

	"github.com/igorpooh1978/GoEF/examples/linq/generated"
)

func Example() {
	city := "London"
	first := generated.FindCustomer(city)
	city = "Berlin"
	second := generated.FindCustomer(city)

	fmt.Println(first.Shape() == second.Shape())
	fmt.Println(first.Value(), second.Value())
	fmt.Println(second.Project(generated.Customer{City: "Berlin", Name: "Ada"}))
	// Output:
	// true
	// London Berlin
	// Ada
}
