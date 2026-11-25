---
name: goef-security-review
description: Review GoEF changes for SQL injection, tenant and filter isolation, privacy, secret handling, migration safety, parser inputs and dependency provenance.
---

# GoEF Security Review

1. Read the [security and lifecycle contract](../../../docs/architecture/SECURITY_AND_LIFECYCLE.md) and [security policy](../../../SECURITY.md). Identify trust boundaries in query building, raw SQL, generated inputs, logging, diagnostics, migration targets and provider credentials.
2. Verify value parameterization, identifier validation/quoting, default tenant filter behavior including joins/subqueries, and explicit opt-out scopes. Exercise negative cases with fictional fixture values and inspect redacted traces.
3. Check least privilege, ownership, resource cleanup, bounded operations, replay of side effects, provider limitations and threat-relevant dependency/license approvals. For destructive schema operations verify explicit target, approval, preservation/recovery evidence and isolation.
4. Report severity, scope/file, triggering behavior, observed consequence and corrective test without publishing private material or unsafe reproduction data. Route credible security findings through the disclosure policy.

If a parameterized predicate is combined with a caller-controlled sort identifier, verify that the identifier is separately constrained or quoted.
