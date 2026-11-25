---
name: goef-provider-postgres
description: Work on the GoEF PostgreSQL and pgx adapter. Verify PostgreSQL dialect, pgx type bindings and SQLSTATE, cancellation, transactional DDL, savepoints and locks with exact vendor and driver versions plus real conformance.
---

# GoEF provider for PostgreSQL and pgx

1. Read the [provider guide](../../../docs/providers/POSTGRESQL.md), [provider SPI](../../../docs/architecture/PROVIDER_SPI.md) and [documentation MCP policy](../../../docs/development/MCP_POLICY.md). Correlate verified documentation MCP results with versioned vendor and driver references. Record an unavailable lookup separately from source-only research.
2. Identify the upstream/provider contract, native types, capability version bounds, engine configuration and relevant Gherkin IDs. Keep driver-exported types and database-specific branches within provider boundaries.
3. Exercise the provider-specific conditions below on a disposable target. Verify results and redacted parameterized SQL traces, including relevant rejection, cancellation and resource cleanup.
4. Update [capability records](../../../docs/parity/PROVIDER_MATRIX.md), the [error mapping](../../../docs/errors/provider-mappings/postgres.json) and the guide. Every evidence row must give the issue, the versioned PostgreSQL and pgx documentation links, the documentation MCP lookup and status, exact commands, engine and driver versions, SHA and observed results.

## Provider checks

- Verify pgx mode, driver ownership and row cleanup, placeholder and identifier quoting, PostgreSQL SQLSTATE and constraint name. Confirm bounded statement/lock behavior under the documented connection mode.
- Exercise arrays, nullable values, numeric precision, timestamps and supported generated values through real round trips. Tie JSONB, RETURNING, index options, advisory locks and transactional DDL to tested versions and transaction phase as applicable.
- For serialization failure, classify the SQLSTATE and phase and verify replay safety with the transaction skill. A transport failure during commit is a separate outcome.
