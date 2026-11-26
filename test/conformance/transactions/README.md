# Transaction conformance

W000 defines the test protocol and initial scenario IDs. The first owned transaction fixture runs `TX-OWN-001-S02` through the public GoEF API with the local driver lab and its pinned builds. A one-connection worker and an independent observer check physical affinity, committed and rolled-back rows, terminal handle rejection, and return to the pool. The remaining experiments below describe broader conformance. Use one disposable database per run and an isolated file for SQLite. Include all four providers in the denominator. Record unsupported configurations with the specific rejection test and a vendor reference. [Behavior specifications](../../../specs/features/transactions/) · [Engine contract](../../../docs/architecture/TRANSACTION_ENGINE.md) · [Local driver lab](../driverlab/README.md) · [Owned transaction fixture](../driverlab/owned_transaction_test.go)

Run the owned transaction fixture on the four default profiles from the repository root.

```powershell
pwsh -NoProfile -File scripts/ci/goef-w002.ps1 -OwnedTx -AcceptSqlServerEula
```

Add `-Compatibility -AcceptLegacySqlServerEula` to include the other pinned profiles when both SQL Server Developer releases are authorized for the run. The corresponding Python selector is `--owned-tx`. [Local driver lab](../driverlab/README.md)

The manual `TestSavepointAndExternalTransaction` fixture checks earlier writes, younger point invalidation, owner visibility after participant detach, and rollback observed independently. Run it on the four default profiles with the following command.

```powershell
pwsh -NoProfile -File scripts/ci/goef-w002.ps1 -SavepointTx -AcceptSqlServerEula
```

The corresponding Python selector is `--savepoint-tx`. The compatibility flags select the same additional profiles as the owned-transaction run. These scenarios exercise explicit DML and savepoint primitives. [Savepoint scenarios](../../../specs/features/transactions/tx-savepoint-001.feature) · [External ownership scenarios](../../../specs/features/transactions/tx-own-001.feature) · [Local driver lab](../driverlab/README.md)

Run the ordered `SaveChanges` fixture on the four default profiles with the following command. Add the compatibility flags above to include the other pinned profiles. The corresponding Python selector is `--save-changes`. The fixture checks real partial-write failure, earlier writes, staged values and opt-out before writes. Automatic graph discovery, MARS, distributed transactions and real doomed-transaction recovery retain separate cases. [SaveChanges evidence](../../../docs/evidence/W005/SAVE_CHANGES.md)

```powershell
pwsh -NoProfile -File scripts/ci/goef-w002.ps1 -SaveChanges -AcceptSqlServerEula
```

Run the owned commit acknowledgement fixture against the default four profiles. The Python selector is `--commit-outcome`. The compatibility flags above extend the same checks to the other pinned profiles. [Commit outcome fault evidence](../../../docs/evidence/W002/COMMIT_OUTCOME_FAULT.md)

```powershell
pwsh -NoProfile -File scripts/ci/goef-w002.ps1 -CommitOutcome -AcceptSqlServerEula
```

| Experiment | Coordination and assertions |
| --- | --- |
| Ownership and atomic save | Open two independent sessions. Confirm a committed insert survives and a rolled-back insert disappears. Force the second command of a graph save to fail after key generation. Verify actual parent/child counts, original versions, temporary keys and tracker states. Repeat inside an external transaction and observe its owner's subsequent commit/rollback. |
| Physical affinity and exclusion | Limit the worker pool to one connection, use a separate observer. Exercise split reads, explicit/lazy loads, stream, raw SQL, bulk write and SaveChanges. Compare physical session IDs and pending rows; for blocking isolation bound waits explicitly. Cancel mid-stream, check the terminal error, close twice, and acquire the pool slot again. |
| Isolation, savepoint, nesting | Use barriers around a reader snapshot and a second writer's commit. Record isolation configuration and visibility or lock waits on each engine. Try supported and unsupported modes, read-only writes and nested begin. After a savepoint rollback, commit an earlier marker. Compare durable rows on another connection. |
| Retry eligibility | Induce a two-writer deadlock/serialization conflict or an engine-specific busy alternative via event coordination. Capture native fields and full rollback, fresh transaction identity, attempt count, elapsed deadline and the final effect count. Also test stale version, unique/FK violations, cancellation during backoff and a callback lacking a replay contract. |
| Lost commit acknowledgement | Place an adapter completion seam after a real database commit and before delivering its acknowledgement. Force a transport-shaped failure only there. Run a separate control losing the connection before COMMIT dispatch. Verify markers, data, tracker and callback counts via a fresh primary connection. A synthetic fault establishes control flow only; real native error mappings require independently reproduced errors. |
| Cleanup and migration interaction | Cancel while acquiring/reading and during a savepoint or lock wait. Force cleanup failure after the primary error. Inspect joined errors, close or discard behavior, lock release, pool in-use baseline and a subsequent successful checkout. Reject a migration with transaction-suppressed DDL inside a caller's data transaction before changing rows or history. |

For native failures collect engine and driver build, configuration flags, deterministic schedule, operation/phase, typed fields, returned kind/ID, retry count, actual transaction state and redacted before/after evidence. Keep secrets, connection strings and customer data out of evidence. Link each executed case to its scenario, test, issue and reviewed integrated SHA. A driver error constructed in a unit test, an inferred SQL string or a sleep-only schedule cannot substantiate a provider claim. [EF Core transaction specification tests](https://github.com/dotnet/efcore/blob/b34c0e9d7e768aa808ded44f19e7f871bea6869f/test/EFCore.Relational.Specification.Tests/TransactionTestBase.cs)
