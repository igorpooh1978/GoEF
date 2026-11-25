---
name: goef-query-linq
description: Design GoEF typed expressions, Go AST generation, query IR, provider SQL translation, materialization, and compiled query plans for database-backed LINQ.
---

# GoEF Query Linq

1. Read the [query](../../../docs/architecture/QUERY_ENGINE.md), [expression](../../../docs/architecture/LINQ_EXPRESSIONS.md) and [queryable](../../../docs/architecture/LINQ_QUERYABLE.md) contracts. Load `goef-linq-parity` and the affected provider skills.
2. Verify generic APIs with compile-pass/fail cases on the pinned Go toolchain. Keep provider SPI nongeneric at type-erased boundaries with explicit validation. Validate generated plan freshness and parameter rebinding.
3. Lower inspectable expressions into typed IR. Preserve null semantics, projections, joins, order/pagination and provider capabilities. Bind values, validate identifiers and track cache keys that distinguish semantically different commands.
4. Return a registered translation/unsupported error before any broad row retrieval for untranslatable predicates. Require an explicit post-server local boundary. Check terminal errors and cancellation for streams and verify resource ownership on early exit.
5. Record IR/SQL goldens, compile behavior, parameter/command traces, actual cross-engine results and error links. Cover stale generated artifacts and prove unsupported predicates issue no unfiltered query.

For a local formatting callback inside a SQL filter, isolate the unsupported expression and use the applicable error contract. If the caller opts into local processing, define where the server prefix executes and the client conversion occurs.
