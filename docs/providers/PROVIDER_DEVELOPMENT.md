# Provider development

Tracked by [W000 #1](https://github.com/igorpooh1978/GoEF/issues/1).

1. Open or reuse an English issue and link the upstream EF Core contract, the engine and driver sources, the feature registry and the Gherkin scenario. Apply the relevant repository skill and the [MCP research policy](../development/MCP_POLICY.md). Record exact target versions and settings in the [provider matrix](../parity/PROVIDER_MATRIX.md).
2. Keep the provider driver, dialect, binder, codecs, schema introspection, migrations, locking and native error translation inside the provider module. Keep the core independent of concrete drivers. Resolve engine/version capabilities before translation or execution. Use a typed unsupported result when a safe implementation is unavailable. Document deliberate emulation and its semantics.
3. Write a portable contract scenario for all four providers and engine-specific scenarios for behavioral differences. Run them against isolated real fixtures. Record native error codes, rows, schema and migration history before and after failures, plus cancellation, cleanup and pool reuse. Use distinct writer and inspection identities if optional inspection is approved.
4. Tie each check to a capability entry with the support decision, conditions and limits, engine and driver versions, scenario and executable test path, official source and passing SHA. Baseline entries without a passing execution stay pending. A reviewed capability exception includes the vendor evidence, issue, ADR and test for the supported alternative or typed rejection.
5. At a version upgrade, review engine release notes, driver changes and advisories. Recheck the exact documentation route, rerun all applicable conformance cells and capture the observed build, server/database settings, image digest or embedded runtime source ID and resolved driver version. Refresh user-facing docs and the status matrix from the same evidence.

## Required capability inventory

| Area | Per-version research and proof |
| --- | --- |
| Dialect and query | Identifier quoting, maximum length and folding, schema and catalog semantics, escaping versus parameter binding, pagination, NULL order, Unicode/collation, native limits and prepared statements. |
| Mapping and materialization | Affinity or strict typing, coercion, precision, UUID, JSON, arrays, binary, nullable values, date/time and zones, collation, generated keys, row counts and returned rows. |
| Model and introspection | Identity and sequences, generated/computed values, defaults, checks, composite keys, FKs and cascade behavior, indexes, constraints, views, triggers and metadata visibility under the fixture identity. |
| DDL and recovery | DDL syntax, atomicity, schema diff, locks and lock ownership, history/checksum, point of interruption, partial execution and persisted recovery evidence. |
| Transactions and errors | Isolation, savepoints, lock contention and concurrent connections, native codes and typed wrapper, timeout, cancellation, connection recovery, ambiguous commit, replay safety and diagnostic redaction. |
| Provider-specific operations | Upsert and bulk behavior, data and parameter limits, required edition, privileges, compile options, configuration and feature gates. |
