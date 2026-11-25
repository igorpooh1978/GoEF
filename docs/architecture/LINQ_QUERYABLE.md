# Provider-backed LINQ queries

**Proposed contract · LINQ-QUERYABLE-001 · [issue #4](https://github.com/igorpooh1978/GoEF/issues/4).** Building a query returns a new typed plan. Two plans derived from the same root can carry different predicates and projections. Composition opens no connection. A terminal operation receives the caller's context and executes through its selected provider.

1. Bind the root, model fields, aliases, tracking mode, required tenant/filter scope and typed parameters. Keep captured values outside reusable plan shapes.
2. Lower runtime expressions or generated declarations into the shared logical representation. Validate each expression, aggregate, ordering, join and requested operation against the provider/version capabilities.
3. Produce a parameterized command and result decoder. Stable pagination requires an explicit order with a unique tie breaker. Set, grouping and collated comparison behavior requires a matched engine fixture.
4. Execute on the current context or transaction connection. Decode values under the existing tracking/identity resolution policy. A command, cancel, decoder or terminal row error retains its category and cause. A stream closes its owned rows on exhaustion, cancellation, failure or early stop.
5. Materialize or stream across an explicitly selected [client boundary](../adr/0007-linq-execution-boundaries.md). A rejected predicate or projection sends zero commands and runs zero local callbacks.

| Comparison | Required checks on PostgreSQL, SQLite, SQL Server and MySQL |
| --- | --- |
| Nullable operands and join keys | Null comparison, compensated equality, SQL unknown, null buckets and outer-join unmatched rows |
| Text | Declared collation, case and accent sensitivity, Unicode and length semantics |
| Order and duplicates | Tied sort keys, explicit final key, NULL placement, DISTINCT/UNION and duplicate join matches |
| Aggregate and conversion | Empty/all-null inputs, overflow, decimal precision, timestamp precision and time-zone mapping |
| EF extensions | Include shape and identity resolution, filter scope, split vs single commands, raw SQL parameters, bulk DML count and transaction affinity |

Each provider and supported version needs actual rows, decoded values or a typed rejection, bound parameter evidence, command counts and appropriate resource cleanup. An absent capability is visible at validation. Passing one engine or producing a SQL string leaves all other engine results open. The [registry](../parity/linq-operators.json) records source discovery and four independent provider cells. [Queryable specifications](../../specs/features/linq/queryable.feature).
