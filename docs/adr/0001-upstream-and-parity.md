# ADR 0001 — Versioned behavioral references

Status: Adopted for research. [Issue #7](https://github.com/igorpooh1978/GoEF/issues/7).

Use pinned upstream source and tests as behavioral evidence. Treat the Go library as independent work and compare observable results and failure states. An unpinned reference can change independently of a GoEF claim, and a source port would import .NET-specific design and additional licensing obligations.

The [source lock](../upstream/lock.json) keeps release and research revisions distinct. Record exact paths and tests independently of a source's license. Link each behavior to the feature registry, applicable providers, an issue, and an acceptance case. Surface semantic deviations and the remaining enumeration denominator. Rebaseline through an issue that compares source differences, updates affected claims, records license review, and reruns acceptance.
