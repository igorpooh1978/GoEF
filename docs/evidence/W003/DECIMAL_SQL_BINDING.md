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

### Integrated source checks

The integrated source candidate is `acd3c5a6250b80dcd716630291174c4dfb93dcce`. On Windows/amd64 with Go 1.27.4, `go mod verify`, `go mod tidy -diff`, `go build -mod=readonly ./...`, `go vet -mod=readonly ./...` and `go test -mod=readonly -count=1 -shuffle=on ./...` exited 0 in the root and driver lab modules. The repository validator checked 76 Gherkin scenarios. All seven bootstrap tests and 78 Python lab tests passed.

The driver fixture with the new test on the earlier `c9a438928cc25b0c8ee8fde7bf78e27d037d6e65` baseline failed to compile because `BindDecimal` was absent. The four focused runner tests failed before the new selector and verifier existed, then passed. Integrated statement coverage was 383/387 for model (99.0%), 75/76 for LINQ Objects (98.7%), 457/478 for transaction (95.6%), and 23/23 for errors (100%). [Real-engine fixture](../../../test/conformance/driverlab/decimal_binding_test.go) · [Runner evidence tests](../../../test/conformance/lab/test_driver_run.py) · [Paging verification](../W008/OBJECTS_SKIP_TAKE.md) · [Composition verification](../W008/OBJECTS_CONCAT.md)

The Windows/386 LINQ Objects suite passed `go test -mod=readonly -count=1 -shuffle=on -cover ./linq/objects` with 100% statement coverage, including the existing index and count overflow paths and both new operation groups.

With Go 1.27.4 in a network-isolated Linux/amd64 container on the workstation, `go test -mod=readonly -race -count=1 -shuffle=on ./...` exited 0 for the five root packages and `go test -mod=vendor -race -count=1 -shuffle=on ./...` exited 0 for the driver lab using a fresh temporary vendor snapshot.

### Real-engine catalog and value checks

The integrated source candidate passed `MODEL-TYPE-001-S01` on all 12 pinned profiles, with 13 rows because SQLite 3.54.1 ran twice. The workstation built test binaries for the host and Linux once, called `driver_run.run_provider(..., model_types=True, owned_tx=True, savepoint_tx=True, save_changes=True, tx_options=True, decimal_binding=True)`, and ran the SQLite compatibility launcher with the same selectors. This invocation exited 0. It also passed 84 distinct driver checks, four SQLite file checks, 12 scalar type profiles, 12 owned transaction profiles, 24 explicit savepoint checks, 36 SaveChanges checks and 12 isolation profiles.

| Provider | Verified profiles | Go driver |
| --- | --- | --- |
| PostgreSQL | 16.16, 17.12, 18.7 and 19.0 | pgx v5.13.0 |
| SQLite | 3.51.3 and 3.54.1 | modernc.org/sqlite v1.47.0 and v1.63.0 |
| SQL Server Developer | 14.0.3570.2 at 140, 15.0.4510.2 at 150, 16.0.4325.4 at 140 and 160 | go-mssqldb v1.11.0 |
| MySQL | 8.0.46 and 8.4.13.1 | go-sql-driver/mysql v1.10.0 |

The fixture uses precision 38 with scale 9 and scale 0, writes bound strings and reads exact positive, negative and boundary values from an independent physical connection. It verifies native DDL facets where available and SQLite text storage with `typeof()`. Higher declaration maxima and scale up to 1000 are covered by Go unit tests. [Fixture source](../../../test/conformance/driverlab/decimal_binding_test.go)

Run the same selection on the workstation with `pwsh -NoProfile -File scripts/ci/goef-w002.ps1 -Compatibility -ModelTypes -OwnedTx -SavepointTx -SaveChanges -TxOptions -DecimalBinding -AcceptSqlServerEula -AcceptLegacySqlServerEula`.
