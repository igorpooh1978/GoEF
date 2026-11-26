# Savepoints and external transaction ownership

[Issue #2](https://github.com/igorpooh1978/GoEF/issues/2) · [Transaction guide](../../user/TRANSACTIONS.md) · [Ownership scenarios](../../../specs/features/transactions/tx-own-001.feature) · [Savepoint scenarios](../../../specs/features/transactions/tx-savepoint-001.feature)

## Scope and references

An owned `Tx` and a non-owning `External` coordinator can create managed savepoints on their native transaction. An active savepoint rolls back later work and retains its target. GoEF retires younger handles after a successful rewind and releases only the innermost live handle. SQL Server release is local retirement and sends no release command. Generated ASCII names fit its 32-character limit. Dialect selection is explicit. [PostgreSQL](https://www.postgresql.org/docs/18/sql-savepoint.html) · [SQLite](https://www.sqlite.org/lang_savepoint.html) · [SQL Server](https://learn.microsoft.com/en-us/sql/t-sql/language-elements/save-transaction-transact-sql?view=sql-server-ver16) · [MySQL](https://dev.mysql.com/doc/refman/8.4/en/savepoint.html)

Participants share a coordinator over one caller-supplied `database/sql.Tx`. Detaching a participant or the coordinator leaves native completion and lease ownership with the caller. A preflight cancellation leaves points intact. A failure reported by a dispatched savepoint command retires the managed stack and leaves recovery decisions with the transaction owner. The tests exercise explicit DML and savepoints. Tracked `SaveChanges`, distributed transactions and SQL Server MARS retain separate acceptance. [Go 1.27.4 transaction implementation](https://github.com/golang/go/blob/go1.27.4/src/database/sql/sql.go) · [Pinned EF Core ownership implementation](https://github.com/dotnet/efcore/blob/b34c0e9d7e768aa808ded44f19e7f871bea6869f/src/EFCore.Relational/Storage/RelationalTransaction.cs) · [EF Core savepoint notes](https://learn.microsoft.com/en-us/ef/core/saving/transactions)

## Engine fixture

`TestSavepointAndExternalTransaction` binds the two scenarios to the selected profile. `TX-SAVEPOINT-001-S03` verifies nested rollback and release and checks that an independent connection sees only the earlier committed row. `TX-OWN-001-S01` verifies two enlisted participants on the original physical session, detachment without native completion and the owner's ability to view pending rows, roll back and continue using its connection. [Executable fixture](../../../test/conformance/driverlab/savepoint_transaction_test.go) · [Local runner](../../../test/conformance/driverlab/README.md)

The first full local run on `a7f35bcebee3871c72051f1d37eef49b3f770902` exited 1 on SQL Server. A separate SQL Server 2022 diagnostic reproduced the failing assertion on the second rollback to the same point after another successful write. GoEF now recreates the native target under the same operation gate after each successful SQL Server rewind. A failed recreate step retires the stack. Added controlled tests failed against the old code and passed after the fix. [SQL Server rollback semantics](https://learn.microsoft.com/en-us/sql/t-sql/language-elements/rollback-transaction-transact-sql?view=sql-server-ver16) · [Savepoint unit tests](../../../transaction/savepoint_test.go)

## Workstation verification

The corrected code revision `93e74c20ccb309aa4f3869b28178182465bed5d8` was verified on 2026-11-26 with Go 1.27.4. The provider SPI, Go module pins and GitHub workflows match the baseline. Independent review checked the correction, native transaction ownership, safe errors and cancellation, and ran the transaction and error tests and `go vet` on Windows.

- On Windows/amd64, `go mod verify`, `go mod tidy -diff`, `go build -mod=readonly ./...`, `go vet -mod=readonly ./...` and `go test -mod=readonly -count=1 -shuffle=on ./...` exited 0 for the root and driver lab modules. The repository validator checked 71 Gherkin scenarios. Seven bootstrap tests and 64 Python lab tests passed.
- `go test -mod=readonly -shuffle=on -count=1 -coverprofile=<profile> ./goeferr ./transaction` exited 0. The run covered 252 of 257 transaction statements (98.1%) and 22 of 22 error-package statements (100%), for 274 of 279 overall (98.2%).
- Both guide examples passed `go test -mod=readonly -count=1 -v` with temporary SQLite files and the pinned driver. The external guide verified caller-controlled commit and rollback separately and kept the caller's explicit connection available after completion. [Transaction guide](../../user/TRANSACTIONS.md)

The workstation built a test binary per target with `driver_run.build_binary`, called `driver_run.run_provider(..., model_types=True, owned_tx=True, savepoint_tx=True)` on the configured profiles and ran `sqlite_compat.main(["--model-types", "--owned-tx", "--savepoint-tx"])` for both SQLite runtimes and the file suite. This run on the corrected revision exited 0. Both new scenarios passed on all 12 profiles, for 24 distinct results and 26 result rows because the current SQLite profile ran twice. The same run passed 84 distinct driver cases, 12 model type profiles, 12 owned transaction profiles and four SQLite file compatibility cases. To select the same checks with the current Python runner, use `python -B scripts/ci/goef_w002.py --compatibility --model-types --owned-tx --savepoint-tx --accept-sqlserver-eula --accept-legacy-sqlserver-eula`. [Local runner and prerequisites](../../../test/conformance/driverlab/README.md)

| Provider | Verified profiles | Go driver |
| --- | --- | --- |
| PostgreSQL | 16.16, 17.12, 18.7 and 19.0 | pgx v5.13.0 |
| SQLite | 3.51.3 and 3.54.1 | modernc.org/sqlite v1.47.0 and v1.63.0 |
| SQL Server Developer | 14.0.3570.2 at 140, 15.0.4510.2 at 150, 16.0.4325.4 at 140 and 160 | go-mssqldb v1.11.0 |
| MySQL | 8.0.46 and 8.4.13.1 | go-sql-driver/mysql v1.10.0 |

On the same corrected revision, `go test -mod=readonly -race -count=1 -shuffle=on ./...` exited 0 for all five root packages, and `go test -mod=vendor -race -count=1 -shuffle=on ./...` exited 0 for the driver lab. Both ran with Go 1.27.4 in a network-isolated Linux/amd64 container on the workstation. The driver lab used a fresh temporary vendor snapshot from the pinned module. Real engine acceptance ran separately on the same workstation.
