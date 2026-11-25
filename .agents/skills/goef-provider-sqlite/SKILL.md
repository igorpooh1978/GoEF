---
name: goef-provider-sqlite
description: Work on the GoEF SQLite and its selected driver adapter. Verify SQLite affinities, PRAGMAs, connection settings, WAL and locking, table rebuild, and extended driver error codes with exact vendor and driver versions plus real conformance.
---

# GoEF provider for SQLite and its selected driver

1. Read the [provider guide](../../../docs/providers/SQLITE.md), [provider SPI](../../../docs/architecture/PROVIDER_SPI.md) and [documentation MCP policy](../../../docs/development/MCP_POLICY.md). Correlate verified documentation MCP results with versioned vendor and driver references. Record an unavailable lookup separately from source-only research.
2. Identify the upstream/provider contract, native types, capability version bounds, engine configuration and relevant Gherkin IDs. Keep driver-exported types and database-specific branches within provider boundaries.
3. Exercise the provider-specific conditions below on a disposable target. Verify results and redacted parameterized SQL traces, including relevant rejection, cancellation and resource cleanup.
4. Update [capability records](../../../docs/parity/PROVIDER_MATRIX.md), the [error mapping](../../../docs/errors/provider-mappings/sqlite.json) and the guide with the issue and exact commands, versions, SHA and observed results.

## Provider checks

- Verify linked SQLite version, build/driver features, foreign-key PRAGMA scope, journal mode, busy handling, affinity and nullability. Use disposable files for behavior involving multiple connections, locking or WAL.
- Check rebuild/copy behavior under foreign keys, indexes, triggers and version-specific ALTER TABLE support. Verify row values and dependent objects after interruption and recovery.
- Use SQLite primary and extended result codes plus phase. Test `BUSY` separately from `LOCKED`. In-memory URI or connection settings can change database identity.
