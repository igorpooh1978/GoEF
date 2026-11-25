---
name: goef-acceptance
description: Independently review a GoEF integrated SHA for issue evidence, EF and LINQ parity, all required providers, coverage, security, docs and GO or NO-GO.
---

# GoEF Acceptance

1. Read the [release gates](../../../docs/development/RELEASE_GATES.md) and [checkpoint](../../../docs/roadmap/CHECKPOINT.md). Work read-only on the exact integrated SHA with independently authored review. Verify scope, issue IDs, contracts, upstream pins and approvals.
2. Trace scenarios to executable baseline/RED/actual GREEN and review output. Validate versions and coverage per owned module and provider. Check real-engine and failure/recovery evidence; a mock, SQL golden or missing engine cannot prove conformance.
3. Verify Objects, Expressions and Queryable separately, including Go compiler proofs, explicit client boundary, generated freshness and actual four-provider semantics. Check transaction outcomes, migration data safety, error mappings, redaction, licenses, English docs and runnable examples.
4. Emit a gate table with evidence path, checked SHA, actual commands/status/exits and blocker or remediation for every missing mandatory item. Return GO only when all required gates hold. Report partial or NO-GO with specific effects. Leave status changes and publication with the authorized owner.

If CI is green for a prior SHA or omitted a required provider, rerun the relevant gate where allowed or report it pending on the integrated SHA.
