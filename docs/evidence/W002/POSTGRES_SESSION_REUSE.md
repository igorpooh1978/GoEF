# PostgreSQL session reuse after a reviewed read

[Issue #39](https://github.com/igorpooh1978/GoEF/issues/39) · [Scenarios](../../../specs/features/providers/provider-reuse-001.feature) · [Executable test](../../../test/conformance/driverlab/postgres_session_reuse_test.go)

The adoption review found that the first alpha retired the physical session after every dispatched `Query` and `Exec`. `Reuse` only returned a lease that had never dispatched SQL. This protected subsequent checkouts from session state left by arbitrary SQL. The original adapter round trip checked available pool slots after release and performed a write on the same lease, so it did not expose reconnection after a successful read-only workload.

In the `388e85cb27d3bebe9a4674b922e235658f647570` baseline with only the characterization test added, the two plain `Query` controls passed. Both the neutral SQL and a `SELECT` using session-level `set_config` obtained a different physical PID after `Reuse`. The requested reviewed read failed with `lease does not provide session-neutral reads`.

With the optional capability implemented, all cases passed on PostgreSQL 18.7 (`server_version_num=180007`), pgx v5.13.0 in `QueryExecModeExec`, and Go 1.27.4. The fixture used Linux/amd64 in the isolated lab with a SCRAM login, TCP loopback, UTC, UTF-8, C collation, Read Committed and `public` search path. The [pinned image](../../../test/conformance/lab/postgres.py) had digest `sha256:32f5fdbf4d27ebe0d4552313126a593400f40b21bff207e43617b2305838bce3`.

Both the direct `database/sql` pool and `stdlib.OpenDBFromPool` preserved the backend PID when the caller vetted the read and consumed it to EOF before closing. `Query` with the same SQL, a session mutation, cancellation, a command error, materialization error, an incomplete stream and explicit discard led to PID replacement. The fixture also checked that a later vetted read could not undo an earlier executed command's discard requirement, that release with open rows reported `ErrContextBusy`, and that the native pool remained available after wrapper close.

Run `python -B test/conformance/lab/postgres_session_reuse.py --benchmark`. The microbenchmark compares three dispatch paths for `SELECT pg_backend_pid(), $1::text, NULL::bigint` over the same native pgx pool with one connection, 150 operations per path and three repeats. The first passing run exited 0.

| Path | Time per operation | Allocations per operation | New physical connections per operation |
| --- | --- | --- | --- |
| Native `pgxpool.QueryRow` | 0.305–0.533 ms | 18–19 | Unmeasured in this control |
| GoEF ordinary `Query` and `Reuse` | 6.367–8.530 ms | 315–316 | 0.993 after one initial warm checkout |
| GoEF `QuerySessionNeutral` and `Reuse` | 0.256–0.296 ms | 55 | 0 |

The fixture uses synthetic values and a local database. Application-level latency and PgBouncer behavior remain unmeasured.

## Source review

On 2026-11-28 UTC, Firecrawl's standalone HTTPS MCP endpoint `https://mcp.firecrawl.dev/v2/mcp` advertised `firecrawl-fastmcp` 3.42.0 and negotiated `2025-06-18`. Anonymous `firecrawl_scrape` calls returned matching release and session-function content. PostgreSQL permits session-level effects from a `SELECT`, including `set_config(..., false)` and advisory locks. The API therefore relies on review of the whole SQL operation and the objects it invokes. The pinned pgx stdlib closes a wrapper lease by returning it to the native pool, and pgxpool destroys closed, busy or non-idle transaction connections. GoEF explicitly closes the native session when it requires discard. [PostgreSQL 18.7 release](https://www.postgresql.org/docs/18/release-18-7.html) · [System administration functions](https://www.postgresql.org/docs/18/functions-admin.html) · [pgx stdlib](https://github.com/jackc/pgx/blob/v5.13.0/stdlib/sql.go) · [pgxpool release](https://github.com/jackc/pgx/blob/v5.13.0/pgxpool/conn.go)

EF Core's pinned `RelationalConnection.Close` delegates ordinary logical closing to the configured driver connection and preserves ownership boundaries. That model is an analogue for resource ownership. GoEF's optional assertion applies to the SQL execution lease. No upstream code was reused. [EF Core v10.0.15 relational connection](https://github.com/dotnet/efcore/blob/b34c0e9d7e768aa808ded44f19e7f871bea6869f/src/EFCore.Relational/Storage/RelationalConnection.cs)
