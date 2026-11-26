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

External enlistment, provider-specific options, returning rows, savepoints, automatic retries and tracker integration are described in the transaction design.

[Ownership scenarios](../../specs/features/transactions/tx-own-001.feature) · [Conformance protocol](../../test/conformance/transactions/README.md)
