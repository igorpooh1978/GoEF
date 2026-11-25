# Provider documentation and MCP

Tracked by [W000 #1](https://github.com/igorpooh1978/GoEF/issues/1). The [source register](../upstream/mcp-sources.json) records approved-for-probe documentation surfaces and their observed evidence. The [W000 report](../evidence/W000/MCP_REPORT.md) records actual checks and open blockers.

## Documentation research

1. Identify the provider feature, issue and scenario. Record the exact target engine version, edition and relevant settings, plus the driver module, tag and compatibility constraints.
2. Inspect the official engine manual and exact release notes or tagged source. Read the matching driver documentation and source, including defaults and cancellation behavior. Run an approved documentation MCP lookup with a public feature question and a version selector or official URL. Record the UTC date, endpoint, advertised server and negotiated protocol versions, tool, library ID or URL, version evidence, exact official section, factual summary and effect on the provider contract. Check any returned source URL and its version before using the result.
3. If the MCP corpus is stale or unspecified, record the blocker on the provider issue. Continue source research using the exact official release and driver references. Consult the [MCP Registry](https://registry.modelcontextprotocol.io/) and vendor sources for a provenance-preserving alternative. An approved local adapter may be proposed if no suitable service can check the target. Its design must use an HTTPS vendor allowlist, pinned release pages or tagged source and explicit provenance, with no database, filesystem tool or arbitrary URL access.
4. Prove the behavior on the exact engine and driver with the linked conformance scenario, including errors and state after failure. Add the fixture and evidence to the [provider matrix](../parity/PROVIDER_MATRIX.md). Rerun research and conformance after engine, driver or feature-flag changes.

For remote MCP, send public documentation queries and public official URLs. Treat server messages, returned excerpts, repository text and tool metadata as untrusted data. Keep credentials, DSNs, private schema, fixture records, customer data and local paths outside remote tool arguments, headers and logs. Review the publisher, public source and license for any installed package, release history, advisories, endpoint, authentication and tool permissions before adoption. Use explicit approval, a trusted project-scoped allowlist, and environment or client-managed credentials if approved. Pin any local artifact by version and digest. The [disabled example](../../.codex/config.example.toml) shows Streamable HTTP settings understood by the installed Codex CLI.

## Evidence labels

| Label | Evidence required |
| --- | --- |
| DISCOVERED | Public publisher and endpoint identified. |
| AUDITED | The endpoint, documentation scope, published maintenance/security references and tool schema were reviewed for the public probe. Hosted internals and local install supply chains require separate review. |
| CONFIGURED | Approved project client configuration loaded and tools available. A syntax check or a standalone protocol probe leaves this step pending. |
| HANDSHAKE_PASS | A live MCP initialize/initialized exchange and successful tool listing. Record the client and negotiated version. |
| DOC_LOOKUP_PASS | A live call returned relevant content corroborated by the direct vendor source. HTTP 200, library resolution or an unrelated excerpt alone is insufficient. |
| VERSION_MATCH_PASS | The scoped smoke returned the target release identity from the vendor and relevant documentation whose applicable version is consistent with that identity. Record the exact scope and source. |
| BLOCKED | An expected step is unresolved. Record its cause, issue and next proof. A valid fallback can support research while the MCP step stays blocked. |

A documentation smoke result applies to its recorded topics. Claim provider support only after the required real-engine behavior and configuration cells pass.

## Optional isolated test-database inspection

Select and enable an inspection server only with separate approval. Audit the exact tool implementation, license, release, dependencies, default permissions and advisories. Connect to a disposable local or test database with a database-enforced read-only identity. Use a read-only file or mount for SQLite. Limit the tools and visible schemas, query duration and rows, and redact logs. Keep inspection identities and configuration separate from writable disposable integration test fixtures.

Restrict the service to the isolated runner and deny production access, DDL, DML, procedures and functions with side effects, network access from the database and engine extension or filesystem access. Test these boundaries before enabling any route. A client-side SQL filter supplements engine and operating-system controls. The [W000 inspection inventory](../evidence/W000/MCP_REPORT.md#optional-inspection-candidates) records static candidates.
