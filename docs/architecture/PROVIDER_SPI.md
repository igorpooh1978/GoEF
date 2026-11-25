# Relational provider SPI

**Architecture decision: approved for W001. Execution SPI v1 is frozen.** The [Go package](../../provider/transport.go) is the authoritative method and type declaration for the v1 execution boundary. Providers use ordinary exported types from the core module and remain independently implementable. The version is a contract generation, independent of Go module release tags. [W001 issue #12](https://github.com/igorpooh1978/GoEF/issues/12) · [Module ADR](../adr/0002-module-and-provider-boundaries.md)

| Frozen contract | Required behavior |
| --- | --- |
| `Backend`, `Descriptor` and `Connector` | Read the static descriptor and reject an empty ID or incompatible major before Acquire or user SQL. A failed acquisition transfers nothing. Acquisition transfers one exclusive physical connection lease. |
| `Lease` and `Executor` | Query and Exec remain on the leased connection. The adapter does not silently fall back to the pool or replay the command. A successful Query transfers one result stream. A failed Query cleans up internally. |
| `Command`, `Bindings` and `Value` | The compiled SQL and ordered input/output layouts are reusable. Bind data separately. ValidateBindings checks count, concrete value kind and nullability before dispatch. A NULL is explicit. Bindings and metadata are borrowed for the call. Copy retained slices and bytes. |
| `Rows` | Rows are collected serially in column order. `io.EOF` means clean exhaustion. The Query context governs dispatch and the stream lifetime, including an in-flight Next. Late failures and causal cancellation stay observable. Borrowed rows and binary cells expire at the next advance or close. Close after success, error or early exit. Close is repeat-safe and its first cleanup error remains observable. |
| `WriteResult` and `Disposition` | A known zero is distinct from an unavailable row count. Release consumes the lease even on error. An unknown, dirty or ambiguous connection is discarded or quarantined. A healthy connection may be reused after reset. Release never closes the caller's pool. |

The consumer closes rows before releasing the lease. Give cleanup a fresh, bounded context if the operation context is already cancelled. Do not leak driver messages, SQL or values across the public error boundary. Preserve primary and cleanup errors, and preserve causal context errors for errors.Is. The [error and lifecycle contracts](ERROR_CONTRACT.md) govern those obligations.

## Compatibility and extensions

Keep the required execution interface and its semantics stable within v1. Add features through separately discoverable optional interfaces and explicit capabilities. A change to required methods or existing lifecycle or value semantics requires a new SPI generation and a compatibility record. A provider cannot satisfy an unavailable feature by routing it through an implicit fallback.

Model codecs/map negotiation, the compiler's relational plan types, transactions and savepoints, and migration planning and execution are staged extensions. Required behaviors remain in the [W005 transaction contract](TRANSACTION_ENGINE.md), [migration contract](MIGRATION_ENGINE.md), and [module graph](MODULE_GRAPH.md). In particular, transaction creation must bind the exact connection, only the owner may complete it, and an unknown commit outcome prohibits replay. This change freezes the transport on which those extensions depend and leaves their public types to their implementing waves, where two concrete implementations can validate them.

The [external module fixture](../../test/architecture/provider-spi/go.mod) compiles against the exact core import path and exercises query and write boundaries. The [W002 lab](../../test/conformance/lab/) tests isolated engines at the CLI layer. Real adapter behavior needs separate tests in all four providers.
