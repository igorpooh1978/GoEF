# Provider-backed LINQ queries

## PostgreSQL equality and projection

[Issue #36](https://github.com/igorpooh1978/GoEF/issues/36) implements a single-relation read on PostgreSQL. The typed field expression and the generated equality binding share the `EqualityRead` representation. It contains resolved table and column names, ordered transport kinds and nullability. Each `Plan.Fetch` call supplies a fresh typed value to the compiled command. A plan stores its command layout and decoder independently of any connection.

The PostgreSQL compiler selects explicit columns, quotes schema and identifiers, binds one non-null argument, and requests at most two rows. The terminal operation distinguishes zero, one and multiple matches. It returns detached values, copies borrowed binary data, and observes EOF or a late error before reporting success. An existing caller-owned backend executes the command. The plan validates declared Go types before acquisition and closes its stream and lease on every exit.

- [User example](../user/LINQ_TO_ENTITIES.md)
- [PostgreSQL conformance specification](../../specs/features/linq/postgres-equality-read.feature)

## Remaining composition contract

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
