# LINQ operator evidence

Status: Proposed. This reference describes the intended contract. Its usage examples are planned.

Use the operator registry for versioned source members, overloads, local and SQL execution modes, semantics and provider cells. Compare ordering, equality, deferred evaluation, empty input, and overflow at the individual overload level before setting support status.

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
