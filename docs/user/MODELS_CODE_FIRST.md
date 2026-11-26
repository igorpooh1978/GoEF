# Author a Code First model

The current `model.Validate` validates explicit, resolved model declarations without a database connection. The complete Code First mapping API, relationships, and provider-specific type mappings remain planned.

Describe keys, nullability and relationships in Go metadata, validate them and compute a deterministic logical snapshot. The planned fixture distinguishes SQL NULL from a present zero. Reject invalid key definitions before any schema I/O.

## Validate a resolved declaration

The following program rejects a nullable primary-key member. It prints the one-based entity, property and key-member positions as `1 1 1`. `errors.Is` identifies the portable invalid-model classification, and `errors.As` retrieves the declaration location.

```go
package main

import (
	"errors"
	"fmt"

	"github.com/igorpooh1978/GoEF/goeferr"
	"github.com/igorpooh1978/GoEF/model"
)

func main() {
	err := model.Validate(model.Declaration{Entities: []model.EntityDecl{{
		Name:       "Customer",
		Properties: []model.PropertyDecl{{Name: "ID", Nullable: true}},
		PrimaryKey: []string{"ID"},
	}}})
	var location *model.ValidationError
	if errors.Is(err, goeferr.ErrInvalidModel) && errors.As(err, &location) {
		fmt.Println(location.EntityPosition, location.PropertyPosition, location.KeyMemberPosition)
	}
}
```

Feature IDs: MODEL-MAP-001, MODEL-NULL-001, MODEL-VALID-001.

- [Design or source evidence](../architecture/MODEL_ENGINE.md)
- [Model tests and integration evidence](../evidence/W003/MODEL_KEY_VALIDATION.md)
- [Planned examples and evidence requirements](../../examples/README.md)
