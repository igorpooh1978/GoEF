# Plan and apply migrations in Go

Use `migration.Pending(registered, applied)` to check an ordered migration registry against the history supplied by the caller. The package requires Go 1.27.4 and works in memory.

The runnable example registers two synthetic artifacts and supplies an applied history containing the first entry. `Pending` returns `002_customer_notes`. The example then changes the first history digest and handles the conflict with `errors.Is(err, goeferr.ErrMigrationConflict)`. [Example source](../../migration/example_test.go) · [Issue #34](https://github.com/igorpooh1978/GoEF/issues/34)

```text
002_customer_notes
review the applied migration history
```

Run the example with `go test -mod=readonly -run '^ExamplePending$' ./migration`.

## Supply the lineage

- Give every entry a unique, case-sensitive UTF-8 ID containing at least one non-whitespace character. GoEF compares the exact bytes and preserves whitespace within valid IDs.
- Leave the first predecessor empty. Set each later predecessor to the preceding entry's exact ID.
- Supply the nonzero SHA-256 digest of each canonical artifact. Supply the recorded ID, predecessor and digest for every applied entry in history order. The application chooses the artifact representation and computes its digest.
- Keep both input slices stable during the call. The result contains independent copies of the pending entries in registry order. An empty history returns the full registry. A complete history or empty lineage returns nil.

A malformed registry or a history that differs from the exact prefix produces `GOEF-MIG-0002` in phase `verify-history`. The detail records `no_commit_attempted` and retry class `never`. Inspect the registered artifacts and recorded history before proceeding. The public error omits the supplied IDs, predecessors and digests. [Error registry](../errors/README.md) · [Verification](../evidence/W004/MIGRATION_PENDING.md)

## Go-authored migration operations

The proposed authoring API defines ordered operations and reviews rename intent, rollback feasibility and checksums before execution. Schema inspection, model diffs and provider-specific execution have separate contracts. [Migration engine design](../architecture/MIGRATION_ENGINE.md) · [Planned runnable examples](../../examples/README.md)

## Apply a reviewed SQLite artifact

Use [migration/sqlite.Apply](../../migration/sqlite/apply.go) with a dedicated SQLite pool, a caller deadline and a finite busy timeout. Set the file's nonzero application ID when provisioning it. Use read-only source and target checks that also verify the identity and dependency inventory specific to the intended file. Check the schema and values that distinguish a safe first application, a completed application and an ambiguous result.

Register the complete immutable lineage through the artifact being applied. The executor takes the SQLite writer lock before verifying durable history and the source or target check. It applies one artifact whose predecessors are already present. It creates and maintains the reserved table `main.__goef_migrations_history` in the same transaction as the reviewed statements. Supply the digest of the reviewed operation and check definitions, and keep the lineage and callbacks stable until the call returns. The run callback uses the supplied connection synchronously. Close open rows before returning and leave transaction control, connection settings and history to the executor.

A confirmed new commit returns `true`. A matching applied history and verified target return `false, nil`. Inspect the error even when the result is `true`, since restoring settings or releasing the lease can fail after a confirmed commit. An unconfirmed commit returns `goeferr.ErrCommitOutcomeUnknown` and discards the connection. Inspect authoritative state before considering another apply. A changed source, a mismatched application ID or an unverifiable target returns `goeferr.ErrMigrationRecoveryRequired`. A conflicting digest or lineage returns `goeferr.ErrMigrationConflict`. An expired native writer wait returns `goeferr.ErrMigrationLocked`.

The [executable rebuild example](../../test/conformance/driverlab/sqlite_executor_rebuild_test.go) preserves values, generated columns, sequence state, dependencies and connections across an interruption. Run it on both pinned SQLite drivers with `python -B test/conformance/lab/sqlite_compat.py`. [Verification](../evidence/W002/SQLITE_EXECUTOR_REBUILD.md) · [Issue #13](https://github.com/igorpooh1978/GoEF/issues/13)
