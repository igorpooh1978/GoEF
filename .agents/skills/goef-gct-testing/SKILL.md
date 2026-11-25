---
name: goef-gct-testing
description: Write GoEF Gherkin, executable RED/GREEN tests and traceability for provider conformance, negative paths, fault injection, races and coverage.
---

# GoEF Gct Testing

1. Read [Testing](../../../docs/development/TESTING.md), [Test Environment](../../../docs/development/TEST_ENVIRONMENT.md) and [Coverage](../../../docs/development/COVERAGE.md). Derive stable scenario IDs and observable expected behavior from a scoped issue and upstream evidence.
2. Freeze scenarios with the contract owner. Demonstrate behavioral RED or an existing characterization before implementation. A broken harness or absent driver counts as an execution blocker, independent of the expected failure.
3. Cover positive, negative, cancellation, concurrency, data preservation and recovery decisions as applicable. Use deterministic synchronization and synthetic fixtures. Run real target engines for provider conformance. Record command counts and final observable state where relevant.
4. Run GREEN and the affected matrix on the integrated SHA. Record precise commands, versions, exits, skips, SHA, redacted evidence path and scenario-to-test-to-doc links. Report per-module/per-provider coverage and separate scenario coverage.

If only a mock observes SQL for a migration, retain it as unit-level evidence and keep the real migration gate pending until engine results are captured.
