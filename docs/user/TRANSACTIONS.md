# Own a transaction explicitly

Use the `transaction` package with an application-owned `*sql.DB`. The pool uses the Go driver for the chosen engine. The example requires Go 1.27.4 and a disposable fixture table `transaction_markers` with a unique, non-null text column `marker`. Supply a trusted `statement` such as `INSERT INTO transaction_markers (marker) VALUES ($1)` for PostgreSQL, using the placeholder syntax for your driver. The marker value goes in a separate parameter.

```go
package example

import (
	"context"
	"database/sql"
	"errors"

	"github.com/igorpooh1978/GoEF/transaction"
)

// WriteMarker inserts one marker and follows the caller's completion choice.
func WriteMarker(ctx context.Context, db *sql.DB, statement, marker string, keep bool) (err error) {
	tx, err := transaction.Begin(ctx, db)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, tx.Close()) }()

	if _, err := tx.Exec(ctx, statement, marker); err != nil {
		return err
	}
	if keep {
		return tx.Commit()
	}
	return tx.Rollback()
}
```

Call `WriteMarker` on an empty fixture first with `"A", true` and then with `"B", false`. An independent checkout sees only A. PostgreSQL with pgx uses `$1`, SQLite and MySQL use `?`, and SQL Server with go-mssqldb uses `@p1`. For the pinned engine and driver versions used for verification, see the local fixture. [Compatibility profiles](../parity/ENGINE_COMPATIBILITY.md) · [Executable fixture](../../test/conformance/driverlab/owned_transaction_test.go) · [Issue #2](https://github.com/igorpooh1978/GoEF/issues/2)

## Lifetime and errors

`Begin` reserves one physical connection for a native transaction with the driver's default isolation and read/write options. `Exec` runs through that transaction. Use it for transactional writes with no result rows. Use the handle's completion methods to manage its lifetime. Pass statements that participate in the native transaction. In particular, run MySQL schema definition statements separately from data writes that must roll back together. [Go transaction usage](https://go.dev/doc/database/execute-transactions) · [MySQL implicit commits](https://dev.mysql.com/doc/refman/8.4/en/implicit-commit.html)

A successful `Commit` publishes `StateCommitted`. A successful `Rollback` publishes `StateRolledBack`. Both release the checkout to the pool and leave the application-owned pool open. `Close` rolls back an active transaction as deferred cleanup and can be repeated. Commands and explicit completion on a finished handle return an error that matches `goeferr.ErrInvalidState` through `errors.Is`.

The context passed to `Begin` governs acquisition and the transaction lifetime. Cancellation requests rollback while the transaction is active. If commit is already in the driver, its result determines the outcome. A context used only for `Exec` controls that command. After a command error, roll back or close the transaction unless you have a provider-specific recovery contract. Cleanup may need to wait for an in-flight native begin to return. The native commit and rollback methods have no context parameter, and their completion time depends on the driver. Concurrent mutating operations receive `goeferr.ErrContextBusy`. `Close` waits for the ongoing operation before performing cleanup, and `State` can be read independently.

When the native commit result is inconclusive, the handle reports `StateOutcomeUnknown`, discards the connection and returns an error matching `goeferr.ErrCommitOutcomeUnknown`. Reconcile the operation using an independent, authoritative connection before deciding whether it can be repeated. A failed rollback records `StateFailed` and discards the connection. `Close` returns any recorded `goeferr.ErrRollbackFailed`, including after cancellation has already attempted cleanup. [Transaction state and reconciliation](../architecture/TRANSACTION_ENGINE.md)

## Choose isolation and read-only mode

Use `transaction.BeginWithOptions(ctx, db, dialect, sql.TxOptions{Isolation: sql.LevelReadCommitted, ReadOnly: true})` for a read-only PostgreSQL or MySQL transaction. The dialect must match the driver and engine behind `db`. Read one result on the transaction with `tx.Scan(ctx, query, args, &value)`. The method consumes the result before returning and reports `sql.ErrNoRows` for an empty result. Use `Commit`, `Rollback` or deferred `Close` to finish the transaction.

| Engine | Accepted isolation options | Read-only option |
| --- | --- | --- |
| PostgreSQL | Default, Read Committed, Repeatable Read, Serializable | Native read-only transaction |
| MySQL | Default, Read Committed, Repeatable Read, Serializable | Native read-only transaction |
| SQL Server | Default, Read Committed, Repeatable Read, Serializable | Returns `ErrUnsupportedFeature` |
| SQLite | Default, Serializable with `PRAGMA read_uncommitted` off on the leased connection | Returns `ErrUnsupportedFeature` |

`Default` uses the server and driver defaults. GoEF rejects unsupported options with `goeferr.ErrUnsupportedFeature`; the SQLite Serializable check reads connection state before beginning. SQL Server explicitly configured isolation uses one physical connection throughout, then discards that connection at completion to prevent the isolation setting carrying over into a later checkout. Use transaction-preserving statements and keep session settings fixed during the transaction. [Isolation fixture](../../test/conformance/driverlab/transaction_options_test.go) · [Local results](../evidence/W005/ISOLATION_OPTIONS.md)

## Use a native PostgreSQL pool or transaction

Import the [PostgreSQL adapter](../providers/POSTGRESQL.md) to use GoEF with a caller-owned pgx pool. Pass your existing pool to `postgres.BeginTx(ctx, pool, sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})`. It returns the same GoEF transaction handle with `Exec`, `Scan`, savepoints and `SaveChanges`. Read the effective isolation with `tx.Scan(ctx, "SHOW transaction_isolation", nil, &isolation)`. PostgreSQL reports `repeatable read` for that request. Use the caller's pool settings and transaction-preserving SQL.

The native API passes your lifetime context to pool acquisition, `BEGIN` and `COMMIT`. Rollback and connection cleanup use fresh five-second budgets. A canceled operation can close the native connection before explicit rollback is acknowledged. In that case `Close` reports `ErrRollbackFailed` and the handle records `StateFailed`. A server-confirmed rollback of an aborted transaction at commit returns `goeferr.ErrCommitRolledBack` and records `StateRolledBack`. An inconclusive commit follows the reconciliation contract above.

If the caller already opened a `pgx.Tx`, pass it to `postgres.NewExternal(native)` and call `Enlist`. Participants can write with `Exec` and read one row with `Scan`. Use the external coordinator rules below. Close the coordinator before the caller commits or rolls back the native transaction. For a root transaction, native completion is the caller's consistency boundary. For a pgx savepoint-backed transaction, releasing it still leaves the enclosing transaction open.

The fixture covers direct connections to PostgreSQL 18.7 with pgx v5.13.0 in Exec query mode. PgBouncer pooling modes and connection budgets require separate measurements. [Executable fixture](../../providers/postgres/transaction_integration_test.go) · [Evidence and command](../evidence/W005/POSTGRES_NATIVE_TRANSACTIONS.md) · [Issue #37](https://github.com/igorpooh1978/GoEF/issues/37)

## Use savepoints inside a transaction

Call `tx.Savepoint(ctx, dialect)` on an owned transaction or `external.Savepoint(ctx, dialect)` on an external coordinator. Supply the dialect for the actual driver and database. The package generates point names. Rolling back to a live point reverts transactional writes made after it, keeps that point available and invalidates younger handles. Release the innermost live point when it is no longer needed. A release keeps the writes pending in the enclosing transaction. A retired handle, or an attempt to release an older point first, returns an error matching `goeferr.ErrInvalidState`.

Keep the same dialect throughout the transaction or external coordinator after its first savepoint. An unknown or different dialect returns `goeferr.ErrUnsupportedFeature` before dispatch. Use the managed point operations so the stack follows the database transaction. Cancellation detected before dispatch leaves the stack valid. If a native point command fails, the error matches `goeferr.ErrSavepointFailed` and the managed stack is invalidated. The owner should roll back the enclosing transaction unless engine evidence establishes safe recovery.

| Dialect | Create and roll back | Release |
| --- | --- | --- |
| `transaction.DialectPostgreSQL` | `SAVEPOINT`, `ROLLBACK TO SAVEPOINT` | `RELEASE SAVEPOINT` |
| `transaction.DialectSQLite` | `SAVEPOINT`, `ROLLBACK TO SAVEPOINT` | `RELEASE SAVEPOINT` |
| `transaction.DialectSQLServer` | `SAVE TRANSACTION`, `ROLLBACK TRANSACTION` to the point | Retire the GoEF handle. The server point expires with the enclosing transaction. |
| `transaction.DialectMySQL` | `SAVEPOINT`, `ROLLBACK TO SAVEPOINT` for transactional tables such as InnoDB | `RELEASE SAVEPOINT` |

On SQL Server, GoEF recreates the target after a successful native rollback so the handle can be used again. If the restore step fails or is canceled after the rewind, GoEF invalidates the managed stack. The fixture uses a local, committable transaction with Multiple Active Result Sets (MARS) disabled. If SQL Server marks the transaction uncommittable, the owner must roll it back in full. Keep statements that implicitly commit, including MySQL schema changes, outside the managed scope. [PostgreSQL](https://www.postgresql.org/docs/18/sql-rollback-to.html) · [SQLite](https://www.sqlite.org/lang_savepoint.html) · [SQL Server](https://learn.microsoft.com/en-us/sql/t-sql/language-elements/save-transaction-transact-sql?view=sql-server-ver16) · [SQL Server MARS policy in the pinned reference](https://github.com/dotnet/efcore/blob/b34c0e9d7e768aa808ded44f19e7f871bea6869f/src/EFCore.SqlServer/Storage/Internal/SqlServerTransaction.cs) · [MySQL](https://dev.mysql.com/doc/refman/8.4/en/savepoint.html)

## Enlist in an external transaction

Use one `transaction.NewExternal(native)` coordinator per caller-owned `*sql.Tx`. Each `Enlist` call creates a participant on that transaction. Participant `Exec`, `Scan` and coordinator savepoint operations share one operation slot. An overlapping operation returns `goeferr.ErrContextBusy`. A participant's `Close` waits for any active managed operation and detaches just that participant. It can be repeated. The caller retains control of the native transaction and connection.

The example uses the same `transaction_markers` fixture and driver parameter syntax as `WriteMarker`. It requires Go 1.27.4. The owner inserts `A` through its native transaction, creates an external coordinator and calls `StageMarker` with `"B", false`. On success the owner closes the coordinator and commits the native transaction. A separate connection sees only `A`. On error the helper returns control to the native owner for cleanup.

```go
package example

import (
	"context"
	"errors"

	"github.com/igorpooh1978/GoEF/transaction"
)

// StageMarker writes a marker inside the caller's enclosing transaction.
func StageMarker(ctx context.Context, external *transaction.External, dialect transaction.Dialect, statement, marker string, keep bool) (err error) {
	participant, err := external.Enlist()
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, participant.Close()) }()

	point, err := external.Savepoint(ctx, dialect)
	if err != nil {
		return err
	}
	if _, err := participant.Exec(ctx, statement, marker); err != nil {
		return err
	}
	if !keep {
		if err := point.Rollback(ctx); err != nil {
			return err
		}
	}
	return point.Release(ctx)
}
```

Serialize the owner's direct `*sql.Tx` calls with GoEF operations. The owner calls `external.Close()` before its native `Commit` or `Rollback`. Closing the coordinator waits for its managed operation and invalidates its participants and savepoint handles without native I/O. Calls through a detached scope return `goeferr.ErrInvalidState`. If the owner completes the native transaction directly, a subsequent SQL operation can detect that the native transaction is finished. Cancellation of an enlisted command affects that operation. The owner controls the lifetime of the native transaction.

## Save a batch of changes

Call `SaveChanges` on an active `Tx`, `External` or `Participant`. The method creates a private savepoint and runs the callback in order on the same physical transaction. `SaveWriter.Exec` executes statements without result rows. `SaveWriter.Scan` reads one result row into local variables, which can supply keys or versions to later statements. Use trusted, parameterized DML. Run the callback synchronously and use its writer for all SQL in the batch. Keep transaction-control statements, operations that implicitly commit, and external side effects outside the callback. The application supplies dependency order.

The following example requires Go 1.27.4 and a disposable SQLite 3.51.3 or 3.54.1 file opened with the corresponding `modernc.org/sqlite` v1.47.0 or v1.63.0 driver. Create `save_markers` with `id INTEGER PRIMARY KEY` and `marker TEXT NOT NULL UNIQUE` on the file before starting. Both writes use SQLite's `?` parameter syntax. [Pinned versions](../parity/ENGINE_COMPATIBILITY.md)

```go
package example

import (
	"context"
	"database/sql"
	"errors"

	"github.com/igorpooh1978/GoEF/transaction"
)

// SaveMarkerPair writes two markers and publishes the first key after commit.
func SaveMarkerPair(ctx context.Context, db *sql.DB, first, second string, firstID *int64) (err error) {
	tx, err := transaction.Begin(ctx, db)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, tx.Close()) }()

	options := transaction.SaveOptions{Dialect: transaction.DialectSQLite, Savepoints: true}
	saved, err := tx.SaveChanges(ctx, options, saveMarkerPair(first, second, firstID))
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return saved.Accept()
}

func saveMarkerPair(first, second string, firstID *int64) func(*transaction.SaveWriter) error {
	return func(w *transaction.SaveWriter) error {
		var id int64
		if err := w.Scan(`INSERT INTO save_markers (marker) VALUES (?) RETURNING id`, []any{first}, &id); err != nil {
			return err
		}
		if err := transaction.StageValue(w, firstID, id); err != nil {
			return err
		}
		_, err := w.Exec(`INSERT INTO save_markers (marker) VALUES (?)`, second)
		return err
	}
}
```

On an empty file, `SaveMarkerPair(ctx, db, "A", "B", &firstID)` commits A and B and puts A's generated key in `firstID`. Calling it next with `"C", "C"` fails on the second insert, leaves the first key unchanged, and stores no C. An independent connection can verify both outcomes.

Use the same callback with `participant.SaveChanges(ctx, options, saveMarkerPair(first, second, &firstID))` in a caller-owned transaction. The caller closes the coordinator and commits its native transaction before accepting a successful save. A successful batch returns a `SavedChanges` receipt. `StageValue` records assignments for that receipt, leaving application fields unchanged until `Accept`. Stage copies of mutable values if the original could change before acceptance. Call `Accept` once, after a confirmed enclosing commit that includes the batch. A managed rewind to an earlier savepoint invalidates receipts for the batches it removes. An owned receipt also checks the enclosing `Tx` state. For external receipts the caller confirms which batches the native commit included and discards receipts after rollback or an unknown outcome.

A callback or writer error rolls back the batch when the engine confirms recovery. Earlier work stays pending, and `SaveChanges` returns no receipt. A canceled operation can attempt recovery with a separate bounded context. Canceling the owned transaction's lifetime still ends that transaction. If the error matches `goeferr.ErrSavepointFailed`, a savepoint operation failed. When recovery cannot be confirmed, GoEF attempts a full rollback of its owned transaction. For an external transaction it detaches the coordinator and also reports `goeferr.ErrRollbackFailed`. The external owner must roll back the native transaction unless it has already ended. GoEF performs no automatic replay.

Set `SaveOptions.Savepoints` only for a configuration you have verified can use savepoints for this batch. A false value or an unsupported dialect returns `goeferr.ErrUnsupportedFeature` before calling the callback or dispatching SQL. On SQL Server use a local, committable transaction with Multiple Active Result Sets (MARS) disabled. GoEF checks that the session has `XACT_ABORT OFF` with a read-only query before creating the savepoint. On SQLite use a conflict policy that preserves the enclosing transaction, such as the default `ABORT`. On MySQL use transactional tables such as InnoDB. Some engine failures, including an InnoDB deadlock, can still end the entire transaction. [SQL Server savepoints](https://learn.microsoft.com/en-us/sql/t-sql/language-elements/save-transaction-transact-sql?view=sql-server-ver16) · [SQL Server XACT_ABORT](https://learn.microsoft.com/en-us/sql/t-sql/statements/set-xact-abort-transact-sql?view=sql-server-ver16) · [SQLite conflict resolution](https://www.sqlite.org/lang_conflict.html) · [InnoDB error recovery](https://dev.mysql.com/doc/refman/8.4/en/innodb-error-handling.html)

The callback API supplies transactional batching and explicit acceptance for generated values. Model-driven change detection, automatic graph ordering and optimistic concurrency resolution belong to the change tracker. [Change tracker](../architecture/CHANGE_TRACKER.md) · [Ownership scenarios](../../specs/features/transactions/tx-own-001.feature) · [Savepoint scenarios](../../specs/features/transactions/tx-savepoint-001.feature) · [Conformance protocol](../../test/conformance/transactions/README.md)

## Verify an unknown owned commit

If **Commit** returns **goeferr.ErrCommitOutcomeUnknown**, GoEF has discarded the affected connection. Keep the transaction handle and its unaccepted receipts. With a fresh, bounded context call **tx.ReconcileCommit(ctx, verify)**. The verifier opens its own fresh connection to the authoritative primary and checks the exact unit. For example, write a unique operation marker and payload identity in the same transaction as the business changes, then check for the matching marker and payload.

| Verifier result | Transaction state and receipts |
| --- | --- |
| **transaction.CommitConfirmed**, nil | **StateCommitted**. Call **Accept** on each valid receipt in save order to publish staged values once. |
| **transaction.RollbackConfirmed**, nil | **StateRolledBack**. The receipts cannot be accepted, and the original fields and pending edits remain available. |
| **transaction.CommitUnverified**, a read failure or a canceled context | **StateOutcomeUnknown**. Keep the receipts unapplied. A later call may verify again. |

An absent marker confirms rollback only if independent evidence establishes that the prior transaction finished without committing. An unavailable primary, a lagging replica, or a mismatched payload leaves the outcome unresolved. Reconciliation never retries the save callback or completes the old native transaction. The application owns the verification connection and any decision to attempt the unit again after confirmed rollback. [Commit outcome scenarios](../../specs/features/transactions/tx-outcome-001.feature) · [EF Core commit verification](https://learn.microsoft.com/en-us/ef/core/miscellaneous/connection-resiliency#transaction-commit-failure-and-the-idempotency-issue) · [Executable example](../../test/conformance/driverlab/commit_outcome_test.go) · [Verification results](../evidence/W002/COMMIT_RECONCILIATION.md)
