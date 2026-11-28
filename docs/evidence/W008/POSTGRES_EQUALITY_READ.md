# PostgreSQL typed equality read

Issue [#36](https://github.com/igorpooh1978/GoEF/issues/36). Development baseline `388e85cb27d3bebe9a4674b922e235658f647570`. Source checked at `9b8a7acdd342a647075f8a24dbce6f43dbb377cb`. Integrated acceptance remains pending.

## Source and scope

The pinned EF Core 10.0.15 compiler extracts parameters before using the compiled-query cache. Its `Where_simple` and `Where_simple_closure` cases exercise equality with a literal or captured value. GoEF uses a typed `EqualField` or generated binding, both lowered into one `EqualityRead`. The application owns the compiled plan and supplies a fresh typed argument to each execution. The terminal returns a detached result and a presence flag, or a typed error for a second match. GoEF uses a bounded two-row SQL read. The Go implementation and fixtures are original. [EF Core compiler](https://github.com/dotnet/efcore/blob/b34c0e9d7e768aa808ded44f19e7f871bea6869f/src/EFCore/Query/Internal/QueryCompiler.cs) [EF Core query tests](https://github.com/dotnet/efcore/blob/b34c0e9d7e768aa808ded44f19e7f871bea6869f/test/EFCore.Specification.Tests/Query/NorthwindWhereQueryTestBase.cs) [Pinned sources and licenses](../../upstream/lock.json)

PostgreSQL 18 documents exact quoted identifiers, a 63-byte identifier limit on the standard build, positional parameters and three-valued equality. The compiler rejects invalid UTF-8, embedded NUL and overlong names before using `pgx.Identifier.Sanitize`. The transport uses the pinned pgx `database/sql` adapter. PostgreSQL evaluates comparison and collation. [Release 18.7](https://www.postgresql.org/docs/18/release-18-7.html) [SQL lexical structure](https://www.postgresql.org/docs/18/sql-syntax-lexical.html) [Comparison operators](https://www.postgresql.org/docs/18/functions-comparison.html) [pgx v5.13.0 identifier implementation](https://github.com/jackc/pgx/blob/v5.13.0/conn.go) [pgx v5.13.0 database/sql adapter](https://github.com/jackc/pgx/blob/v5.13.0/stdlib/sql.go)

A standalone documentation lookup on 2026-11-28 at 07:30:50 UTC called `firecrawl_scrape` for the three linked official PostgreSQL pages. The endpoint was `https://mcp.firecrawl.dev/v2/mcp`. The server advertised `firecrawl-fastmcp` 3.42.0 and negotiated protocol `2025-06-18`. Each public URL returned a source status of 200, matching source and final URLs, and the expected exact-release heading or relevant topic. Direct vendor pages corroborated the result. Status `VERSION_MATCH_PASS` for the release and `DOC_LOOKUP_PASS` for syntax and comparison. The probe used only public documentation URLs and public search objectives.

## Executable verification

Go 1.27.4 and Python 3.14.4 ran on Windows amd64. The live launcher built the Go test binary for Linux amd64 and ran it in a disposable, network-isolated PostgreSQL 18.7 container using the pinned `postgres-18` profile. The fixture checked `server_version_num=180007`, pgx v5.13.0 using Exec mode, UTC, UTF8, C collation, default read committed and the private lab marker before writes. Its schema is synthetic.

| Check | Command and observed result |
| --- | --- |
| Baseline missing capability | `go test -run '^TestPostgresQueryableEquality$' .` in `test/conformance/driverlab` with the new fixture at baseline. Expected exit 1 because `linq/expressions` and its production execution path were absent. No database connection was attempted. |
| Core tests and vet | `go test -count=1 -shuffle=on -coverprofile=<temporary-file> ./...` and `go vet ./...` from the root, both exit 0. The compiler-negative test confirmed that a wrong row owner and numeric argument for a text plan are rejected by Go. |
| Provider tests and vet | `go test -count=1 -shuffle=on -coverprofile=<temporary-file> ./...` and `go vet ./...` from `providers/postgres`, both exit 0. The compiler SQL golden includes quoted punctuation and a Unicode identifier. |
| Query and expression coverage | `go test -count=1 -shuffle=on -coverpkg=./linq/queryable,./linq/expressions -coverprofile=<temporary-file> ./linq/queryable`, exit 0. Queryable 148/153 statements (96.7%). Expressions 2/2 (100%). |
| Module unit coverage | Root module 1742/1799 statements (96.8%), including the generated binding example. PostgreSQL module 37/38 statements (97.4%), including 12/12 compiler statements. |
| Linux race detector | `go test -race -shuffle=on -count=1 ./linq/... ./goeferr`, exit 0 in a cached `golang:1.27.4` Linux amd64 container with CGO enabled, no network and the source mounted read-only. Image digest `sha256:5f1b98fa02fe396ebce2a0227a0e08309e11792915b9dd2e1f71053d7e7e6666`. The Windows check exited 1 with CGO disabled. |
| Live engine | `python -B test/conformance/lab/postgres_queryable.py`, exit 0 with all three named scenarios passing after cleanup. |

| Scenario | Observed result |
| --- | --- |
| LINQ-PG-001-S01 | One compiled command matched two independently bound keys, one containing Unicode, an apostrophe and a backslash. The six-column projection preserved signed revisions, SQL NULL, timestamps to microsecond precision, a boolean and text. A missing key returned the zero result and a false presence flag. Command tracing observed three acquisitions and three dispatches with identical SQL and distinct bindings. |
| LINQ-PG-001-S02 | The repository's generated customer binding selected the scalar on PostgreSQL. A mapping type mismatch was rejected before acquisition. The unit test supplied a panicking local accessor and confirmed execution never called it. |
| LINQ-PG-001-S03 | A duplicate matched the registered cardinality error. The fixture observed a PostgreSQL lock wait before cancellation, received both the registered cancellation and its Go context cause, released the lock and read the expected value through the same caller-owned pool. Only the fixture writer remained checked out. Unit cases also observed late errors, materialization failures and ordered cleanup. |

The checked path is one non-null equality predicate, one schema-qualified PostgreSQL relation, and a detached scalar or struct projection. Other Queryable dialects, tracking, transaction-bound queries, PgBouncer performance and runtime EF Core differential comparison remain unverified for this path. Native UUID, JSONB, arrays and exact numeric mappings require separate codec work. [Conformance fixture](../../../test/conformance/driverlab/postgres_queryable_test.go) [Gherkin specification](../../../specs/features/linq/postgres-equality-read.feature) [User guide](../../user/LINQ_TO_ENTITIES.md)
