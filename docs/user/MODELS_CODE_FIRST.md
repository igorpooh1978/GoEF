# Author a Code First model

`model.Map` resolves table and column names for explicitly registered Go structs. `model.Validate` validates resolved model declarations. Both run without a database connection.

The mapper records direct field nullability and declared primary keys before any schema I/O. The provider binding stage adds physical scalar types. Relationships and custom codecs remain planned.

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

## Map a Go struct

Give each struct an explicit entity name and list its primary-key fields in key order. Direct exported fields become properties. Pointer fields are nullable and embedded fields are unsupported. The default table and column names are the entity and field names, respectively. Names are exact and case-sensitive.

Give a field one `goef` key in a conventional Go struct tag. Its value accepts comma-separated `column=name` and `maxLength=positiveInteger` settings. Use `ToTable`, `ColumnName` and `MaxLength` for fluent settings. They override individual facets, so changing a tagged column name keeps its tagged maximum length. Contradictions at one precedence are invalid even when a higher-precedence option sets that facet.

This program prints `customers customer_name 60`.

```go
package main

import (
	"fmt"

	"github.com/igorpooh1978/GoEF/model"
)

func main() {
	type Customer struct {
		ID   int
		Name string `goef:"column=tag_name,maxLength=60"`
	}
	mapping, err := model.Map(model.Struct[Customer](
		"Customer", []string{"ID"},
		model.ToTable("customers"),
		model.ColumnName("Name", "customer_name"),
	))
	if err != nil {
		panic(err)
	}
	customer := mapping.Entities()[0]
	name := customer.Properties[1]
	fmt.Println(customer.TableName, name.ColumnName, name.MaxLength)
}
```

`Map` returns a typed `MappingError` for invalid mapping options and duplicate effective table or column names. `errors.Is(err, goeferr.ErrInvalidModel)` identifies invalid models. For a column conflict, `errors.As` retrieves the two original struct field positions. The message omits the raw column name. Key failures use `ValidationError` as in the previous example. Its property position counts mapped exported fields only.

`Entities` and `Snapshot` return independent copies. Snapshot version 1 is JSON. It sorts entities by exact name and keeps struct field and key-member order. It records table and column names, maximum lengths, pointer nullability and keys. Dialect-specific type binding has its own physical snapshot. Identifier quoting, schema introspection and generation are separate stages.

## Bind the mapped model to a provider

Call `model.BindTypes` with `model.TypePostgreSQL`, `model.TypeSQLite`, `model.TypeSQLServer` or `model.TypeMySQL`. The resulting `BoundModel` provides column type declarations and a separate version 1 physical snapshot. This example uses Go 1.27.4 and runs without a driver or database connection. Its output is `ID bigint false` and `Label varchar(16) true` on separate lines.

```go
package main

import (
	"fmt"

	"github.com/igorpooh1978/GoEF/model"
)

func main() {
	type Record struct {
		ID    int64
		Label *string `goef:"maxLength=16"`
	}
	mapped, err := model.Map(model.Struct[Record]("Record", []string{"ID"}))
	if err != nil {
		panic(err)
	}
	bound, err := model.BindTypes(mapped, model.TypePostgreSQL)
	if err != nil {
		panic(err)
	}
	for _, field := range bound.Entities()[0].Properties {
		fmt.Println(field.Name, field.StoreType, field.Nullable)
	}
}
```

| Go field | PostgreSQL | SQLite | SQL Server | MySQL |
| --- | --- | --- | --- | --- |
| `bool` | `boolean` | `INTEGER` | `bit` | `boolean` |
| `int8` | `smallint` | `INTEGER` | `smallint` | `tinyint` |
| `int16` | `smallint` | `INTEGER` | `smallint` | `smallint` |
| `int32` | `integer` | `INTEGER` | `int` | `int` |
| `int64`, `int` | `bigint` | `INTEGER` | `bigint` | `bigint` |
| `string` | `text` | `TEXT` | `nvarchar(max)` | `longtext CHARACTER SET utf8mb4` |
| `string` with maximum length `n` | `varchar(n)` | `TEXT` | `nvarchar(n)` | `varchar(n) CHARACTER SET utf8mb4` |
| `[]byte` | `bytea` | `BLOB` | `varbinary(max)` | `longblob` |

A single pointer to a supported Go type is nullable. Plain `[]byte` remains a non-nullable model property. Use non-nil byte slices when supplying values to a non-null SQL column through a driver. Built-in aliases have the same mapping. Distinct defined types and unsupported types such as `uint64`, floating-point values and exact decimals fail with `*model.TypeBindingError` and `goeferr.ErrInvalidModel`. The error reports positions in canonical entity and mapped field order. An unsupported field prevents publication of the entire binding.

Length is a text facet. PostgreSQL and MySQL use character counts. SQL Server counts UTF-16 units under a supplementary-character collation. SQLite retains length as metadata and emits `TEXT`. SQL Server collation and PostgreSQL client and server encodings determine the supported text domain. MySQL bounded declarations also share a row-size limit. Supply valid text representable in the selected dialect and enforce the intended length before passing values to a driver. Value encoding, identifier quoting, index validation, DDL and schema generation belong to later stages.

[Compiled example and binding tests](../../model/type_binding_test.go) · [Source and engine evidence](../evidence/W003/PROVIDER_TYPE_BINDING.md)



Feature IDs: MODEL-MAP-001, MODEL-NULL-001, MODEL-TYPE-001, MODEL-VALID-001.

- [Design or source evidence](../architecture/MODEL_ENGINE.md)
- [Code First mapping tests and evidence](../evidence/W003/MODEL_MAPPING.md)
- [Model validation tests and integration evidence](../evidence/W003/MODEL_KEY_VALIDATION.md)
- [Planned examples and evidence requirements](../../examples/README.md)
