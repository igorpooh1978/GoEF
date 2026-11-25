# Testing and evidence

[Issue #9](https://github.com/igorpooh1978/GoEF/issues/9) owns this policy. Bootstrap has document and compiler-probe checks. [Issue #13](https://github.com/igorpooh1978/GoEF/issues/13) owns executable Gherkin and database infrastructure.

## Specify and verify

1. Read pinned behavior and its negative cases. Create an issue, stable feature and scenario IDs, and English Gherkin. Name the expected rows, schema, history, error or resource outcome.
2. Add a named executable RED case or record exact existing behavior when feasible. Distinguish unmet behavior from a failed setup. Convert outline rows into named subtests keyed by provider and version. Keep the four-engine suite separate from fast, deterministic unit tests.
3. Implement, run GREEN, and exercise negative and cleanup cases. Prefer synthetic fixtures, deterministic synchronization and explicit deadlines. Capture race, lifecycle and isolation evidence for concurrent paths.
4. On the integrated commit rerun required gates and link the raw results with toolchain, provider settings, command, exit code, fixture seed and clean state. An independent reviewer checks the behavior and coverage of the evidence.

| Suite | Required proof |
| --- | --- |
| Unit, compile-pass/fail and contract | Values, typed failures, resource lifetimes, public API and generator diagnostics |
| Fuzz and property tests | Decode/IR, identifiers, model diff and snapshot determinism, null and boundary behavior |
| Golden output | Versioned SQL, schema, snapshots and generated Go with independent behavioral checks |
| Provider integration | Actual rows/schema/history, transactions, native errors, isolation, migration locks, outage, interruption and replay |
| Differential | Pinned EF/.NET workload and fixture seed with explicit semantic differences, plus GoLINQ object conformance |
| Security and concurrency | Parameter binding, identifier validation, redaction, permission boundaries, cancellation, race, leak and fault tests |
| Docs and benchmark | Compiled/real-engine examples, deterministic outputs, equivalent workload and allocation/latency measures |

A mock cannot pass a real-engine cell. SQL goldens prove rendering independently of SQL execution. Report BLOCKED when a required engine or proof is unavailable. Use [traceability](../../specs/README.md), the [provider matrix](../parity/PROVIDER_MATRIX.md), and the [coverage policy](COVERAGE.md).
