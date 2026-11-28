# Physical session reuse across providers

[Issue #39](https://github.com/igorpooh1978/GoEF/issues/39) · [Scenarios](../../../specs/features/providers/provider-reuse-001.feature) · [Test entry point](../../../test/conformance/driverlab/production_adapter_test.go) · [Session assertions](../../../test/conformance/driverlab/production_session_reuse_test.go)

## Baseline and observed behavior

The driver-lab tests use a one-connection caller-owned pool over the exact native connector from each pinned fixture. The connector counts each successful physical open. It returns the native connection unchanged. Fixture setup uses independent writer and observer leases. The version and isolated target identity are checked before a seed row is inserted.

At `4183f1da710533890b0a5ee680978652255d60d6` with only the new tests added, the ordinary-read negative control opened exactly one replacement after successful typed streaming and `Reuse` on SQLite 3.54.1, MySQL 8.4.13.1, and SQL Server 16.0.4325.4. That control exited 0 on each engine. Reviewed-read and explicit-discard probes exited 1 on each because the optional interface was absent. For the original PostgreSQL baseline and the wrapped pgx pool, see the [separate PostgreSQL evidence](POSTGRES_SESSION_REUSE.md).

Enabling the optional interface on the three constructors produced zero physical opens on the next checkout after a verified read. The caller observed the bound result, reached EOF and closed the stream. An ordinary query, prior session mutation, incomplete stream, canceled stream, native query failure, and explicit discard each opened exactly one replacement. The settings written to the discarded session were absent on the replacement. The test observed causal cancellation, no outstanding lease, and a working caller pool after release in each case.

## Live results

The reference command below exited 0 with seven exact adapter scenario records for each engine, including all three session-reuse scenarios. Host Go was 1.27.4 on Windows/amd64. SQLite ran on Windows. The other engines ran inside the lab's pinned Linux/amd64 containers. The fixture used the actual engine, pinned driver and settings from the [profile registry](../../../test/conformance/lab/driver_run.py).

```sh
python -B test/conformance/lab/driver_run.py all --production-adapters --accept-sqlserver-eula
python -B test/conformance/lab/sqlite_compat.py --production-adapters
```

The compatibility command also exited 0, with all three reuse scenarios passing on both SQLite versions. These first runs used the test lane with the three constructor flags applied locally. The integrated acceptance run records its own exact commit and results.

| Engine | Driver | Reuse scenarios passed | Covered adapter statements | Covered shared transport statements |
| --- | --- | ---: | ---: | ---: |
| PostgreSQL 18.7 | pgx v5.13.0 | 3/3 | 13/26 | 128/196 |
| SQLite 3.51.3 | modernc.org/sqlite v1.47.0 | 3/3 | 11/22 | 127/196 |
| SQLite 3.54.1 | modernc.org/sqlite v1.63.0 | 3/3 | 11/22 | 127/196 |
| SQL Server 2022 CU28 16.0.4325.4, level 160 | go-mssqldb v1.11.0 | 3/3 | 19/30 | 127/196 |
| MySQL Community 8.4.13.1 | go-sql-driver/mysql v1.10.0 | 3/3 | 10/24 | 128/196 |

The coverage counts apply to the live adapter and shared transport packages reported by the lab. Module unit tests and other profile results are recorded separately. The non-PostgreSQL runs count physical opens. Request latency under application load was unmeasured.

## Versioned source review

On 2026-11-28 UTC, the Firecrawl Streamable HTTP documentation endpoint at `https://mcp.firecrawl.dev/v2/mcp` advertised `firecrawl-fastmcp` 3.42.0 and negotiated `2025-06-18`. Anonymous `firecrawl_scrape` lookups matched the public release identifiers and session-setting behavior in the following sources.

- SQLite `PRAGMA cache_size` applies for the open session and reverts on close. [SQLite 3.51.3](https://www.sqlite.org/releaselog/3_51_3.html) · [SQLite 3.54.1](https://www.sqlite.org/releaselog/3_54_1.html) · [Cache-size pragma](https://www.sqlite.org/pragma.html#pragma_cache_size) · [Pinned older driver](https://pkg.go.dev/modernc.org/sqlite@v1.47.0) · [Pinned lab driver](https://pkg.go.dev/modernc.org/sqlite@v1.63.0)
- MySQL user-defined variables belong to the current session. The pinned Go driver's reset checks connection liveness. Its close terminates the connection. [MySQL 8.4.13.1](https://dev.mysql.com/doc/relnotes/mysql/8.4/en/news-8-4-13-1.html) · [User-defined variables](https://dev.mysql.com/doc/refman/8.4/en/user-variables.html) · [Pinned driver source](https://github.com/go-sql-driver/mysql/blob/v1.10.0/connection.go)
- SQL Server `SET CONTEXT_INFO` sets state on the current session. The pinned Go driver's close terminates the TDS transport. [SQL Server 2022 CU28](https://support.microsoft.com/en-us/servicing/sql/sql-server-2022/cumulative-update/kb5125472-cu28) · [SET CONTEXT_INFO](https://learn.microsoft.com/en-us/sql/t-sql/statements/set-context-info-transact-sql?view=sql-server-ver16) · [Pinned driver source](https://github.com/microsoft/go-mssqldb/blob/v1.11.0/mssql.go)
