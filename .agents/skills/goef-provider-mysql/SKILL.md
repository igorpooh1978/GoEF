---
name: goef-provider-mysql
description: Work on the GoEF MySQL/InnoDB and its selected driver adapter. Verify MySQL SQL dialect, InnoDB isolation and locks, sql_mode, collation, native error numbers, and implicit DDL commits with exact vendor and driver versions plus real conformance.
---

# GoEF provider for MySQL/InnoDB and its selected driver

1. Read the [provider guide](../../../docs/providers/MYSQL.md), [provider SPI](../../../docs/architecture/PROVIDER_SPI.md) and [documentation MCP policy](../../../docs/development/MCP_POLICY.md). Correlate verified documentation MCP results with versioned vendor and driver references. Record an unavailable lookup separately from source-only research.
2. Identify the upstream/provider contract, native types, capability version bounds, engine configuration and relevant Gherkin IDs. Keep driver-exported types and database-specific branches within provider boundaries.
3. Exercise the provider-specific conditions below on a disposable target. Verify results and redacted parameterized SQL traces, including relevant rejection, cancellation and resource cleanup.
4. Update [capability records](../../../docs/parity/PROVIDER_MATRIX.md), the [error mapping](../../../docs/errors/provider-mappings/mysql.json) and the guide with the issue and exact commands, versions, SHA and observed results.

## Provider checks

- Verify server variant/version, InnoDB and selected driver/version. Capture session SQL mode, character set and collation. Exercise decimal/time and null values, quoting, parameter binding and generated IDs.
- Check metadata locks, isolation, savepoints and statement classes that cause implicit commits using actual engines. Distinguish schema recovery from an application-level rollback.
- Classify native error numbers and SQLSTATE where available. Include phase and constraints for duplicate keys, deadlock and lock timeouts, and preserve cancellation separately.
