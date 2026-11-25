# ADR 0004 — Versioned migrations and recovery

**Status:** Proposed W000 architecture. **Scope:** W004 foundation, W006 executor and W016 tooling. [Bootstrap issue](https://github.com/igorpooh1978/GoEF/issues/2)

A desired model, a released snapshot and a live database answer different questions. Applying a fresh model diff directly to production would lose the reviewed artifact identity and could misinterpret a rename as destruction. Migration history alone cannot establish whether an interrupted DDL command changed the schema.

GoEF will accept manual Go definitions and generated operations through the same in-process registry. Require durable operation identity, dependency ordering, source/target hashes, provider capability and explicit reversal metadata. Scaffolding produces reviewable source and snapshots. The optional CLI delegates to the library. A versioned plan binds approvals to the operation, digest and database. The executor acquires the appropriate lock and rechecks the live schema, applied lineage and prerequisites.

Use a single atomic group including history where supported. For implicit commits and transaction-suppressed commands, persist intent, inspect the effect and checkpoint before continuing. Unknown outcomes require reconciliation with durable schema, history and data. A recovery procedure may pause for reviewed repair. It can resume an independently proven unapplied operation only with a safe replay contract. Cross-provider acceptance must exercise the failure between DDL and its journal acknowledgement. The tradeoff is extra inspection, persistent journal state and explicit destructive approval. Applications gain a verifiable route from partially applied changes back to a known target.

[Migration engine and provider contracts](../architecture/MIGRATION_ENGINE.md) · [Gherkin cases](../../specs/features/migrations/)
