---
name: goef-provider-sqlserver
description: Work on the GoEF SQL Server and its selected driver adapter. Verify T-SQL dialect, SQL Server errors, identity and OUTPUT, version/compatibility settings, isolation, locking and savepoints with exact vendor and driver versions plus real conformance.
---

# GoEF provider for SQL Server and its selected driver

1. Read the [provider guide](../../../docs/providers/SQLSERVER.md), [provider SPI](../../../docs/architecture/PROVIDER_SPI.md) and [documentation MCP policy](../../../docs/development/MCP_POLICY.md). Correlate verified documentation MCP results with versioned vendor and driver references. Record an unavailable lookup separately from source-only research.
2. Identify the upstream/provider contract, native types, capability version bounds, engine configuration and relevant Gherkin IDs. Keep driver-exported types and database-specific branches within provider boundaries.
3. Exercise the provider-specific conditions below on a disposable target. Verify results and redacted parameterized SQL traces, including relevant rejection, cancellation and resource cleanup.
4. Update [capability records](../../../docs/parity/PROVIDER_MATRIX.md), the [error mapping](../../../docs/errors/provider-mappings/sqlserver.json) and the guide with the issue and exact commands, versions, SHA and observed results.

## Provider checks

- Verify server product version and database compatibility level separately, including isolation settings and collation. Test T-SQL parameter and identifier rules, identity/OUTPUT, nullable and precision round trips and required pagination order.
- Check transaction/savepoint operation phases and DDL restrictions against exact supported versions. Probe database settings that affect isolation and cancellation.
- Map documented error numbers plus state/context only where justified by observed driver results. Exercise deadlock and cancellation separately and keep MARS-sensitive behavior explicitly scoped.
