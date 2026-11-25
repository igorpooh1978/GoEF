# ADR 0006 — Issue-linked documentation and behavior evidence

Status: Adopted for bootstrap. [Issue #9](https://github.com/igorpooh1978/GoEF/issues/9).

Adopt stable feature and scenario IDs in [Gherkin](../../specs/README.md). Connect each scenario to the applicable engine and driver, executable test, user guide, and evidence revision. Record failing RED before a behavior change, or exact characterization where behavior already exists. Rerun on the integration revision before independent acceptance.

Considered using only Go test names or an issue checklist. Go test names can change and omit documentation or provider coverage. An issue alone cannot prove runtime behavior. A maintained cross-reference lets reviewers detect orphan IDs, missing cases, and stale evidence.

Documentation and example updates ship with the behavior. The bootstrap validator proves artifact integrity. The executable behavior suite follows the [test policy](../development/TESTING.md).
