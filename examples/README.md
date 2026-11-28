# Example catalog

## Database quickstarts

These examples run against the published GoEF and adapter modules at `v0.2.0-alpha.1` in a separate Go module without `replace`. Each walkthrough includes the installation commands, connection settings, and expected output.

| Database | Walkthrough | Go source |
| --- | --- | --- |
| SQLite | [Temporary-file query](../README.md#quick-start-with-sqlite) | [Adapter example](../providers/sqlite/example_test.go) |
| PostgreSQL | [Parameterized query](../README.md#postgresql) | [PostgreSQL adapter code](testdata/alpha-quickstart/postgres/adapter.go) |
| SQL Server Express LocalDB | [Windows named-pipe query](../README.md#sql-server-express-localdb) | [LocalDB adapter code](testdata/alpha-quickstart/sqlserver/adapter.go) |
| MySQL | [Parameterized query](../README.md#mysql) | [MySQL adapter code](testdata/alpha-quickstart/mysql/adapter.go) |

PostgreSQL, LocalDB, and MySQL share [main.go](testdata/alpha-quickstart/main.go) and a [Python launcher](testdata/alpha-quickstart/run.py). Use both files with the adapter code for your database. Each query returns `42`.

## GoLINQ and generated bindings

The [GoLINQ Objects guide](../docs/user/LINQ_OBJECTS.md) links runnable examples for filtering and projection, counting, indexed filtering, quantifiers, paging, and sequence composition. Run them from the repository root with Go 1.27.4.

```sh
go test -run '^Example' -count=1 -v ./linq/objects
```

The [generated customer binding](linq/generated/query.go) binds two city arguments and projects a scalar field. Run `go generate ./examples/linq/generated` and `go test -run '^Example$' -count=1 -v ./examples/linq/generated`. See the [generated binding guide](../docs/user/LINQ_GENERATED.md).

## Planned examples

The [language feasibility fixture](../test/bootstrap/go-feasibility/) demonstrates Go compiler behavior. The workflows below still need canonical runnable examples.

| Planned runnable example | Scope | Required acceptance |
| --- | --- | --- |
| Code First customer model | [Quickstart](../docs/user/QUICKSTART.md), MODEL-MAP-001 | Compiles and preserves model and rows on all four engines |
| Database query then explicit client computation | [Client/server boundary](../docs/user/LINQ_CLIENT_SERVER_BOUNDARY.md), LINQ-BOUNDARY-001 | Records bounded server result and explicit client transition |
| Caller-owned transaction and rollback | [Transactions](../docs/user/TRANSACTIONS.md), TX-OWN-001 | Persists or rolls back rows exactly as the caller requests |
| Reviewed migration in Go | [Migrations in Go](../docs/user/MIGRATIONS_IN_GO.md), MIG-API-001 | Preserves existing data and applies the migration exactly once |
| Safe handling of portable errors | [Error handling](../docs/user/ERROR_HANDLING.md), ERR-CONTRACT-001 | Shows known and unknown errors without leaking inputs |

A runnable example needs canonical Go source, imports, commands, expected output, isolated test data, and recorded execution. See the [documentation policy](../docs/development/DOCUMENTATION_POLICY.md).
