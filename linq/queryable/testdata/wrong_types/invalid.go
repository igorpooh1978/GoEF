package wrong_types

import (
	"context"

	"github.com/igorpooh1978/GoEF/linq/expressions"
	"github.com/igorpooh1978/GoEF/linq/queryable"
	"github.com/igorpooh1978/GoEF/provider"
)

type Customer struct{ City string }
type Order struct{ City string }

var _ expressions.Equality[Customer, string] = expressions.EqualField[Order, string]("City")

func wrongParameter(plan queryable.Plan[Customer, string], backend provider.Backend) {
	plan.Fetch(context.Background(), backend, 42)
}
