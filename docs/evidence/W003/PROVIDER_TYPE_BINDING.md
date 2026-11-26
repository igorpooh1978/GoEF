# Code First provider type binding

[Issue #3](https://github.com/igorpooh1978/GoEF/issues/3) · [Code First guide](../../user/MODELS_CODE_FIRST.md#bind-the-mapped-model-to-a-provider) · [Scenarios](../../../specs/features/model/model-type-001.feature)

## Pinned source research

EF Core v10.0.15 at `b34c0e9d7e768aa808ded44f19e7f871bea6869f` uses provider type mapping sources. The SQL Server source and tests pair CLR Boolean and signed integers with `bit` and signed numeric types, and pair text and binary with `nvarchar` and `varbinary`. The SQLite source and tests exercise storage affinity mappings and provider-specific type names. GoEF uses independently written mappings for an initial Go scalar set. The CLR byte is unsigned. GoEF's `int8` is signed and selects `smallint` on SQL Server and PostgreSQL. [Pinned SQL Server implementation](https://github.com/dotnet/efcore/blob/b34c0e9d7e768aa808ded44f19e7f871bea6869f/src/EFCore.SqlServer/Storage/Internal/SqlServerTypeMappingSource.cs) · [Pinned SQL Server tests](https://github.com/dotnet/efcore/blob/b34c0e9d7e768aa808ded44f19e7f871bea6869f/test/EFCore.SqlServer.Tests/Storage/SqlServerTypeMappingSourceTest.cs) · [Pinned SQLite implementation](https://github.com/dotnet/efcore/blob/b34c0e9d7e768aa808ded44f19e7f871bea6869f/src/EFCore.Sqlite.Core/Storage/Internal/SqliteTypeMappingSource.cs) · [Pinned SQLite tests](https://github.com/dotnet/efcore/blob/b34c0e9d7e768aa808ded44f19e7f871bea6869f/test/EFCore.Sqlite.Tests/Storage/SqliteTypeMappingTest.cs) · [Source license](https://github.com/dotnet/efcore/blob/b34c0e9d7e768aa808ded44f19e7f871bea6869f/LICENSE.txt)

The tagged provider drivers and exact engine targets come from the existing local lab. On 2026-11-26 no dedicated provider documentation MCP tools were available in this session. This research uses the direct official manuals and pinned upstream source routes. The exact engine check remains a separate gate. [MCP policy](../../development/MCP_POLICY.md) · [Engine compatibility](../../parity/ENGINE_COMPATIBILITY.md) · [Pinned drivers](../../../test/conformance/driverlab/go.mod)

| Target | Native storage contract used by the binding |
| --- | --- |
| PostgreSQL | Signed integers use `smallint`, `integer` or `bigint`. Text uses `text` or `varchar(n)`. PostgreSQL counts characters for the limit and caps `n` at 10,485,760. Binary uses `bytea`. [Numeric types](https://www.postgresql.org/docs/18/datatype-numeric.html) · [Character types](https://www.postgresql.org/docs/18/datatype-character.html) · [Binary type](https://www.postgresql.org/docs/18/datatype-binary.html) · [pgx codec documentation](https://pkg.go.dev/github.com/jackc/pgx/v5@v5.13.0/pgtype) |
| SQLite | `INTEGER`, `TEXT` and `BLOB` are accepted strict-table types. Booleans are represented by integer 0 and 1. The bounded text facet remains metadata and the binder uses `TEXT`. [Datatypes](https://www.sqlite.org/datatype3.html) · [Strict tables](https://www.sqlite.org/stricttables.html) · [modernc.org/sqlite v1.63.0](https://pkg.go.dev/modernc.org/sqlite@v1.63.0) |
| SQL Server | `nvarchar(n)` reserves `n` byte-pairs, with a 4,000-unit limit for the sized declaration. Supplementary characters need a supporting collation and may use two units. Unspecified text and binary lengths use the `max` forms. [Types](https://learn.microsoft.com/en-us/sql/t-sql/data-types/data-types-transact-sql?view=sql-server-ver16) · [Unicode and lengths](https://learn.microsoft.com/en-us/sql/t-sql/data-types/nchar-and-nvarchar-transact-sql?view=sql-server-ver16) · [go-mssqldb v1.11.0](https://github.com/microsoft/go-mssqldb/blob/v1.11.0/README.md) |
| MySQL | `boolean` aliases `tinyint(1)`. The binding selects `utf8mb4` for both text declarations. The maximum declaration bound of 16,383 uses four bytes per character and the two-byte prefix within the 65,535-byte row budget. The full row layout and SQL mode still affect what can be stored. [Numeric syntax](https://dev.mysql.com/doc/refman/8.4/en/numeric-type-syntax.html) · [VARCHAR limits](https://dev.mysql.com/doc/refman/8.4/en/char.html) · [BLOB and TEXT](https://dev.mysql.com/doc/refman/8.4/en/blob.html) · [go-sql-driver/mysql v1.10.0](https://github.com/go-sql-driver/mysql/blob/v1.10.0/README.md) |

## Local Go verification

On Windows/amd64 with Go 1.27.4, the baseline `9b5defe005efdc9adcbbb63daabbc261818bdf85` failed to compile `go test -mod=readonly -count=1 -run '^TestBindTypesMODEL_TYPE_001S0[23]$' ./model` after adding the assertions. A behavioral mutation then changed PostgreSQL Boolean to `bigint` and accepted `uint64` as a signed integer. Both target tests failed with exit 1. The original bytes were restored and the targeted tests passed.

`TestBindTypesMODEL_TYPE_001S02` asserts the selected type, logical kind, length facet, field order and nullability for each dialect. `TestBindTypesMODEL_TYPE_001S03` asserts rejection of `uint64` across all four targets, typed classification, numeric coordinates and redacted text. Additional checks cover native length boundaries, unsupported kinds, snapshot determinism and defensive ownership. The portable model snapshot remains version 1. [Unit tests and example](../../../model/type_binding_test.go)

The final Windows run of `go test -mod=readonly -count=1 -shuffle=on -cover ./model ./goeferr` exited 0 with 98.6% and 100% coverage, respectively. The runnable PostgreSQL example prints `ID bigint false` and `Label varchar(16) true`.

## Engine verification

S02 uses the generated type declarations in disposable tables on the local four-engine lab and compares the catalog and parameterized write/read results with the Go fixture. The round trip covers the tested input domain. Custom codecs, broader data types, schema generation and indexability retain their own acceptance checks. Exact decimal remains separately specified in S01.

The initial SQL Server 2022 fixture failed to read its catalog under the pinned case-sensitive collation. The fixture uses the catalog's uppercase names for that dialect. It also widens signed Go `int8` to `int16` when binding a parameter through go-mssqldb v1.11.0. That driver encodes a direct `int8` as unsigned `tinyint`. Using the `smallint` parameter preserves the negative boundary without changing the mapped storage type. [Parameter encoding](https://github.com/microsoft/go-mssqldb/blob/v1.11.0/mssql.go) · [TDS declaration names](https://github.com/microsoft/go-mssqldb/blob/v1.11.0/types.go)

The PostgreSQL 18.7 `varchar` declaration checks its type modifier against `MaxAttrSize`, which is `10 * 1024 * 1024`. [Declaration check](https://github.com/postgres/postgres/blob/REL_18_7/src/backend/utils/adt/varchar.c#L28-L62) · [Size definition](https://github.com/postgres/postgres/blob/REL_18_7/src/include/access/htup_details.h#L734-L742)

## Integrated verification

Revision `7796de91e1ddb573a589078916d831eb05605312` combines the provider binding, the database fixture and `Where` → `Select` → `Count`. The Windows/amd64 workstation used Go 1.27.4 on 2026-11-26. Independent reviewers matched the implementation and fixture trees to the reviewed lanes and checked the scope and traceability.

- `go test -mod=readonly -shuffle=on -count=1 -coverprofile=<profile> ./goeferr ./model ./linq/objects` exited 0. The tested packages covered 319 of 324 statements. Coverage was 12 of 12 for `goeferr`, 292 of 296 for `model` and 15 of 16 for `linq/objects`.
- The Windows/386 `go test -mod=readonly -count=1 -shuffle=on -cover -v ./linq/objects` run with `GOARCH=386` and `CGO_ENABLED=0` exited 0 with 16 of 16 statements covered. It exercised actual `Count` overflow and source cleanup. [Count evidence](../W008/OBJECTS_COUNT.md)
- All five complete programs in the Code First and LINQ Objects guides ran on the workstation with `go run -mod=readonly` and produced their documented output.

The initial `pwsh -NoProfile -File scripts/ci/goef-w002.ps1 -Compatibility -ModelTypes -AcceptSqlServerEula -AcceptLegacySqlServerEula` passed the bootstrap validator, 61 Python lab tests, Go build, vet and module checks. It exited 1 at the SQL Server fixtures before the catalog correction. The final executable revision `3ffc24a80a6208455065c0afdec141ed4049ceb0` changed only the SQL Server branch of the Go fixture and its redacted catalog error. The model and Objects trees match the earlier reviewed revision.

The workstation reran the complete engine matrix on `3ffc24a80a6208455065c0afdec141ed4049ceb0` using `driver_run.build_binary` once per target, `driver_run.run_provider(..., model_types=True)` on every pinned profile and `sqlite_compat.main(["--model-types"])` for the two linked SQLite runtimes and file cases. The rerun exited 0. There were 12 distinct S02 profile results, 84 distinct driver scenario results and four bidirectional SQLite file results. The current SQLite profile ran in both suites, producing 13 S02 rows and 91 driver rows in total. The catalog matched the bound types and all three synthetic rows preserved their signed boundaries, booleans, nullable text and bytes. [Local command and fixture](../../../test/conformance/driverlab/README.md) · [Go test](../../../test/conformance/driverlab/model_types_test.go)

| Provider | Verified S02 engine profiles | Go driver |
| --- | --- | --- |
| PostgreSQL | 16.16, 17.12, 18.7, 19.0 | pgx v5.13.0 |
| SQLite | 3.51.3, 3.54.1 | modernc.org/sqlite v1.47.0, v1.63.0 respectively |
| SQL Server Developer | 14.0.3570.2 at 140, 15.0.4510.2 at 150, 16.0.4325.4 at 140 and 160 | go-mssqldb v1.11.0 |
| MySQL | 8.0.46, 8.4.13.1 | go-sql-driver/mysql v1.10.0 |

On `3ffc24a80a6208455065c0afdec141ed4049ceb0`, the exact driver lab passed `go vet -mod=readonly ./...` on Windows. Race tests used network-isolated Go 1.27.4 Linux/amd64 containers on the same workstation. The root command `go test -mod=readonly -race -count=1 -shuffle=on ./...` passed all four packages. The previous cached vendor snapshot lacked the newly imported model package. The driver lab rerun used a fresh temporary snapshot from `go mod vendor -o <temp directory>` with the existing module pins and passed `go test -mod=vendor -race -count=1 -shuffle=on ./...` with exit 0.
