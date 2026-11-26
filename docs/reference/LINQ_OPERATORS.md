# LINQ operator evidence

The in-memory `Where`, `Select` and `Count` implementations have direct local evidence for filtering, projection, early stop and terminal traversal. The registry also tracks the planned overloads and execution modes. [Runnable examples](../user/LINQ_OBJECTS.md) · [Pipeline evidence](../evidence/W008/OBJECTS_COUNT.md)

Use the operator registry for versioned source members, overloads, local and SQL execution modes, semantics and provider cells. Compare ordering, equality, deferred evaluation, empty input, and overflow at the individual overload level before setting support status.

For local selection, `objects.First` reports presence and `objects.FirstOrDefault` takes an explicit fallback. Compose `Where` for a predicate-based selection. The [selection evidence](../evidence/W008/OBJECTS_FIRST.md) records the equivalent pinned overloads and the Go empty-input contract.

Feature IDs: LINQ-OBJECTS-001, LINQ-QUERYABLE-001, LINQ-CONFORMANCE-001.

- [Canonical design or registry](../parity/linq-operators.json)
- [Example acceptance criteria](../../examples/README.md)

## Registry counts

| Mode | Source member groups | Signatures |
| --- | ---: | ---: |
| Objects | 257 | 411 |
| Expressions | 200 | 406 |
| Queryable | 170 | 470 |

| Queryable provider | Planned groups | Inapplicable groups |
| --- | ---: | ---: |
| mysql | 120 | 50 |
| postgres | 120 | 50 |
| sqlite | 124 | 46 |
| sqlserver | 154 | 16 |
