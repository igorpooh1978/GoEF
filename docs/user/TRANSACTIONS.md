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

## Use savepoints inside a transaction

Call `tx.Savepoint(ctx, dialect)` on an owned transaction or `external.Savepoint(ctx, dialect)` on an external coordinator. Supply the dialect for the actual driver and database. The package generates point names. Rolling back to a live point reverts transactional writes made after it, keeps that point available and invalidates younger handles. Release the innermost live point when it is no longer needed. A release keeps the writes pending in the enclosing transaction. A retired handle, or an attempt to release an older point first, returns an error matching `goeferr.ErrInvalidState`.

Keep the same dialect throughout the transaction or external coordinator after its first savepoint. An unknown or different dialect returns `goeferr.ErrUnsupportedFeature` before dispatch. Use the managed point operations so the stack follows the database transaction. Cancellation detected before dispatch leaves the stack valid. If a native point command fails, the error matches `goeferr.ErrSavepointFailed` and the managed stack is invalidated. The owner should roll back the enclosing transaction unless engine evidence establishes safe recovery.

| Dialect | Create and roll back | Release |
| --- | --- | --- |
| `transaction.DialectPostgreSQL` | `SAVEPOINT`, `ROLLBACK TO SAVEPOINT` | `RELEASE SAVEPOINT` |
| `transaction.DialectSQLite` | `SAVEPOINT`, `ROLLBACK TO SAVEPOINT` | `RELEASE SAVEPOINT` |
| `transaction.DialectSQLServer` | `SAVE TRANSACTION`, `ROLLBACK TRANSACTION` to the point | Retire the GoEF handle. The server point expires with the enclosing transaction. |
| `transaction.DialectMySQL` | `SAVEPOINT`, `ROLLBACK TO SAVEPOINT` for transactional tables such as InnoDB | `RELEASE SAVEPOINT` |

The SQL Server fixture uses a local, committable transaction with Multiple Active Result Sets (MARS) disabled. If SQL Server marks the transaction uncommittable, the owner must roll it back in full. Keep statements that implicitly commit, including MySQL schema changes, outside the managed scope. [PostgreSQL](https://www.postgresql.org/docs/18/sql-rollback-to.html) · [SQLite](https://www.sqlite.org/lang_savepoint.html) · [SQL Server](https://learn.microsoft.com/en-us/sql/t-sql/language-elements/save-transaction-transact-sql?view=sql-server-ver16) · [SQL Server MARS policy in the pinned reference](https://github.com/dotnet/efcore/blob/b34c0e9d7e768aa808ded44f19e7f871bea6869f/src/EFCore.SqlServer/Storage/Internal/SqlServerTransaction.cs) · [MySQL](https://dev.mysql.com/doc/refman/8.4/en/savepoint.html)

## Enlist in an external transaction

Use one `transaction.NewExternal(native)` coordinator per caller-owned `*sql.Tx`. Each `Enlist` call creates a participant on that transaction. Participant `Exec` and coordinator savepoint operations share one operation slot. An overlapping operation returns `goeferr.ErrContextBusy`. A participant's `Close` waits for any active managed operation and detaches just that participant. It can be repeated. The caller retains control of the native transaction and connection.

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

The public savepoint methods are manual controls for transaction-preserving statements. Tracked `SaveChanges`, provider-specific transaction options, returning rows and automatic retries have their own design and conformance cases. [Ownership scenarios](../../specs/features/transactions/tx-own-001.feature) · [Savepoint scenarios](../../specs/features/transactions/tx-savepoint-001.feature) · [Conformance protocol](../../test/conformance/transactions/README.md)
