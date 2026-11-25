# Coverage gates

Status: Policy. [Issue #9](https://github.com/igorpooh1978/GoEF/issues/9).

- Require at least 95% statement coverage in each owned production module and each required provider. Target at least 95% for critical query, migration, tracking, transaction, error and security packages. Require at least 90% for other non-generated production packages unless a documented, reviewed exception applies.
- Require executable evidence for every mandatory behavior and critical failure in the claimed feature scope. Statement coverage counts statements and cannot establish branch or semantic coverage.
- Record covered and total statements using an explicit repository-owned package set per module. Report generated/platform-specific code, unit results and per-engine integration results separately. For instrumented processes, collect coverage before cleanup and merge only compatible package/build coverage metadata. Keep each provider's numerator and denominator visible.
- Link exceptions to an issue, owner, reason, impact, review, and expiry. Keep unsupported and unrun cells visible. Missing critical recovery and error scenarios cannot be waived by a percentage.

W000 has zero owned production statements and zero GoEF runtime scenarios executed. Coverage is not applicable for W000. The isolated language probe is verification of Go behavior.
