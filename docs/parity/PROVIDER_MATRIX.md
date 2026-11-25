# Required provider matrix

Tracked by [W000 #1](https://github.com/igorpooh1978/GoEF/issues/1). This is the W000 plan for **four mandatory real-engine cells**, with provisional documentation candidates for W001 selection. Runtime conformance has passed in **0 of 4** cells. No test image, package dependency, exact runner or driver operating mode is approved by this matrix.

| Engine and provisional version | Candidate Go driver | Required fixture and identity | Documentation MCP smoke | Runtime |
| --- | --- | --- | --- | --- |
| [PostgreSQL 18.7](../providers/POSTGRESQL.md) | `github.com/jackc/pgx/v5` v5.13.0 | Server with pinned image digest, disposable database/schema, server and driver version, search path, collation and time zone | Context7 falls back to older 18.x content. The [exact 18.7 tagged source](../evidence/W000/MCP_POSTGRES_RECHECK.md) was read through GitHub MCP. Dedicated documentation route remains blocked [#10](https://github.com/igorpooh1978/GoEF/issues/10). | NOT RUN |
| [SQLite 3.54.1](../providers/SQLITE.md) | `modernc.org/sqlite` v1.63.0 | Actual embedded SQLite source ID and compile options, isolated file, separate connections, WAL and rollback journal coverage, explicit FK settings | Lookup and scoped exact source identity passed [#8](https://github.com/igorpooh1978/GoEF/issues/8). | NOT RUN |
| [SQL Server 2022 CU28 (16.0.4325.4)](../providers/SQLSERVER.md) | `github.com/microsoft/go-mssqldb` v1.11.0 | Supported architecture and edition, pinned server artifact, disposable database, compatibility level, collation and isolation settings | Lookup and scoped exact build identity passed [#6](https://github.com/igorpooh1978/GoEF/issues/6). | NOT RUN |
| [Oracle MySQL 8.4.13.1 LTS](../providers/MYSQL.md) | `github.com/go-sql-driver/mysql` v1.10.0 | Exact release and artifact digest, disposable InnoDB database, SQL mode, collation, time zone and isolation settings | Context7 returns 8.4. Oracle identifies the exact update directly. The [MCP recheck](../evidence/W000/MCP_MYSQL_RECHECK.md) remains blocked [#11](https://github.com/igorpooh1978/GoEF/issues/11). | NOT RUN |

## Acceptance per version

| Suite | Required proof | W000 execution |
| --- | --- | --- |
| Shared runtime and query | Real CRUD and materialization, key generation, parameter binding, query semantics, typed unsupported behavior, comparison against reference cases where feasible | 0/4 |
| Transactions and errors | Commit/rollback state, isolation, connection affinity, external ownership and savepoints, cancellation and cleanup, native constraint/lock classifications and ambiguous outcome behavior | 0/4 |
| Migrations | Library API, Up/Down or explicit irreversible handling, drift and checksums, locking, interruption/recovery and persisted schema, data and history | 0/4 |
| Provider-specific contracts | PostgreSQL concurrent DDL, SQLite rebuild, SQL Server transaction states and MySQL implicit commits, plus version/capability negatives | 0/4 |

Record the exact integrated SHA, environment, engine/driver identity and settings, Gherkin IDs, executable test and result for every claimed capability. Inspect database state as well as emitted SQL. The minimum statement coverage is 95% in each owned executable module and provider. Measure the denominator when implementation exists. Track behavioral, negative, race, fault, security, and performance evidence separately, including any reviewed capability exception and its test. Claims roll up only when each advertised engine/version and driver cell has passed.

Use the [provider development workflow](../providers/PROVIDER_DEVELOPMENT.md) for coverage details and the [MCP report](../evidence/W000/MCP_REPORT.md) for the source lookup evidence. See the [W002 lab design](../development/CONFORMANCE_LAB.md) and [candidate artifact evidence](../evidence/W002/TEST_LAB_DESIGN.md) for image digests, driver sums and planned run gates.
