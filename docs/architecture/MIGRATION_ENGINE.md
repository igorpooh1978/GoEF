# Migration engine

**W000 proposed contract.** A Go application can author, scaffold, inspect, script and apply migrations using library calls alone. Schema comparison, planning and durable execution have separate inputs and permissions. W004 develops artifacts and physical diff. W005 establishes transaction/connection affinity before the W006 executor. [Safety decision](../adr/0004-migration-model-and-safety.md) · [Initial migration scenarios](../../specs/features/migrations/)

## Read the right state

| Question | Comparison | Result |
| --- | --- | --- |
| What changed in the model? | Versioned last released snapshot → current desired model | Deterministic provider-resolved physical operations, generated Go definition and target snapshot. Identical inputs produce an empty diff. |
| Which migrations are pending? | Namespaced durable history → ordered registered definitions | A path to the target, or a conflict for duplicate IDs, changed digests, missing predecessors, cycles or a divergent branch. |
| Has the live schema drifted? | Expected snapshot for the **applied** prefix → normalized inspection of the target | Physical differences in supported objects, including dependencies. Unknown objects affecting the plan block unsafe reconstruction. |

Stable logical identities or an author-reviewed old/new hint establish renames. An unexplained removal and addition are flagged for review. Destructive approval binds the operation IDs, exact plan digest and specific database identity. It expires when any of them changes. Design-time comparison never updates a deployed schema. [EF model differ](https://github.com/dotnet/efcore/blob/b34c0e9d7e768aa808ded44f19e7f871bea6869f/src/EFCore.Relational/Migrations/Internal/MigrationsModelDiffer.cs) · [Scaffolder](https://github.com/dotnet/efcore/blob/b34c0e9d7e768aa808ded44f19e7f871bea6869f/src/EFCore.Design/Migrations/Design/MigrationsScaffolder.cs)

## Author and plan

A manual Go definition has an immutable ID, dependencies, Up, Down or a declared irreversible boundary, source/target model hashes and capability requirements. Builders produce typed operations. Authored and scaffolded definitions share one registry and order. A versioned canonical manifest includes parameters, provider annotations, stable order and raw SQL/source identity or an explicit digest for custom code. Function addresses, timestamps and map iteration order cannot enter content identity. Appending a forward correction preserves released artifacts.

The initial operation vocabulary covers schemas, tables, columns, sequences, keys, constraints, indexes, computed/default/identity/precision/collation changes, views, triggers, raw operations and data shifts. Order creates and drops by actual dependencies. Detect cycles or allocate deterministically reviewed intermediate names. Each command declares pre/postconditions, risk, locking scope, transaction mode and reversibility. Custom SQL/data work requires an explicit version, effect contract and recovery check. Backfills use deterministic predicates and bounded durable batch checkpoints. Expand → backfill → validate → contract steps permit compatibility checks against both application versions. Down that deletes data requires destructive approval. An unprovable reversal returns `ErrIrreversibleMigration`.

| Library entry point (names proposed) | Expected effect |
| --- | --- |
| Diff, Scaffold, HasPendingModelChanges | Compute artifacts from known model inputs. Return files/bytes or a result for author review. |
| Inspect, List, Applied, Pending, Verify | Read live schema, lineage, checksums and outstanding journal entries; report structured diagnostics. |
| PlanTo, GenerateScript, GenerateBundle | Resolve target, history, provider options, prerequisites and risks. Dry-run reads only. Idempotent scripts require an advertised capability. Bundles pin artifacts and describe native prerequisites. |
| Apply, Migrate, Recover | Obtain the scoped lock, revalidate the target and reviewed plan, and execute or explicitly reconcile. Startup migration is opt-in with separate credentials and availability policy. |
| Baseline | Record a verified legacy schema match or an explicitly reviewed mapping. Development create/delete utilities require separate lifecycle handling and cannot silently initialize versioned history. |

An optional CLI, IDE or go generate frontend calls these APIs. Library callers never need a binary or subprocess. Compare CLI and library plans, scripts, exit classification and resulting state for identical inputs. Scripts and bundles remain generated artifacts until explicitly executed. [EF migration management](https://learn.microsoft.com/en-us/ef/core/managing-schemas/migrations/managing) · [Application options](https://learn.microsoft.com/en-us/ef/core/managing-schemas/migrations/applying)

## Execute under one lock

1. Pin the target identity, expected history, plan digest, dependencies and approval. Acquire a bounded, cancellable lock covering all conflicting objects and histories in that database. Hold through all commands, checkpoints and final history. Shared objects in separate contexts require the same conflicting lock domain.
2. Reread history, journal and normalized schema under lock. Reject changed checksums or branch shape with `ErrMigrationConflict` and mismatched live schema with `ErrSchemaDrift`. A waiting runner that sees the exact reviewed migration already applied verifies its target and returns a no-op.
3. Group transactional commands and their completion history in a transaction where the provider supports the whole group. Verify durable schema and data after commit. On acknowledged rollback the previous state stands. An unknown commit response enters reconciliation.
4. For each transaction-suppressed or implicit-commit command, durably record intent **before dispatch**. Journal migration ID, operation ID and digest, owner generation, expected predecessor, precise pre/postconditions and batch checkpoint. Check the actual effect and durably mark it confirmed. Keep final history unpublished until every command and postcondition is confirmed.
5. Finalize `__goef_migrations_history` (configurable) once per context/set, recording ID, dependencies, source/target hash, artifact digest, provider/engine version and audit metadata. Release only the current owner's lock. A broken session requires a new lock and full state inspection before continuing. Prove owner death and validate durable state before reclaiming any persistent lock.

Use a dedicated migration connection. Connection-owned locks stay on it across commits. A transaction-scoped lock alone cannot protect a chain containing nontransactional commands. Reject incompatible caller transactions before execution. Use finite wait budgets and distinguish cancellation from lock timeout. [EF migrator](https://github.com/dotnet/efcore/blob/b34c0e9d7e768aa808ded44f19e7f871bea6869f/src/EFCore.Relational/Migrations/Internal/Migrator.cs) · [Migration command executor](https://github.com/dotnet/efcore/blob/b34c0e9d7e768aa808ded44f19e7f871bea6869f/src/EFCore.Relational/Migrations/Internal/MigrationCommandExecutor.cs)

## Decide after interruption

| Evidence found by a fresh lock owner | Permitted action |
| --- | --- |
| Exact target/history plus expected data effects | Report applied. Preserve the existing history entry. |
| Journal intent, independently proven postcondition and unique effect identity | Mark that operation confirmed. Continue with the first remaining validated operation. |
| Original precondition, no effect, prior session is final, and replay contract holds | Resume the unapplied step. |
| Partial object definition, uncertain data transformation, changed identity or unavailable authority | Return `ErrMigrationRecoveryRequired`. Preserve the journal, report safe IDs and require a reviewed repair plan. |

An absent history row only identifies an unfinished migration. A reversible DDL statement can still destroy values. Preserve a recovery snapshot or mark that path irreversible. Process restarts, canceled lock waits, statement failures, disconnects before/after dispatch and lost commit acknowledgements each require independent history, schema and row assertions. [Transaction outcome contract](TRANSACTION_ENGINE.md)

## Provider-specific evidence

| Provider | Planned acceptance |
| --- | --- |
| PostgreSQL | Run CREATE INDEX CONCURRENTLY outside transactional groups. Inspect index validity and definition after interruption. Check session-owned advisory lock continuity, timeout, connection release and PgBouncer mode if advertised. [CREATE INDEX](https://www.postgresql.org/docs/18/sql-createindex.html) |
| SQLite | Inventory dependencies before rebuild. Copy explicit writable columns into a replacement table, restore generated/sequence semantics, rebuild indexes and triggers, validate foreign keys and swap transactionally where supported. Check connection-local PRAGMAs before and after, failure halfway, process death, retained old data and unknown object rejection on a file-backed database. [SQLite ALTER TABLE](https://www.sqlite.org/lang_altertable.html) · [EF SQLite generator](https://github.com/dotnet/efcore/blob/b34c0e9d7e768aa808ded44f19e7f871bea6869f/src/EFCore.Sqlite.Core/Migrations/SqliteMigrationsSqlGenerator.cs) |
| SQL Server | Discover actual default constraint names and dependencies. Exercise rename, computed columns, rowversion and identity changes with populated tables. Confirm emitted DDL, data and usable transaction state on the selected version. [ALTER TABLE](https://learn.microsoft.com/en-us/sql/t-sql/statements/alter-table-transact-sql?view=sql-server-ver16) |
| MySQL | Record engine, server/sql_mode, and supported ALGORITHM/LOCK choices. Journal implicit-commit DDL step by step, retain a session-scoped lock across groups and recheck metadata locks/session identity on reconnect. Inspect actual data and target objects after a crash between DDL and checkpoint. [Implicit commit statements](https://dev.mysql.com/doc/refman/8.4/en/implicit-commit.html) |

For **each** real engine exercise empty → current, seeded previous → current, down/up, no change, pending model change, rename, backfilled required column, invalid type/constraint, composite PK/FK, unique and indexes, branch/checksum conflict, drift, both authoring paths, dry run, competing processes, cancellation, interrupted DDL, unknown commit, unsupported capability, seed replay and unsafe downgrade. Include deterministic generation, script/bundle where supported, CLI/library equivalence and database-first baseline/round trip. Inspect schema, constraints, existing values, history, journal, locks and connection cleanup after each run. [EF relational migration specification tests](https://github.com/dotnet/efcore/blob/b34c0e9d7e768aa808ded44f19e7f871bea6869f/test/EFCore.Relational.Specification.Tests/Migrations/MigrationsTestBase.cs)
