# Savepoints and external transaction ownership

[Issue #2](https://github.com/igorpooh1978/GoEF/issues/2) · [Transaction guide](../../user/TRANSACTIONS.md) · [Ownership scenarios](../../../specs/features/transactions/tx-own-001.feature) · [Savepoint scenarios](../../../specs/features/transactions/tx-savepoint-001.feature)

## Scope and references

An owned `Tx` and a non-owning `External` coordinator can create managed savepoints on their native transaction. An active savepoint rolls back later work and retains its target. GoEF retires younger handles after a successful rewind and releases only the innermost live handle. SQL Server release is local retirement and sends no release command. Generated ASCII names fit its 32-character limit. Dialect selection is explicit. [PostgreSQL](https://www.postgresql.org/docs/18/sql-savepoint.html) · [SQLite](https://www.sqlite.org/lang_savepoint.html) · [SQL Server](https://learn.microsoft.com/en-us/sql/t-sql/language-elements/save-transaction-transact-sql?view=sql-server-ver16) · [MySQL](https://dev.mysql.com/doc/refman/8.4/en/savepoint.html)

Participants share a coordinator over one caller-supplied `database/sql.Tx`. Detaching a participant or the coordinator leaves native completion and lease ownership with the caller. A preflight cancellation leaves points intact. A failure reported by a dispatched savepoint command retires the managed stack and leaves recovery decisions with the transaction owner. The tests exercise explicit DML and savepoints. Tracked `SaveChanges`, distributed transactions and SQL Server MARS retain separate acceptance. [Go 1.27.4 transaction implementation](https://github.com/golang/go/blob/go1.27.4/src/database/sql/sql.go) · [Pinned EF Core ownership implementation](https://github.com/dotnet/efcore/blob/b34c0e9d7e768aa808ded44f19e7f871bea6869f/src/EFCore.Relational/Storage/RelationalTransaction.cs) · [EF Core savepoint notes](https://learn.microsoft.com/en-us/ef/core/saving/transactions)

## Engine fixture

`TestSavepointAndExternalTransaction` binds the two scenarios to the selected profile. `TX-SAVEPOINT-001-S03` verifies nested rollback and release and checks that an independent connection sees only the earlier committed row. `TX-OWN-001-S01` verifies two enlisted participants on the original physical session, detachment without native completion and the owner's ability to view pending rows, roll back and continue using its connection. [Executable fixture](../../../test/conformance/driverlab/savepoint_transaction_test.go) · [Local runner](../../../test/conformance/driverlab/README.md)
