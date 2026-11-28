# PostgreSQL transactional migration executor

Delivery scope [#35](https://github.com/igorpooh1978/GoEF/issues/35).
Baseline `388e85cb27d3bebe9a4674b922e235658f647570`. Implementation checked at
`a16f941579d6fc09fc22a5a759c60cde6add5f08` on 2026-11-28 UTC. The
[Go guide](../../user/MIGRATIONS_POSTGRESQL.md) describes the API and operator
response to typed failure.

## Upstream and provider behavior

- EF Core v10.0.15 at `b34c0e9d7e768aa808ded44f19e7f871bea6869f` (MIT)
  acquires the migration lock before loading applied migrations. The
  relational migration infrastructure tests cover concurrent and range
  application. Npgsql v10.0.4 at
  `fe1109ecf52646ed6fc718a9a233353ef2e7baf9` (PostgreSQL License) locks the
  history relation with an access-exclusive table lock. GoEF takes a
  cooperating advisory lock before creating its exact-digest history and
  holds it through the complete transactional suffix. The implementation
  uses independently authored Go and SQL. [EF Core migrator](https://github.com/dotnet/efcore/blob/b34c0e9d7e768aa808ded44f19e7f871bea6869f/src/EFCore.Relational/Migrations/Internal/Migrator.cs) · [EF Core tests](https://github.com/dotnet/efcore/blob/b34c0e9d7e768aa808ded44f19e7f871bea6869f/test/EFCore.Relational.Specification.Tests/Migrations/MigrationsInfrastructureTestBase.cs) · [Npgsql repository](https://github.com/npgsql/efcore.pg/blob/fe1109ecf52646ed6fc718a9a233353ef2e7baf9/src/EFCore.PG/Migrations/Internal/NpgsqlHistoryRepository.cs)
- PostgreSQL 18 releases a transaction-level advisory lock when the
  transaction ends. READ COMMITTED gives the subsequent history statement a
  fresh snapshot after waiting. PostgreSQL rejects a concurrent index build
  in a transaction block. [PostgreSQL 18.7](https://www.postgresql.org/docs/release/18.7/) · [Advisory locks](https://www.postgresql.org/docs/18/explicit-locking.html#ADVISORY-LOCKS) · [Isolation](https://www.postgresql.org/docs/18/transaction-iso.html) · [Index transaction restriction](https://www.postgresql.org/docs/18/sql-createindex.html#SQL-CREATEINDEX-CONCURRENTLY)
- pgx v5.13.0 only uses the begin context for the begin command. GoEF owns
  explicit bounded rollback after a caller cancellation. pgxpool discards
  failed, non-idle or closed connections. On an ambiguous commit GoEF takes
  ownership of the checkout and closes the physical connection. [Driver transaction source](https://github.com/jackc/pgx/blob/v5.13.0/tx.go) · [Driver pool release](https://github.com/jackc/pgx/blob/v5.13.0/pgxpool/conn.go)

The standalone public documentation MCP lookup passed at 07:27:42 UTC
(`DOC_LOOKUP_PASS`). A Python 3.14.4 client initialized
`https://mcp.firecrawl.dev/v2/mcp` over Streamable HTTP, negotiated
`2025-06-18` with `firecrawl-fastmcp` `3.42.0` and discovered
`firecrawl_scrape`. It sent the official PostgreSQL 18 exclusive/advisory
locking manual URL and a public objective about §13.3.5. The response
reported HTTP 200 for the exact source and corroborated expiration at
transaction end. The release pin remains tied to the
[exact-release lookup](../W000/MCP_EXACT_RELEASES.md) and server identity.
Project-client configuration was outside this standalone check.

## Verification

The runner used Docker Desktop engine 29.4.0 and the PostgreSQL Linux amd64
image `docker.io/library/postgres@sha256:32f5fdbf4d27ebe0d4552313126a593400f40b21bff207e43617b2305838bce3`.
It checked PostgreSQL 18.7, `server_version_num=180007`, and a run-specific
target marker before DDL. The binary used Go 1.27.4 and pgx v5.13.0.
The [isolated runner](../../../test/conformance/lab/postgres_migrations.py)
exports the scenario markers and statement numerator/denominator.

| Command | Exit | Observation |
| --- | --- | --- |
| `python -B test/conformance/lab/postgres_migrations.py` on baseline plus API declaration and tests | 1 (expected) | The native fixture reached the application assertion in S01. The unimplemented API returned zero instead of committing the first artifact. |
| `python -B test/conformance/lab/postgres_migrations.py` at the implementation commit | 0 | 6/6 scenarios passed on PostgreSQL 18.7. Migration package statement coverage is 134/140 (95.71%) in the instrumented native run. |
| `go test -mod=readonly -shuffle=on ./...` and `go vet -mod=readonly ./...` from `providers/postgres` | 0 each | Provider module tests and vet passed on Windows amd64. |
| `go test -mod=readonly -shuffle=on ./...` and `go vet -mod=readonly ./...` from `test/conformance/driverlab` | 0 each | Driverlab compilation and default checks passed on Windows amd64. The dedicated runner executed the migration scenarios. |

| Scenario | Durable and lifecycle observation |
| --- | --- |
| MIG-POSTGRES-001-S01 | Apply the first artifact, then extend to the second. The result counts were 1, 1 and 0. History matched positions, IDs, predecessors and 32-byte digests. The final target check verified the seed row. |
| MIG-POSTGRES-001-S02 | Invalid plans, wrong database, missing schema, changed digest, history gaps, excess history, missing read permission and a history table containing a null legacy digest were rejected. Source and target checks also blocked dispatch or replay. |
| MIG-POSTGRES-001-S03 | DDL and DML before failed execution or a deferred constraint rejection left no schema, data or history effects. Corrupted history during an artifact also prevented commit. A later explicit call applied the corrected plan exactly once. |
| MIG-POSTGRES-001-S04 | Server lock state showed a second backend waiting. The first committed both artifacts and the waiter returned zero with one seed row and two history rows. |
| MIG-POSTGRES-001-S05 | Caller cancellation, the Go lock budget and PostgreSQL's native lock timeout preserved the holder's lock and left no waiter history. Server statement timeout retained the generic driver classification. |
| MIG-POSTGRES-001-S06 | A local transport shim withheld a real PostgreSQL commit response. Apply returned `ErrCommitOutcomeUnknown`, quarantined the checkout, and dispatched no retry. A fresh backend and an independent pool confirmed the exact history and seed row. |

The [Gherkin specification](../../../specs/features/migrations/mig-postgres-001.feature)
and [native test bindings](../../../test/conformance/driverlab/postgres_migrations_test.go)
cover the transactional forward subset. Cross-process orchestration,
automatic legacy baseline import, nontransactional crash recovery, other
PostgreSQL versions and PgBouncer remain outside the checked scope.
Independent integration review and whole-provider coverage remain under the
release gate.
