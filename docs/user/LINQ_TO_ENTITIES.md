# Compose a database-backed LINQ query

Status: Proposed. The example is a planned acceptance case and has no runnable GoEF API yet.

Keep a typed predicate and ordered projection in the provider plan until execution. The planned fixture observes bound parameters, result ordering and types on each engine. A provider capability miss yields a typed error before issuing SQL.

Feature IDs: LINQ-QUERYABLE-001, QUERY-PARAM-001.

- [Design or source evidence](../architecture/LINQ_QUERYABLE.md)
- [Planned examples and evidence requirements](../../examples/README.md)
