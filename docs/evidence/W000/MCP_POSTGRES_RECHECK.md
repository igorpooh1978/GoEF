# W000 PostgreSQL documentation MCP recheck

Tracking [#10](https://github.com/igorpooh1978/GoEF/issues/10). Public documentation was checked on 2026-11-25 UTC against GoEF baseline `c789602e46c7063e6c0f1ddbfec68bc22c2dbb75` in `BOOTSTRAP_ONLY` mode. The research cell is PostgreSQL 18.7 with `github.com/jackc/pgx/v5` v5.13.0. Context7 passed the handshake and PostgreSQL 18 topic checks. Exact 18.7 provenance on the project's documentation MCP route remains blocked. Separate read-only GitHub MCP calls confirmed the official tagged documentation for 18.7. Runtime conformance remains `NOT RUN`. [Original probe](MCP_REPORT.md#postgresql) · [MCP policy](../../development/MCP_POLICY.md) · [pgx candidate](https://github.com/jackc/pgx/blob/v5.13.0/README.md)

The [2026-11-26 exact-release follow-up](MCP_EXACT_RELEASES.md) passed through a different standalone documentation MCP route. The observations below record the 2026-11-25 probes.

## Context7

Three anonymous one-shot Python 3.14.4 standard-library probes ran at 17:50:57–17:53:24 UTC. Each posted JSON-RPC `initialize`, `notifications/initialized` and `tools/list` to `https://mcp.context7.com/mcp` using Streamable HTTP. The client requested `application/json, text/event-stream` and proposed protocol `2025-06-18`. Context7 advertised 4.7.3, accepted the proposed protocol, and returned HTTP 200 SSE, 202, and 200 SSE respectively, with no session ID. Subsequent requests used the negotiated `MCP-Protocol-Version` header. The tool list contained `resolve-library-id` and `query-docs`. The live `query-docs` schema offered `libraryId`, `query` and optional `language`. Upstash's newer 4.8.0 release notes describe a separate version argument with closest-version fallback. The hosted schema checked here used the version suffix. [Transport specification](https://modelcontextprotocol.io/specification/2025-06-18/basic/transports) · [Context7 releases](https://github.com/upstash/context7/releases)

For reproduction, use `tools/call` and the following `arguments`. Read each JSON-RPC reply by request ID. There were nine HTTP 200 SSE tool replies, eight with successful MCP results and one with `isError: true`. The three Python probe processes exited 0.

| Tool and selector | Query | Observed result |
| --- | --- | --- |
| `resolve-library-id` with `libraryName` values `PostgreSQL 18.7` and `postgresql` | PostgreSQL 18.7 release notes https://www.postgresql.org/docs/release/18.7/ or https://www.postgresql.org/docs/18/release-18-7.html. Return the release heading, date and specific changes from the matching source. | Both resolutions listed `/websites/postgresql` with major versions 13–19 and `/postgres/postgres` with `rel_17_6` and `rel_16_10`. The broader search also listed `/postgres/pgweb`. |
| `resolve-library-id` with `libraryName` `PostgreSQL Release Notes` | Find the current official PostgreSQL 18.7 Release Notes, https://www.postgresql.org/docs/18/release-18-7.html or https://www.postgresql.org/docs/release/18.7/, from the PostgreSQL project. | Listed the same major-version website corpus alongside wiki and development documentation. |
| `query-docs` with `libraryId` values `/websites/postgresql/18.7` and `/websites/postgresql/18` | PostgreSQL 18.7 release notes https://www.postgresql.org/docs/release/18.7/ or https://www.postgresql.org/docs/18/release-18-7.html. Return the release heading, date and specific changes from the matching source. | The exact selector explicitly reported that 18.7 was unindexed and fell back to 18. Both replies cited the PostgreSQL 18 appendixes and linked 18.6 as E.1. Neither returned the exact release page. [Appendixes](https://www.postgresql.org/docs/18/appendixes.html) · [Release appendix](https://www.postgresql.org/docs/18/release.html) |
| `query-docs` with `libraryId` `/postgres/postgres/REL_18_7` | PostgreSQL tagged source REL_18_7 doc/src/sgml/release-18.sgml official 18.7 release heading and release date. | `isError: true`. The tool reported no documentation match in `/postgres/postgres`. |
| `query-docs` with `libraryId` `/websites/postgresql/18` | PostgreSQL 18 documentation version in the official root table of contents https://www.postgresql.org/docs/18/index.html. What precise 18.x release does the documentation book identify in its title? | The returned PostgreSQL 18 appendixes again linked 18.6 as E.1. |
| `query-docs` with `libraryId` `/websites/postgresql/18` | PostgreSQL 18 Appendix A official error codes https://www.postgresql.org/docs/18/errcodes-appendix.html. Return the version stated in the paragraph that introduces Table A.1 and entries for serialization_failure (40001) and deadlock_detected (40P01). | The returned introduction identified the table as PostgreSQL 18.6. The current paragraph at the returned vendor URL identifies 18.7. [Error code appendix](https://www.postgresql.org/docs/18/errcodes-appendix.html) |
| `query-docs` with `libraryId` `/websites/postgresql/18` | PostgreSQL 18 CREATE INDEX CONCURRENTLY requirements for running inside a transaction block. Use the official Building Indexes Concurrently section of https://www.postgresql.org/docs/18/sql-createindex.html. | The returned section confirmed that the concurrent operation requires execution outside a transaction block. This agrees with the official manual and exact tagged source. [Building Indexes Concurrently](https://www.postgresql.org/docs/18/sql-createindex.html#SQL-CREATEINDEX-CONCURRENTLY) |

Result under the project policy: `HANDSHAKE_PASS` and `DOC_LOOKUP_PASS` for the sampled major-version topic. The explicit version fallback and the stale release and SQLSTATE version markers keep exact `VERSION_MATCH_PASS` blocked for this route.

## Tagged documentation through GitHub MCP

The pre-existing GitHub connector returned public vendor documentation with `fetch_file`. Four calls succeeded. Each file was read at `REL_18_7` and then at the exact commit. Both the PostgreSQL upstream Git endpoint and the project's GitHub mirror returned `f5c41f6681c910b3f69951bf6fb97c6d74605d25` for the tag, with exit 0 for each discovery command.

```text
git ls-remote https://git.postgresql.org/git/postgresql.git refs/tags/REL_18_7 'refs/tags/REL_18_7^{}'
git ls-remote https://github.com/postgres/postgres.git refs/tags/REL_18_7 'refs/tags/REL_18_7^{}'
```

Each `fetch_file` request used `repository_full_name: postgres/postgres` and the listed `path`, `ref`, `start_line` and `end_line`. The connector supplied the referenced source URL and blob SHA.

| Path | `REL_18_7` lines | Commit lines | Blob SHA | Evidence |
| --- | --- | --- | --- | --- |
| `doc/src/sgml/release-18.sgml` | 1–80 | 1–45 | `f2a71ff008a57829639f04b675027bc9caa75823` | 18.7, 2026-11-12 and upgrade notes about affected indexes and incremental backups. [Release source](https://github.com/postgres/postgres/blob/f5c41f6681c910b3f69951bf6fb97c6d74605d25/doc/src/sgml/release-18.sgml#L4-L37) |
| `doc/src/sgml/ref/create_index.sgml` | 520–760 | 691–702 | `b9c679c41e8dbc699760921401a47d4cd0c00218` | The concurrent index build must run outside a transaction block. [CREATE INDEX source](https://github.com/postgres/postgres/blob/f5c41f6681c910b3f69951bf6fb97c6d74605d25/doc/src/sgml/ref/create_index.sgml#L691-L702) |

These calls corroborate exact tagged documentation through a general source connector. Its raw endpoint and transport handshake are outside the connector's exposed interface. The project documentation route stays blocked because that gate requires a verified documentation-only surface.

## Other route and independent vendor control

Two anonymous candidate endpoints each rejected JSON-RPC `initialize` with HTTP 403 and Cloudflare error 1010 (`browser_signature_banned`, `retryable: false`). Both one-shot probes exited 1 at initialization, before document URLs were sent.

| Endpoint | UTC time | Documented tool and audit references |
| --- | --- | --- |
| `https://mcp.exa.ai/mcp?tools=web_fetch_exa` | 17:52:01 | Exa's anonymous Streamable HTTP endpoint with the single `web_fetch_exa` tool selected. [Publisher documentation](https://exa.ai/docs/get-started/exa-mcp) · [Source and license](https://github.com/exa-labs/exa-mcp-server) · [Advisories](https://github.com/exa-labs/exa-mcp-server/security) |
| `https://gitmcp.io/postgres/postgres` | 17:55:54 | GitMCP's repository-scoped endpoint documents `fetch_url_content` for linked public pages. [Publisher documentation and source](https://github.com/idosal/git-mcp) · [Advisories](https://github.com/idosal/git-mcp/security) · [MCP catalog](https://hub.docker.com/mcp/server/gitmcp/overview) |

Docfork documents a documentation-only toolset with account sign-in or API-key access. It remains outside this anonymous probe. [Docfork setup](https://github.com/docfork/docfork#mcp-setup)

The public [MCP Registry listing](https://github.com/modelcontextprotocol/registry/blob/main/docs/reference/api/official-registry-api.md) returned HTTP 200 with three results for `postgresql`, zero for `postgresql-docs`, three for `context7`, and zero for `exa-mcp-server`. Requests used `GET /v0.1/servers?version=latest&search=<term>` and none returned a next cursor. The PostgreSQL results target database access. The Context7 results covered Upstash and third-party derivatives. This bounded search found no additional vendor-backed documentation-only route.

Five direct, independent HTTP reads returned 200 and passed the version and content checks. The official release archive and versioned release page confirmed 18.7 and 2026-11-12. The live release index listed 18.7 as E.1. The error code appendix identified 18.7 and contained SQLSTATE `40001` and `40P01`. The CREATE INDEX section corroborated the transaction boundary. These requests provide a vendor control for the MCP observations. [Release archive](https://www.postgresql.org/docs/release/18.7/) · [Versioned release notes](https://www.postgresql.org/docs/18/release-18-7.html) · [Release index](https://www.postgresql.org/docs/18/release.html) · [Error code appendix](https://www.postgresql.org/docs/18/errcodes-appendix.html) · [Building Indexes Concurrently](https://www.postgresql.org/docs/18/sql-createindex.html#SQL-CREATEINDEX-CONCURRENTLY)

Keep the 18.7 candidate and the existing migration transaction boundary in the [provider guide](../../providers/POSTGRESQL.md). Recheck Context7 once the hosted corpus returns 18.7 and its relevant topic, or review a provenance-preserving adapter with a vendor HTTPS allowlist under the [MCP policy](../../development/MCP_POLICY.md). The real-engine transaction, migration and native-error scenarios remain tracked by [#13](https://github.com/igorpooh1978/GoEF/issues/13).
