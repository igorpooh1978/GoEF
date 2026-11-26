# Owned transaction and pool lifecycle

[Issue #2](https://github.com/igorpooh1978/GoEF/issues/2) · [Transaction guide](../../user/TRANSACTIONS.md) · [Ownership scenario](../../../specs/features/transactions/tx-own-001.feature)

## Contract and source

`transaction.Begin` reserves a `database/sql.Conn` from the application's pool and opens a default-options native transaction on that connection. Go's `sql.Tx` executes through its bound connection. GoEF owns release of the explicit lease after the native transaction ends. A known completion returns the connection. An uncertain native result causes the handle to discard the connection. [Go transaction guide](https://go.dev/doc/database/execute-transactions) · [Go 1.27.4 `database/sql`](https://github.com/golang/go/blob/go1.27.4/src/database/sql/sql.go)

The reference EF Core implementation also separates transaction completion from ownership and connection cleanup. GoEF's first entry point uses an application-supplied SQL pool and exposes `Begin`, parameterized `Exec`, `Commit`, `Rollback`, `State` and cleanup through `Close`. The remaining feature cases cover caller-owned enlistment and atomic tracked graph saves. [Pinned EF Core transaction implementation](https://github.com/dotnet/efcore/blob/b34c0e9d7e768aa808ded44f19e7f871bea6869f/src/EFCore.Relational/Storage/RelationalTransaction.cs)

## Acceptance fixture

`TestOwnedTransaction` implements `TX-OWN-001-S02` for the four database families. It tests parameterized DML in a pool with one open slot, verifies the physical session for both commands in each transaction and observes a competing acquisition through the pool's wait counter. The queued acquisition must receive the same physical connection after commit and rollback. A separate physical observer sees only the committed row. Both completed handles must reject further commands and completion attempts, and the pool returns to its initial in-use count. [Go fixture](../../../test/conformance/driverlab/owned_transaction_test.go) · [Local test commands](../../../test/conformance/driverlab/README.md)
