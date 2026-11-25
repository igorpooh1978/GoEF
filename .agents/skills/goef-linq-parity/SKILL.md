---
name: goef-linq-parity
description: Inventory LINQ operator and overload parity across Objects, Expressions, Queryable and providers. Classify source provenance, support and explicit client conversion boundaries.
---

# GoEF Linq Parity

1. Read [master plan §21](../../../docs/roadmap/MASTER_PLAN.md), the [operator registry](../../../docs/parity/linq-operators.json) and [operator reference](../../../docs/reference/LINQ_OPERATORS.md). Link one record per testable operator/overload family and execution mode to feature, issue, Gherkin and evidence IDs.
2. Verify pinned .NET and go-linq provenance and EF Core behavior. Separate later upstream from stable baselines. List expected differences in cardinality, order, equality, grouping, nullable logic and empty aggregates.
3. Mark cells separately for Objects, Expressions, Queryable and actual provider versions. Preserve typed unsupported capability and explicit client boundaries. Include negative translation and command-count evidence before claiming any SQL parity.
4. Use local differential fixtures for local semantics and equivalent EF plus real-engine fixtures for SQL. Report open gaps, unsupported cells, unknowns and denominators. Update matrix, docs and examples in the same issue. Give each changed mode and version cell its exact upstream provenance, support status, and evidence or next proof.

When one provider's collation or null join behavior differs, record a provider/version distinction even if the Object output and other providers match.
