# Move explicitly from SQL to objects

Status: Proposed. The example is a planned acceptance case and has no runnable GoEF API yet.

Apply supported filters, order and limit on the server, materialize the bounded result, and then run a local Go callback. Compare server-executed and locally executed operations separately. Unsupported server operations cannot download a table implicitly.

Feature IDs: LINQ-BOUNDARY-001, QUERY-UNSUPPORTED-001.

- [Design or source evidence](../adr/0007-linq-execution-boundaries.md)
- [Planned examples and evidence requirements](../../examples/README.md)
