# ADR 0007 · Explicit LINQ execution boundaries

**Proposed for W001 · LINQ-BOUNDARY-001 · [issue #4](https://github.com/igorpooh1978/GoEF/issues/4)**

Local callbacks can inspect any Go value. A provider needs an inspectable plan, including typed fields and parameters. Sending a table to the client when translation fails can change null and collation semantics, expose rows, apply a tenant filter too late and exhaust memory.

### Decision proposed for compiler and provider fixtures

| Entry | Input | Evaluation and owner |
| --- | --- | --- |
| Objects | Go collections, native iterators, typed callbacks | Consumer enumerates local values. The source owner defines replay, cancellation and mutation rules. |
| Expressions | Typed field descriptors, slots, constants and registered operations | Compose immutable symbolic data. Type, alias and registration checks precede execution. |
| Queryable | A model-bound root plus runtime or generated expressions | The shared IR and provider compiler execute under the caller's context, filters and existing transaction. |

A server query ends at an explicit materializer, proposed as `ToList(ctx)`, or at an explicitly requested error-aware stream. Only after successful materialization can a caller construct an independent Objects sequence over the returned values. A local sequence supplied in the other direction can become a bound collection parameter only through an explicit, provider-supported expression with validated size and type. There is no inverse conversion that turns a callback or a consumed sequence into a server predicate.

A materialized list retains its rows in memory. Restrict columns and bounds on the server before crossing when that fits the intended operation. The resulting ordinary values can be enumerated after context disposal. Copies/projections retain their documented semantics. Tracked entity identity and tracking ownership continue to follow the originating context while it is alive.

An explicit stream, proposed as `iter.Seq2[T, error]`, yields each success with a nil error. A read/decode failure or cancellation observed while consuming produces at most one final zero-value/error pair, then the sequence ends. A consumer who stops before a later failure cannot observe it. Early stop still closes owned rows, releases a borrowed connection according to its owner, and leaves a caller-owned transaction open. A sorting or grouping stage after a stream boundary can still buffer the complete input. Prove these paths before freezing signatures.

An unsupported server expression raises a registered translation/capability error before fetching rows. Record location and operator, keep bindings out of diagnostics, and assert zero executed commands and zero local callback invocations. Closure inspection by address, arbitrary callback execution to infer SQL and implicit local fallback violate this decision. Typed concrete generic methods and generic functions are candidates for the public facade. The provider SPI can use ordinary non-generic methods over validated plans.

**Acceptance.** W001 proves the Go 1.27.4 API constraints including inference, incompatible owners and recursively expanding types. W002 supplies the resource/error probes. W008 verifies the materializer, stream and initial SQL on each configured engine/version. W012 and W013 extend those checks to joins, aggregates and generated entrypoints. See the [boundary specifications](../../specs/features/linq/boundary.feature) and [conformance plan](../../test/conformance/linq/README.md).
