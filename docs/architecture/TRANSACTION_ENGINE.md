# Transaction engine

The `transaction` package implements owned transactions, explicit external enlistment and manual savepoints in W005. W005 also covers broader transaction options and provider hooks. W006 uses transaction lifetimes for migrations, W008 binds runtime queries, W009 exercises advanced replay and W011 integrates the change tracker. [User guide](../user/TRANSACTIONS.md) · [Transaction scenarios](../../specs/features/transactions/) · [Conformance protocol](../../test/conformance/transactions/README.md) · [Issue #2](https://github.com/igorpooh1978/GoEF/issues/2)

## Implemented owned transaction boundary

`transaction.Begin` acquires a dedicated `database/sql.Conn` from the application pool and opens a native transaction with default options. The GoEF handle keeps that checkout until it publishes a terminal outcome. `Exec` sends all commands through the bound native transaction. The package imports the standard library SQL APIs, and the application selects the driver and owns the pool. The provider execution SPI remains available to other subsystems. [Executing Go transactions](https://go.dev/doc/database/execute-transactions) · [Go 1.27.4 SQL implementation](https://github.com/golang/go/blob/go1.27.4/src/database/sql/sql.go)

One operation at a time can use the native handle. An overlapping `Exec`, `Commit` or `Rollback` call returns `ErrContextBusy`. `Close` waits for the active operation and rolls back if the handle is still active. The lifetime context cancels the active command and initiates rollback and release. Explicit completion checks the lifetime before sending commit. A native commit already in progress can still complete. `database/sql` exposes context-free finalizers, so cleanup waits for the driver to return.

| Terminal outcome | Lease and handle behavior |
| --- | --- |
| Commit acknowledged or rollback confirmed | Release the checkout so the pool can reuse it. Commands and explicit completion through the handle return `ErrInvalidState`. |
| Commit error with unconfirmed result | Report `StateOutcomeUnknown` and `ErrCommitOutcomeUnknown`. Discard the checkout and leave reconciliation to the caller. |
| Rollback failure or unusable connection before commit | Discard the connection and record `StateFailed`. An explicit completion call returns any error it observes. Cancellation-driven rollback publishes its state for inspection. |

The behavior of owned transactions is exercised by `TX-OWN-001-S02` through a one-checkout worker and an independent observer on each engine. [Ownership scenario](../../specs/features/transactions/tx-own-001.feature)

## Implemented external enlistment

`transaction.NewExternal` accepts one existing caller-owned `*sql.Tx`. Participants obtained through the same `External` coordinator dispatch through exactly that native transaction and share its operation gate. Statement errors and cancellation leave completion to the native owner. A participant's `Close` detaches just that participant. Closing the coordinator waits for the current managed operation and invalidates all managed handles. Both forms of close leave the native transaction and connection with the caller.

The application uses one coordinator per native transaction and serializes its direct native calls with managed calls. The owner closes the coordinator before native completion. When the owner completes directly, GoEF can observe that outcome on a later native dispatch. The ownership fixture checks that the owner can read pending rows after participants detach, roll those rows back, and keep using its connection. [Ownership scenario](../../specs/features/transactions/tx-own-001.feature) · [EF Core external transaction reference](https://learn.microsoft.com/en-us/ef/core/saving/transactions#cross-context-transaction)

## Implemented manual savepoints

An owned `Tx` and a borrowed `External` each hold a scoped stack of savepoint handles. Savepoint operations pass through the same gate as transaction work. The package generates private names within SQL Server's 32-character limit. A successful rollback to an active point undoes work after the point and invalidates younger handles. Releasing the innermost point keeps its work in the enclosing transaction. SQL Server has no native release statement, so release there retires only the handle. The owner chooses the dialect and uses DML that preserves the transaction boundary. [User guide and dialect rules](../user/TRANSACTIONS.md#use-savepoints-inside-a-transaction)

The manual fixture checks preservation of earlier writes and invalidation of younger handles. The tracked save protocol below adds tracker recovery, capability negotiation and savepoint use around `SaveChanges`. [Savepoint scenarios](../../specs/features/transactions/tx-savepoint-001.feature)

## Ownership across three lifetimes

| Resource | Owner and completion |
| --- | --- |
| Pool and borrowed connection | An application owns its pool. A library transaction returns its lease after completion and cleanup. A caller-supplied connection stays open for its owner. |
| Owned database transaction | Begin binds the physical connection. Explicit isolation and read-only options are planned. Only the owning handle can commit or roll back. Close requests rollback on an active handle and is idempotent. |
| External transaction | Explicit enlistment borrows the exact native transaction and its physical connection. Participants detach without completing the transaction or closing the connection. Future tracked participants will need an owner signal or a reload before accepting provisional state. |
| DbContext and tracker | A context has a single active operation. A stream holds that slot until close or terminal exhaustion. An enlisted transaction also serializes access across participating contexts. Concurrent use returns `ErrContextBusy`. Close ends the context lifetime. |

A bound executor supplies the same transaction to sets, scalar queries, split/follow-up loads, SaveChanges, bulk commands and raw SQL. An incompatible enlistment fails before dispatch. A connection pool with capacity one is an acceptance fixture for accidental out-of-transaction calls. Isolation of the independent observer is explicitly controlled. Ambient or cross-resource enlistment and nested independent begin return `ErrUnsupportedFeature`. An explicit savepoint is the available scoped alternative where the provider supports it. [Go transaction API](https://go.dev/doc/database/execute-transactions) · [EF relational transaction ownership](https://github.com/dotnet/efcore/blob/b34c0e9d7e768aa808ded44f19e7f871bea6869f/src/EFCore.Relational/Storage/RelationalTransaction.cs)

## Completion and tracker state

| Event | Transaction state | Observable tracker and cleanup behavior |
| --- | --- | --- |
| Begin succeeded | active | Snapshot original versions, temporary keys and graph links before commands that may alter them. |
| Flush succeeds in a shared or explicitly controlled transaction | active | Generated keys can connect dependent writes. Keep original versions and provisional values until the owner confirms its decision. |
| Commit acknowledged | committed | Accept pending changes once if automatic acceptance is selected. With explicit acceptance retain the pending snapshot until AcceptAllChanges. A repeated completion fails with `ErrInvalidState`. |
| Rollback confirmed | rolled_back | Restore original concurrency tokens and temporary keys, retain intended user edits and restore graph links for correction. |
| Command failed | active after confirmed savepoint recovery, otherwise failed if the unit cannot continue | An owned atomic save rolls back the entire unit. In a borrowed transaction, signal the owner if full rollback is required. |
| Commit dispatched and response lost | outcome_unknown | Return `ErrCommitOutcomeUnknown`, freeze provisional entries, quarantine/discard the unsafe connection and require reconciliation or reload. |
| Rollback failed or connection became unusable before commit | failed | Preserve the primary and cleanup errors. Keep the rollback uncertainty explicit. Discard a connection whose state cannot be reset. |

SaveChanges orders principal/dependent inserts, generated-key propagation and dependent/principal deletes as a single atomic unit. In an existing transaction it takes a savepoint before the first write and restores both the savepoint and the in-memory snapshot on failure. Detect an unusable configuration before dispatch and return `ErrUnsupportedFeature`. If the engine dooms a transaction despite the savepoint, the participant reports failed state to the external owner. It must not release or complete that owner's transaction. A savepoint name belongs to its creating scope, is safe to render, and cannot shadow another live name. Rollback-to invalidates later savepoints according to the adapter contract. Where release is unavailable, the adapter may retire the local handle after verification. [EF transactions and savepoints](https://learn.microsoft.com/en-us/ef/core/saving/transactions)

A callback returning an error triggers cleanup. A panic unwinds after owned resources are cleaned up. A bounded independent cleanup context may run after the caller's context ends. Close readers first, collect late iteration and cleanup errors, then release or discard the connection. Preserve causal cancellation and safe secondary errors through wrapping or joining. Interceptors observe begin, command, savepoint, attempts, final disposition and cleanup with redacted identifiers. An observer failure after confirmed commit cannot turn it into a rolled-back business operation.

## Options and provider experiments

The adapter declares supported effective isolation levels, read-only enforcement, savepoint behavior and transaction options for an engine/driver/configuration tuple. Explicit requests receive their declared behavior or a typed preflight rejection. For the provider default, record the effective server setting as part of the result. The table defines experiments awaiting real fixtures.

| Provider | Isolation and failure probes |
| --- | --- |
| PostgreSQL | Coordinate READ COMMITTED, REPEATABLE READ and SERIALIZABLE readers/writers. Capture 40001 and 40P01, aborted state and rollback-to recovery. Observe transaction/session advisory lock lifetime. Exercise pgx connection pinning and each advertised PgBouncer pooling mode separately. [Engine isolation](https://www.postgresql.org/docs/18/transaction-iso.html) |
| SQLite | Use two connections to the same temporary file, in WAL and rollback-journal modes. Probe deferred, immediate and exclusive entry, one writer, extended BUSY/LOCKED codes, busy timeout, savepoint nesting and cancellation. Verify FK/PRAGMA state on the owning connection before and after. [SQLite transactions](https://www.sqlite.org/lang_transaction.html) |
| SQL Server | Record isolation configuration, SNAPSHOT and READ_COMMITTED_SNAPSHOT flags. Probe lock timeout and deadlock victim, SET XACT_ABORT, XACT_STATE(), MARS on/off, rowversion and identity behavior after failure. A doomed transaction cannot roll back to a savepoint. [Transaction state](https://learn.microsoft.com/en-us/sql/t-sql/functions/xact-state-transact-sql?view=sql-server-ver16) |
| MySQL | Record InnoDB, sql_mode, isolation, autocommit and rollback-on-timeout. Coordinate next-key/gap locking when applicable. Distinguish deadlock, statement lock wait timeout and metadata locks. Check savepoints, implicit DDL commits, session lock lifetime and reconnect. [InnoDB error handling](https://dev.mysql.com/doc/refman/8.4/en/innodb-error-handling.html) |

## Execution strategy

Whole-unit replay requires an explicitly replay-safe callback, a classified transient failure, confirmed full rollback, rebuilt inputs and tracker, a fresh transaction, and remaining caller deadline. Bound attempts and backoff with jitter. External effects need an application idempotency or transactional outbox design. Never replay a single statement in an aborted unit. A statement-safe strategy is restricted to independently proved operations before observable side effects. The unknown-commit path takes precedence over generic connection classification. [EF execution strategy](https://github.com/dotnet/efcore/blob/b34c0e9d7e768aa808ded44f19e7f871bea6869f/src/EFCore/Storage/ExecutionStrategy.cs) · [Commit verification options](https://learn.microsoft.com/en-us/ef/core/miscellaneous/connection-resiliency)

An application may write a unique operation marker in the same transaction as its business effects. Reconcile over a fresh connection to an authoritative primary using that marker and the expected payload identity. A matching committed marker proves success. Immediate absence is insufficient while the old transaction might still commit. Only an authoritative negative after the prior unit is known final can establish rollback. If either proof is unavailable, keep outcome_unknown and suppress automatic replay. [Error and retry decision](../adr/0005-errors-and-retries.md)
