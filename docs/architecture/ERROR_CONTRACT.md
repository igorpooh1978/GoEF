# Error contract

**Proposed public behavior.** Stable machine IDs, sentinel names, typed detail, native mapping proposals and evidence status live in the [error registry](../errors/README.md). W002 owns public primitives and test harnesses. W005 owns native adapter proof, with resilience and tracker proof in W009 and W011.

## A caller can choose the next action

| Observation | Meaning and caller action |
| --- | --- |
| Cardinality requires one result and observes zero | Return `ErrNotFound`. An ordinary empty sequence succeeds. A failed scan, permission error or dropped connection preserves its own classification. |
| Compare-and-write affects zero rows using the tracked original token | Return `ErrConcurrencyConflict`. Preserve the caller's pending edits and original snapshot. The application resolves or reloads before another attempt. [EF concurrency conflicts](https://learn.microsoft.com/en-us/ef/core/saving/concurrency) |
| Server reports a constraint or conversion failure | Inspect the typed native code plus operation-specific context. Use a portable constraint kind when evidence uniquely identifies it. Keep row values and vendor text private. |
| Caller context caused cancellation or deadline | Return the portable detail and preserve `errors.Is(err, context.Canceled)` or `errors.Is(err, context.DeadlineExceeded)`. A coincidentally canceled context alone is insufficient. A server lock/statement budget with a live context has its own kind and phase. |
| Commit sent, durable result unknown | Return `ErrCommitOutcomeUnknown` even if a lower layer reports timeout, cancellation or unavailable transport. Preserve causal cancellation as an additional safe cause. Suspend tracker acceptance, stop replay and require reconciliation. |
| No unique native rule applies | Return `ErrUnknownDriver` and keep retry class never. If an adapter knows the phase but lacks enough evidence to classify the cause, emit safe code, phase and provider. |

## Go errors and diagnostic boundaries

Portable callers use `errors.Is` for kinds and causal context errors, and `errors.As` for the proposed `*ErrorDetail`. Ordinary application wrapping with `%w` preserves those operations. Typed detail exposes the stable ID/kind, operation, phase, provider, known native code, retry class, disposition/outcome and whitelisted aliases for entity, constraint, migration or operation where applicable. Omit unknown or unsafe fields. Capture actual runtime outcome separately from the registry's decision policy. The public unwrap chain contains only documented safe causes. Driver types, diagnostic strings, rendered SQL, bind values, DSNs and credentials stay outside it. An explicitly opted-in, provider-specific diagnostic hook may expose allowlisted typed native fields under its own access policy. [Standard Go error inspection](https://pkg.go.dev/errors)

Record a primary failure once at the handling boundary. When cleanup also fails, join redacted primary and secondary errors, preserving both classifications. The primary branch determines the top-level operation and retry/outcome decision. Consumers inspecting a cleanup error of the same detail type can traverse the joined branches explicitly. Expected validation or database errors return ordinary error values. An internal invariant failure gets its own code. A programming panic in a caller callback unwinds after cleanup. The adapter cannot treat a panic as a recoverable database error.

The proposed CLI emits the same stable ID, kind, phase, provider, outcome and retry class in JSON. Exit zero means success. Use distinct nonzero categories for usage/validation, ordinary operation failure, and outcome requiring operator resolution. A numeric exit-code table and executable examples are W007 work. Printable messages are safe English summaries and may evolve independently of IDs. [Canonical fields and allocation](../errors/README.md)

## Classify and govern replay

1. Record dispatch state and transaction ownership. Protect lost commit acknowledgements first and preserve causal cancellation.
2. Use runtime semantics where the ORM has evidence of cardinality or optimistic write predicates. A server's affected-row count needs the operation intent.
3. Read typed server code, full SQLite extended code or a driver-specific typed error. Match engine/driver versions, exact fields, configuration and required phase. Require a unique applicable rule at the highest priority. Ambiguous matches remain unknown.
4. Classify retry and transaction disposition independently. Distinguish statement failure, aborted transaction, failed cleanup and unconfirmed commit. Explicitly confirm rollback before considering replay. A constraint or caller cancellation ends the attempt. An independently demonstrated statement-safe operation must have no observable effects, with a valid transaction state. A transaction-safe unit needs bounds, fresh state and an application replay contract.
5. Capture redacted evidence and cleanup state. Promotion requires a positive real repro, negative/control cases, observed native fields, actual engine/driver configuration and the integrated evidence SHA. Engine documentation supplies expectations for the experiment. Unit-constructed vendor errors exercise classifier routing only. [Mapping governance](../errors/README.md) · [Execution strategy](TRANSACTION_ENGINE.md)
