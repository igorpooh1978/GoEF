# W000 exact PostgreSQL and MySQL documentation check

Tracking [#10](https://github.com/igorpooh1978/GoEF/issues/10) and [#11](https://github.com/igorpooh1978/GoEF/issues/11). On 2026-11-26 UTC, a standalone Firecrawl MCP probe returned the exact PostgreSQL 18.7 and Oracle MySQL 8.4.13.1 release notes and the applicable manual sections. Both documentation scopes reached `VERSION_MATCH_PASS`. GoEF baseline was `dc13ff64370f7e7348f0f7958dbf996819177a90`. The check uses public documentation. It leaves project client configuration at `NOT_CONFIGURED` and adds no runtime conformance evidence. [MCP policy](../../development/MCP_POLICY.md)

## Transport and scope

The Firecrawl publisher documents an anonymous Streamable HTTP endpoint at `https://mcp.firecrawl.dev/v2/mcp`. The public Firecrawl MCP server source is MIT-licensed. The hosted server advertised `firecrawl-fastmcp` version `3.41.0`, which identifies the observed service. The public source and its release and security pages were reviewed for this probe. The audit scope covers the published source and the live protocol surface. Hosted internals and the mapping between the service build and public source are outside the scope. [Publisher documentation](https://docs.firecrawl.dev/mcp-server) · [Repository and license](https://github.com/firecrawl/firecrawl-mcp-server) · [Releases](https://github.com/firecrawl/firecrawl-mcp-server/releases) · [Security page](https://github.com/firecrawl/firecrawl-mcp-server/security)

A Python 3.14.4 standard-library client called `initialize`, `notifications/initialized`, and `tools/list` at 15:02:43 and 15:03:50 UTC. The server negotiated protocol `2025-06-18`. Initialize and tool discovery returned HTTP 200 event streams. The notification returned HTTP 202. The endpoint used stateless requests without a session header. The client used `MCP-Protocol-Version: 2025-06-18` after initialization and matched event messages by JSON-RPC request ID. Both client processes exited 0. [MCP transport](https://modelcontextprotocol.io/specification/2025-06-18/basic/transports)

Discovery listed `firecrawl_scrape`, `firecrawl_search`, and `firecrawl_parse`. Only `firecrawl_scrape` was used, for the exact HTTPS vendor URLs below. The live input schema required `url` and `objective`. The tool advertised `readOnlyHint: true`, `destructiveHint: false`, and `openWorldHint: true`. The client used a fixed list of public URLs, anonymous requests, and a public documentation objective. It ran standalone with the Python standard library.

To repeat the check, use JSON-RPC `tools/call` with `name: "firecrawl_scrape"` and these arguments. Replace `<vendor URL>` only with a listed source. Pass the returned conversation ID if the service supplies one for subsequent calls.

```json
{
  "url": "<vendor URL>",
  "formats": ["markdown"],
  "onlyMainContent": true,
  "objective": "Verify the exact versions and documented transaction boundaries in public database vendor documentation.",
  "storeInCache": false,
  "domainTools": false,
  "skipTlsVerification": false,
  "timeout": 25000
}
```

## Observed matches

All seven calls in the expanded probe returned HTTP 200 event streams and successful MCP tool results. Each reported `structuredContent` with a Markdown body and metadata. For every call, the reported source URL and final URL matched the requested document, and the source status was 200. The release headings, dates, and documented behavior were compared with direct vendor pages.

| Source | Matched result |
| --- | --- |
| [PostgreSQL 18 release note for 18.7](https://www.postgresql.org/docs/18/release-18-7.html) | Section E.1 identifies the exact release as 18.7, dated 2026-11-12. |
| [PostgreSQL 18 CREATE INDEX](https://www.postgresql.org/docs/18/sql-createindex.html) | The concurrent index section allows a regular index build in a transaction block and requires a separate transaction boundary for `CREATE INDEX CONCURRENTLY`. |
| [Oracle MySQL 8.4.13.1 release note](https://dev.mysql.com/doc/relnotes/mysql/8.4/en/news-8-4-13-1.html) | The release note identifies 8.4.13.1, dated 2026-11-24, as a Critical Security Patch Update. |
| [Oracle MySQL 8.4 implicit commits](https://dev.mysql.com/doc/refman/8.4/en/implicit-commit.html) | Section 15.3.3 names `ALTER TABLE` and ordinary `CREATE TABLE` among DDL statements that commit an existing transaction before execution. |
| [PostgreSQL 18 transaction isolation](https://www.postgresql.org/docs/18/transaction-iso.html) | Section 13.2 covers Read Committed and Serializable. It describes serialization failures and restarting the whole transaction. |
| [PostgreSQL 18 SQLSTATE appendix](https://www.postgresql.org/docs/18/errcodes-appendix.html) | Class 40 lists `40001` for serialization failure and `40P01` for deadlock detection. |
| [Oracle MySQL 8.4 InnoDB isolation](https://dev.mysql.com/doc/refman/8.4/en/innodb-transaction-isolation-levels.html) | Section 17.7.2.1 identifies `REPEATABLE READ` as the InnoDB default and describes its snapshot on the first consistent read. |

## Provider implications

PostgreSQL migration execution needs a separate boundary for concurrent index commands. The database determines isolation semantics and native error identity. The pinned pgx v5.13.0 driver exposes SQLSTATE on `PgError`, so classification can preserve that identity and separately account for transaction phase. [CREATE INDEX concurrent section](https://www.postgresql.org/docs/18/sql-createindex.html#SQL-CREATEINDEX-CONCURRENTLY) · [pgx error type](https://github.com/jackc/pgx/blob/v5.13.0/pgconn/errors.go) · [PostgreSQL guide](../../providers/POSTGRESQL.md)

MySQL migration recovery must check persisted schema and journal state after a failure across implicit commit boundaries. The go-sql-driver/mysql v1.10.0 cancellation behavior can close the connection when cancellation precedes a result. Read persisted state to determine the actual DDL outcome after that failure. Record the exact four-part engine identity with `VERSION_WITH_UPDATE()` in a real run. [MySQL driver context support](https://github.com/go-sql-driver/mysql/blob/v1.10.0/README.md#contextcontext-support) · [MySQL guide](../../providers/MYSQL.md) · [Migration recovery scenario](../../../specs/features/migrations/mig-mysql-001.feature)

Context7 was checked again at 15:00 UTC. It still advertised version 4.7.3 and protocol `2025-06-18`. It served the 18.6 release index after falling back from PostgreSQL 18.7 to major 18. For MySQL 8.4.13.1 it returned family documentation about four-part version numbers instead of the Oracle release note. The standalone exact-source results above fulfill the version and topic evidence for #10 and #11. Conformance and application proof remain under [#13](https://github.com/igorpooh1978/GoEF/issues/13). [PostgreSQL Context7 history](MCP_POSTGRES_RECHECK.md) · [MySQL Context7 history](MCP_MYSQL_RECHECK.md)
