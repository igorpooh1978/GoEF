# Choose a LINQ execution mode

The in-memory `Where` and `Select` example runs against the current source. The comparison with a provider-executed query remains a planned acceptance case. [In-memory example](LINQ_OBJECTS.md)

Use the Objects mode for Go collections, Expressions to represent typed operations, and Queryable to build provider-executed plans. A planned fixture computes the same result from a bounded database query and a local sequence with explicit comparison settings.

Feature IDs: LINQ-OBJECTS-001, LINQ-EXPRESSION-001, LINQ-QUERYABLE-001.

- [Design or source evidence](../upstream/LINQ_RESEARCH.md)
- [Planned examples and evidence requirements](../../examples/README.md)
