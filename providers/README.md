# GoEF SQL execution adapters

The [SQLite example](sqlite/example_test.go) runs on Go 1.27.4 against a temporary file. It reads a bound value and handles an incompatible result layout through `errors.Is`.

Issue [#13](https://github.com/igorpooh1978/GoEF/issues/13) implements the frozen [execution SPI](../docs/architecture/PROVIDER_SPI.md) over the pinned database/sql drivers. Each provider is a separate Go module. An application imports the selected provider and owns its pool.

| Provider | Module subdirectory | Driver selection |
| --- | --- | --- |
| PostgreSQL | `providers/postgres` | pgx stdlib v5.13.0 |
| SQLite | `providers/sqlite` | modernc.org/sqlite v1.47.0 minimum; v1.63.0 in the reference build |
| Microsoft SQL Server | `providers/sqlserver` | go-mssqldb v1.11.0 |
| MySQL/InnoDB | `providers/mysql` | Go MySQL Driver v1.10.0 |

`New(db)` adapts an existing pool built with the named driver. `Open(dsn)` constructs a pool without acquiring a connection. PostgreSQL `Open` uses pgx exec mode. MySQL `Open` enables temporal parsing. Use equivalent native options when the application constructs a pool. For SQLite, put required per-connection pragmas in the driver URI, for example `file:example.db?_pragma=foreign_keys(1)`. Use an on-disk database or maintain an independent anchor connection for a shared in-memory database.

Acquire a lease with a live context, pass separate typed parameters and declare the output columns in order. When using named SQL Server parameters, give each slot the name of the placeholder without the leading `@`. An explicit `provider.Null{}` differs from an empty `provider.Binary`. The adapter supports one result layout and rejects unexpected additional result sets. Nullable columns accept database NULL. MySQL and SQLite read a driver integer of 0 or 1 as a Boolean when the layout requests it. Temporal columns require the driver to supply `time.Time`.

Use the lease serially. The query context governs opening and reading the result. A context-defined cause remains discoverable with `errors.Is`, and its text stays out of the message. Close streams before releasing the lease. `database/sql` exposes synchronous result close, so driver cleanup finishes before the lease returns. A connection that executed ordinary `Query` or `Exec` is retired because arbitrary authorized SQL can affect session state. An unused lease can be returned with `provider.Reuse`. Releasing never closes the caller's pool. When finished with the application, close `backend.DB()`.

The [owned transaction API](../docs/user/TRANSACTIONS.md) accepts the same pool through `backend.DB()`. Transaction ownership and savepoints follow that API. For PostgreSQL these transactions require a direct pgx stdlib pool, such as the one returned by postgres.Open. Wrapping a separate native pgx pool with stdlib.OpenDBFromPool is outside the transaction contract because managed transactions cannot invoke the physical-close hook. Querying with a new SPI lease acquires a separate connection.

## Reuse after a reviewed read

All four adapters in current source expose `provider.SessionNeutralQuerier` on acquired leases. Use `QuerySessionNeutral` for a read you have verified leaves no session state after execution. Review the SQL and all database objects it invokes. Examples of relevant state include PostgreSQL session settings and advisory locks, MySQL user variables, SQL Server session context, and SQLite per-connection pragmas and temporary objects.

Read through `io.EOF`, close the rows, and release with `provider.Reuse` using a live cleanup context. An earlier ordinary command on the same lease, failure, cancellation, incomplete iteration, open rows on release, or explicit `Discard` can require physical replacement. Pool capacity, lifetimes and driver health checks still apply. The PostgreSQL guide covers the [native-pool wrapper](../docs/providers/POSTGRESQL.md#reuse-after-a-reviewed-read).

Native error mappings depend on structured driver codes. SQL Server error 547 can be either a foreign-key or check violation, so the adapter exposes code 547 with `ErrUnknownDriver`. The public error exposure contains registered detail and safe codes. The adapter never unwraps vendor error text to the caller or replays a statement on another connection.

## Documentation lookup

On 2026-11-26 at 16:18 UTC, a standalone, public-document-only Streamable HTTP probe negotiated protocol `2025-06-18` with Context7 4.9.0 and Microsoft Learn MCP Server 1.0.0. The probe used `query-docs` and `microsoft_docs_fetch`. The implementation uses the tagged driver sources and the official engine references for the exact mappings.

| Provider | Documentation probe and corroborating evidence |
| --- | --- |
| PostgreSQL | Context7 `/websites/postgresql/18` surfaced 23505, 40P01 and 40001. The versioned vendor page supplied 23503 and the complete code list for PostgreSQL 18.7. [Appendix A](https://www.postgresql.org/docs/18/errcodes-appendix.html) · [pgx v5.13.0 stdlib](https://github.com/jackc/pgx/blob/v5.13.0/stdlib/sql.go) |
| SQLite | Context7 `/websites/sqlite_docs` returned 1555, 787 and 517 from the SQLite code list. Exact runtime provenance is in the [3.51.3 and 3.54.1 evidence](../docs/evidence/W002/LEGACY_COMPATIBILITY.md). [Code list](https://www.sqlite.org/rescode.html) · [v1.47.0 Go driver](https://pkg.go.dev/modernc.org/sqlite@v1.47.0) |
| SQL Server | Microsoft Learn fetched the SQL Server 2022 code tables for uniqueness and deadlocks. [2601 and 2627](https://learn.microsoft.com/en-us/sql/relational-databases/errors-events/database-engine-events-and-errors-2000-to-2999?view=sql-server-ver16) · [1205](https://learn.microsoft.com/en-us/sql/relational-databases/errors-events/mssqlserver-1205-database-engine-error?view=sql-server-ver16) · [go-mssqldb v1.11.0](https://github.com/microsoft/go-mssqldb/tree/v1.11.0) |
| MySQL | Context7 `/websites/dev_mysql_doc_refman_en/8.4` surfaced 1205 and 1213 from the InnoDB manual. Oracle's versioned error reference supplied 1062 and the full selected code list. [Oracle 8.4 error reference](https://dev.mysql.com/doc/mysql-errors/8.4/en/server-error-reference.html) · [Go MySQL Driver v1.10.0](https://github.com/go-sql-driver/mysql/tree/v1.10.0) |
