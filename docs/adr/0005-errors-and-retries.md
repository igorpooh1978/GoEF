# ADR 0005 — Portable errors and outcome-aware replay

**Status:** Proposed W000 architecture. **Scope:** W002 primitives, W005 adapters, W009 resilience and W011 tracked writes. [Bootstrap issue](https://github.com/igorpooh1978/GoEF/issues/2)

A caller needs to distinguish a stale write, a uniqueness violation, a failed transaction and a lost acknowledgement. Identical vendor numbers can require different treatment by operation or configuration. A disconnect after commit dispatch can hide a durable successful write.

Reserve stable portable IDs and sentinels with a typed, redacted detail. Adapter rules use typed native fields plus phase and configuration. Unknown or ambiguous mapping selects the stable unknown classification. Public wrapping preserves causal context errors and secondary cleanup failures. Only safe diagnostics cross the portable boundary.

Keep retry eligibility separate from kind and severity. Whole-unit replay requires a known rollback, a fresh transaction and tracker, an explicitly replay-safe callback and a bounded attempt/deadline budget. Constraint failures, caller cancellation, optimistic conflicts and unknown outcomes suppress automatic replay. A lost commit acknowledgement exposes a reconciliation state. A unique, transactional operation marker and a fresh authoritative read can prove success. The application must keep the result unresolved if the original unit may still complete.

This decision adds per-provider observation work and can require caller intervention after ambiguous completion. It preserves uncertainty that an automatic retry could turn into a duplicate effect. A rule obtains verified status only with exact versioned engine observations, a negative control, redaction checks and integrated test evidence. [Error registry](../errors/README.md) · [Transaction contract](../architecture/TRANSACTION_ENGINE.md) · [Error scenarios](../../specs/features/errors/)
