# Parity registry

[Feature records](features.json) | [JSON Schema](feature.schema.json) | [Open gaps](GAPS.md) | [LINQ operators](linq-operators.json) | [Research index](../upstream/RESEARCH_INDEX.md)

A record represents observable behavior with a permanent, uppercase feature ID. A portable record has four provider cells. An engine-specific or provider-independent record still has four cells, with an applicability reason wherever a cell does not apply. In each applicable cell, record concrete engine and Go driver versions when they are established. As soon as a feature covers multiple supported version combinations, the linked conformance report must include an explicit result for each one.

Use `origin` for EF Core behavior, the .NET LINQ reference or a GoEF extension. Use `scope` to identify portable versus specialized contracts. An `upstream` source record binds to one lock ID and gives the upstream source and test symbols. Its `relation` distinguishes the relevant behavior, an analogue, or a divergence. A supporting analogue cannot establish parity on its own. The object, expression and queryable operator inventory links back to the canonical features in its own matrix.

- Use `DISCOVERED` for a sourced behavior awaiting its specification and `SPECIFIED` when the Gherkin scenario exists.
- Record `RED` only with a reproducible failing test linked to the scenario. Use `IMPLEMENTING` for active work and `GREEN` after executable checks pass.
- Record `VERIFIED` only with independent review and completed conformance at the integrated SHA. Link the exact SHA and result artifacts. Provider `VERIFIED_UNSUPPORTED` needs a tested typed unsupported result, applicable version evidence and a reviewed deviation.
- Record `PARTIAL`, `BLOCKED`, `DEFERRED` and `NOT_APPLICABLE` with reasons. Retain the issue and target wave for outstanding work. Verified unsupported and inapplicable cells do not count toward supported parity.

Validate JSON Schema draft 2020-12 and cross-file traceability. Check unique feature IDs and scenario IDs, valid issue tracking, dependency existence and acyclicity, exact pins and file paths, local docs and ADR links, status and evidence consistency, and exact scenario-to-test correspondence before making acceptance claims. The [GCT development policy](../../specs/README.md), [traceability index](../../specs/traceability.json) and [bootstrap validator](../../test/bootstrap/README.md) own the cross-file checks. Report counts and denominators by family, origin and provider including each missing or blocked cell. Commit registry updates with the corresponding scenarios, docs, tests and evidence.
