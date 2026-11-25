---
name: goef-docs
description: Update GoEF English Godoc, Markdown guides, API references, examples and skill instructions with behavior changes. Check links and generated freshness.
---

# GoEF Docs

1. Read the [documentation policy](../../../docs/development/DOCUMENTATION_POLICY.md). Inventory public APIs, scenarios, guides, examples, errors, matrix cells, ADRs and affected Skills that share the issue.
2. Write examples first. Link the canonical Go sample and actual expected result, prerequisites and versions, a typed failure, cancellation, ownership and limitations. For LINQ, check separate local and server guides, their explicit conversion boundary and the operator reference.
3. Update Godoc and authoritative Markdown alongside the testable behavior. Change generator inputs for generated artifacts. Verify code, snippets and synced outputs at the pinned toolchain, and check links, language, support claims and traceability.
4. Label prospective APIs and unexecuted samples as proposed. Record evidence, status and blocked commands. Request docs review on the integrated SHA.

If an example still uses a removed method, repair the canonical example and all copied excerpts before marking the guide verified.
