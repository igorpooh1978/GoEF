# Exact SQL decimal column binding

[Issue #3](https://github.com/igorpooh1978/GoEF/issues/3) · [Model type scenarios](../../../specs/features/model/model-type-001.feature) · [Go decimal text](DECIMAL_VALUE.md) · [Implementation](../../../model/decimal_binding.go) · [Tests and example](../../../model/decimal_binding_test.go)

## Binding contract

`model.BindDecimal` takes an explicit provider, positive precision, and scale from zero through the precision. Invalid declarations return a `*model.TypeBindingError` that matches `goeferr.ErrInvalidModel`. `StoreType` returns the generated type declaration. `Encode` validates the number of integer digits and writes exact text at the requested scale. It adds missing fractional zeros and removes only redundant fractional zeros. `Decode` uses the same checks for returned text. Both report an error for a value that would require rounding or that exceeds the integer capacity. Conversion diagnostics omit the supplied value.

| Provider | Declaration | Maximum precision | Maximum scale, within precision |
| --- | --- | --- | --- |
| PostgreSQL | `numeric(p,s)` | 1000 | 1000 |
| SQLite | `TEXT` | 1000 in the Go codec | 1000 in the Go codec |
| SQL Server | `decimal(p,s)` | 38 | 38 |
| MySQL | `decimal(p,s)` | 65 | 30 |

The SQLite declaration preserves the encoded text and relies on the Go codec for precision and scale validation. SQLite applies text comparison to these values. Use explicit bindings for each column and pass encoded strings as parameters. Parameter typing and returned text conversion remain the caller's SQL dialect responsibility. This API returns a type declaration and a codec. Model graph and automatic type binding are separate contracts. [PostgreSQL numeric types](https://www.postgresql.org/docs/19/datatype-numeric.html) · [SQLite affinity](https://www.sqlite.org/datatype3.html) · [SQL Server decimal and numeric](https://learn.microsoft.com/en-us/sql/t-sql/data-types/decimal-and-numeric-transact-sql?view=sql-server-ver16) · [MySQL decimal characteristics](https://dev.mysql.com/doc/refman/8.4/en/precision-math-decimal-characteristics.html)

The pinned EF Core decimal mapping accepts separate precision and scale facets. GoEF validates its narrower portable declaration shape before returning the storage type and checks conversions before a parameter reaches the database. [Pinned EF Core decimal mapping](https://github.com/dotnet/efcore/blob/b34c0e9d7e768aa808ded44f19e7f871bea6869f/src/EFCore.Relational/Storage/DecimalTypeMapping.cs)

## Workstation verification

Windows/amd64, Go 1.27.4, executable source revision `07ceb46d9f6f77c69a3ec6e415c924a8010395d5`, baseline `c9a438928cc25b0c8ee8fde7bf78e27d037d6e65`. Tests cover all four provider declarations, minimum and maximum bounds, full-scale values, exact large values, fixed-scale round trips, canonical zero, discarded nonzero digits, integer overflow and diagnostic redaction.

- RED (exit 1). With the new tests on the baseline, `go test -mod=readonly -count=1 -run '^(TestBindDecimal|TestDecimalBinding)' ./model` failed because the decimal column binding API was absent.
- GREEN (exit 0). `go build -mod=readonly ./...`, `go vet -mod=readonly ./...`, and `go test -mod=readonly -count=1 -shuffle=on ./...` passed, including the runnable example.
- Coverage (exit 0). `go test -mod=readonly -count=1 -shuffle=on -coverprofile=<profile> ./model` covered 383 of 387 model statements (99.0%) and 38 of 38 decimal binding statements (100%).

These checks cover the Go codec and declaration logic. Database catalog and round-trip evidence requires the separate integration run on the pinned engine and driver profiles.
