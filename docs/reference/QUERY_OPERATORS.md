# Query operators

Status: Proposed. This reference describes the intended contract. Its usage examples are planned.

Server operators form an immutable plan. Filter binds captured values and validates identifiers; projection returns a typed row; terminal operators enforce cardinality; ordering and pagination have explicit prerequisites. Unsupported operations produce typed failures before execution.

Feature IDs: QUERY-PARAM-001, QUERY-UNSUPPORTED-001.

- [Canonical design or registry](../architecture/QUERY_ENGINE.md)
- [Example acceptance criteria](../../examples/README.md)
