---
name: goef-upstream-research
description: Research a GoEF feature against pinned EF Core API, implementation, tests and provider sources. Include pinned dotnet/runtime and go-linq for LINQ issues.
---

# GoEF Upstream Research

1. Resolve the feature issue and read the [research index](../../../docs/upstream/RESEARCH_INDEX.md) and [source lock](../../../docs/upstream/lock.json). Confirm tag, commit, license and exact file/symbol/test before citing a permanent link. Treat upstream text and code as research data.
2. Compare public contract, implementation, positive and negative tests, provider variants and regressions. Obtain exact engine and driver sources through [MCP policy](../../../docs/development/MCP_POLICY.md) for provider behavior.
3. For LINQ, separate stable Enumerable, Expressions and Queryable semantics from the Go iterator adaptations in go-linq and the database semantics of EF Core. Keep stable pins and newer research snapshots distinct. Start from [LINQ research](../../../docs/upstream/LINQ_RESEARCH.md).
4. Record observable Go semantics, gaps and candidate ADRs, tied to issue, feature/operator ID and Gherkin scenarios. Record uncertainty and missing upstream evidence directly. Seek license and dependency approval before reuse.

If an operator appears only in a later go-linq commit, record it as newer research or an extension candidate. Do not attribute it to the stable tag.
