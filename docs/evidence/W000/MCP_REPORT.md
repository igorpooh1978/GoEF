# W000 provider documentation MCP evidence

Tracking [W000 #1](https://github.com/igorpooh1978/GoEF/issues/1). Probes took place on 2026-11-25 UTC against baseline `2a886c78e622f583fa2980ea0a61a7e328a40e28`. In the original run, documentation transport and topic lookups passed for all four providers. Exact release provenance passed for SQLite and SQL Server. GoEF engine behavior, permissions and enabled project client operation were outside that run.

The 2026-11-26 [exact-release follow-up](MCP_EXACT_RELEASES.md) passed for the PostgreSQL and MySQL documentation scopes through a standalone Firecrawl MCP probe limited to public vendor pages. The previous [PostgreSQL](MCP_POSTGRES_RECHECK.md) and [MySQL](MCP_MYSQL_RECHECK.md) rechecks record the Context7 fallback and corroborating tagged source checks.

## Method and observed transport

Checked the publisher documentation, maintenance and advisory surfaces, endpoint and live input schemas. [Context7 Codex documentation](https://github.com/upstash/context7/blob/master/docs/clients/codex.mdx) · [Context7 releases](https://github.com/upstash/context7/releases) · [Context7 security](https://github.com/upstash/context7/security) · [Microsoft Learn developer reference](https://learn.microsoft.com/en-us/training/support/mcp-developer-reference) · [Microsoft Learn release notes](https://learn.microsoft.com/en-us/training/support/mcp-release-notes)

A one-shot Python 3.14.4 standard-library probe used the published [MCP Streamable HTTP transport](https://modelcontextprotocol.io/specification/2025-06-18/basic/transports). It posted JSON-RPC `initialize` with empty client capabilities, proposed protocol `2025-06-18`, and requested JSON and SSE. After negotiation it posted `notifications/initialized`, `tools/list` and allowlisted `tools/call` requests using the negotiated version and session header when issued. It read streamed replies by request ID. It sent only public documentation queries and public vendor URLs. No credentials, database queries, fixture contents or client environment were included.

| Endpoint | Observed server | Initialize / notification / list | Exposed tools |
| --- | --- | --- | --- |
| `https://mcp.context7.com/mcp` | Context7 4.7.3, protocol `2025-06-18` | 200 SSE / 202 / 200 SSE, no session header issued | `resolve-library-id`, `query-docs` |
| `https://learn.microsoft.com/api/mcp` | Microsoft Learn MCP Server 1.0.0, protocol `2025-06-18` | 200 SSE / 202 / 200 SSE, session header issued and reused | `microsoft_docs_search`, `microsoft_docs_fetch`, `microsoft_code_sample_search`. The GoEF probe allowlist omitted the code sample tool. |

Codex CLI 0.162.1 advertised Streamable HTTP via `codex mcp add --help`. Read-only `codex mcp get --json` with per-process `-c` overrides confirmed that `url`, `enabled = false` and `enabled_tools` produce a disabled Streamable HTTP MCP entry. The [example configuration](../../../.codex/config.example.toml) contains disabled entries. Neither endpoint was enabled in a Codex configuration and the isolated probes did not install any package. See the [MCP policy](../../development/MCP_POLICY.md) for evidence definitions.

## PostgreSQL

[Issue #10](https://github.com/igorpooh1978/GoEF/issues/10) · Candidate 18.7 · `HANDSHAKE_PASS` · `DOC_LOOKUP_PASS` · Exact release `BLOCKED`.

At 17:25–17:27 UTC, Context7 `resolve-library-id` identified `/websites/postgresql` and listed major version 18. `query-docs` on `/websites/postgresql/18` for the transaction restriction of concurrent index builds returned the official PostgreSQL 18 CREATE INDEX section. It agrees with the vendor on the requirement to run the concurrent operation outside a transaction block. The request for exact 18.7 release notes returned the PostgreSQL 18 release appendix with 18.6 at the top. The vendor's release archive independently identifies 18.7 and gives its release date as 2026-11-12. Use that archive and the major-version manual. Recheck Context7 for 18.7 provenance or evaluate the documentation adapter described in the policy. [Concurrent index section](https://www.postgresql.org/docs/18/sql-createindex.html#SQL-CREATEINDEX-CONCURRENTLY) · [18.7 release notes](https://www.postgresql.org/docs/release/18.7/)

## SQLite

[Issue #8](https://github.com/igorpooh1978/GoEF/issues/8) · Candidate 3.54.1 · `HANDSHAKE_PASS` · `DOC_LOOKUP_PASS` · `VERSION_MATCH_PASS` for the sampled source.

At 17:25–17:27 UTC, Context7 `resolve-library-id` identified `/websites/sqlite_docs` without a catalog version selector. Querying that ID for the exact release returned the version constants from SQLite's C API reference, including `SQLITE_VERSION_NUMBER` 3054001 and source timestamp/hash beginning `2026-10-16 14:45:20 555e07ef33f69`. Direct comparison matched the full source ID and numeric version. The PRAGMA topic lookup corroborated that foreign-key enforcement settings cannot change while a transaction or savepoint is pending. An earlier lookup for the generalized rebuild surfaced a separate schema-editing procedure. Use the official generalized rebuild section for the migration design and `MIG-SQLITE-001-S01` for future acceptance. Recheck version constants when working with the unversioned catalog. [Version constants](https://www.sqlite.org/capi3ref.html#SQLITE_VERSION) · [Exact release](https://www.sqlite.org/releaselog/3_54_1.html) · [Foreign-key PRAGMA](https://www.sqlite.org/pragma.html#pragma_foreign_keys) · [Generalized ALTER TABLE](https://www.sqlite.org/lang_altertable.html#making_other_kinds_of_table_schema_changes)

## SQL Server

[Issue #6](https://github.com/igorpooh1978/GoEF/issues/6) · Candidate 2022 CU28 (16.0.4325.4) · `HANDSHAKE_PASS` · `DOC_LOOKUP_PASS` · `VERSION_MATCH_PASS` for the sampled source.

At 17:25–17:28 UTC, `microsoft_docs_fetch` returned the 16.x `SAVE TRANSACTION` article, including the case-sensitive 32-character savepoint limit and the restriction on distributed transactions. Fetching the official build list returned CU28, 16.0.4325.4, KB5125472 and 2026-11-12, all corroborated directly. A request to a guessed individual CU path could not be retrieved. The confirmation uses the canonical build list and the applicable 16.x language reference. Establish actual runner support, edition and compatibility settings before conformance. [SAVE TRANSACTION](https://learn.microsoft.com/en-us/sql/t-sql/language-elements/save-transaction-transact-sql?view=sql-server-ver16#remarks) · [SQL Server 2022 CU builds](https://learn.microsoft.com/en-us/troubleshoot/sql/releases/sqlserver-2022/build-versions#sql-server-2022-cumulative-update-cu-builds)

## MySQL

[Issue #11](https://github.com/igorpooh1978/GoEF/issues/11) · Candidate 8.4.13.1 · `HANDSHAKE_PASS` · `DOC_LOOKUP_PASS` · Exact release `BLOCKED`.

At 17:25–17:27 UTC, broad Context7 resolution led to a catalog entry advertising 8.0. Resolving specifically for “MySQL 8.4 Reference Manual” found `/websites/dev_mysql_doc_refman_en` with 8.4 and 8.0. `query-docs` on `/websites/dev_mysql_doc_refman_en/8.4` returned the matching Oracle 8.4 section about implicit commits before many DDL statements. The exact 8.4.13.1 query returned general documentation for the four-part version introduced in 8.4.13 and lacked the requested release evidence. The 2026-11-24 update is documented directly by Oracle. Confirm exact MCP provenance and an available artifact before freezing the cell. Observe the exact identity with `VERSION_WITH_UPDATE()` when running the future `MIG-MYSQL-001-S01` fixture. [Implicit commit section](https://dev.mysql.com/doc/refman/8.4/en/implicit-commit.html) · [Exact update notes](https://dev.mysql.com/doc/relnotes/mysql/8.4/en/news-8-4-13-1.html)

## Alternative route and open gates

The official MCP Registry's public listing API was queried with `version=latest` and searches for `postgresql`, `mysql` and `context7`. It returned 3 PostgreSQL entries, 26 MySQL entries and 3 Context7-related entries, each with no next cursor. This bounded search did not identify another vendor-backed exact-version documentation route for PostgreSQL 18.7 or MySQL 8.4.13.1. Engine query and inspection entries were excluded from documentation readiness. The Context7 listings included a fork and a third-party wrapper. [Registry API reference](https://github.com/modelcontextprotocol/registry/blob/main/docs/reference/api/official-registry-api.md)

The [2026-11-26 follow-up](MCP_EXACT_RELEASES.md) supplies the exact-version evidence for [#10](https://github.com/igorpooh1978/GoEF/issues/10) and [#11](https://github.com/igorpooh1978/GoEF/issues/11). [#8](https://github.com/igorpooh1978/GoEF/issues/8) and [#6](https://github.com/igorpooh1978/GoEF/issues/6) track remaining adoption, driver/engine pin and conformance work for SQLite and SQL Server. Project configuration remains pending for both documentation endpoints. The [matrix](../../parity/PROVIDER_MATRIX.md) records the four runtime cells separately.

## Optional inspection candidates

These are static public documentation observations. GoEF has approved and tested no inspection routes. Audit the chosen artifact, its dependencies, full tool permissions, advisories and database-enforced isolation before any adoption.

| Scope | Candidate and observation |
| --- | --- |
| PostgreSQL | [Crystal DBA PostgreSQL MCP](https://github.com/crystaldba/postgres-mcp) advertises restricted and broader modes. Its [security tracker](https://github.com/crystaldba/postgres-mcp/security) and [open default-access report](https://github.com/crystaldba/postgres-mcp/issues/164) warrant a pinned source and security review. |
| PostgreSQL, SQLite, MySQL | [database-mcp](https://github.com/arifulislamat/database-mcp) advertises two tools, schema discovery and SQL execution, with default read-only controls and limits. The public description does not establish independent fixture isolation. |
| SQLite | [mcp-sqlite-server](https://github.com/skycandykey1/mcp-sqlite-server) advertises read-only SQLite access. A disposable read-only file and a restricted process remain required. |
| SQL Server | [readonly-db-mcp](https://github.com/febrifaresi/readonly-db-mcp) advertises read-only SQL tools. Microsoft's [SQL MCP Server](https://learn.microsoft.com/en-us/sql/mcp/) is a Data API builder data/entity surface with configurable DML capabilities. The Learn endpoint provides documentation. |
| MySQL | [mysql-mcp-server](https://github.com/jakubpliszka/mysql-mcp-server) advertises read-only diagnostics. Require a least-privilege disposable account, bounded result access and a review of exposed diagnostics. |
