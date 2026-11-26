# Plan pending migrations in Go

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

The proposed authoring API defines ordered operations and reviews rename intent, rollback feasibility and checksums before execution. Schema inspection, durable history loading, model diffs and applying operations have separate contracts. [Migration engine design](../architecture/MIGRATION_ENGINE.md) · [Planned runnable examples](../../examples/README.md)
