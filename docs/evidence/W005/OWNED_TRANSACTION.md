# Owned transaction and pool lifecycle

[Issue #2](https://github.com/igorpooh1978/GoEF/issues/2) · [Transaction guide](../../user/TRANSACTIONS.md) · [Ownership scenario](../../../specs/features/transactions/tx-own-001.feature)

## Contract and source

`transaction.Begin` reserves a `database/sql.Conn` from the application's pool and opens a default-options native transaction on that connection. Go's `sql.Tx` executes through its bound connection. GoEF owns release of the explicit lease after the native transaction ends. A known completion returns the connection. An uncertain native result causes the handle to discard the connection. [Go transaction guide](https://go.dev/doc/database/execute-transactions) · [Go 1.27.4 `database/sql`](https://github.com/golang/go/blob/go1.27.4/src/database/sql/sql.go)

The reference EF Core implementation also separates transaction completion from ownership and connection cleanup. GoEF's first entry point uses an application-supplied SQL pool and exposes `Begin`, parameterized `Exec`, `Commit`, `Rollback`, `State` and cleanup through `Close`. The remaining feature cases cover caller-owned enlistment and atomic tracked graph saves. [Pinned EF Core transaction implementation](https://github.com/dotnet/efcore/blob/b34c0e9d7e768aa808ded44f19e7f871bea6869f/src/EFCore.Relational/Storage/RelationalTransaction.cs)

## Acceptance fixture

`TestOwnedTransaction` implements `TX-OWN-001-S02` for the four database families. It tests parameterized DML in a pool with one open slot, verifies the physical session for both commands in each transaction and observes a competing acquisition through the pool's wait counter. The queued acquisition must receive the same physical connection after commit and rollback. A separate physical observer sees only the committed row. Both completed handles must reject further commands and completion attempts, and the pool returns to its initial in-use count. [Go fixture](../../../test/conformance/driverlab/owned_transaction_test.go) · [Local test commands](../../../test/conformance/driverlab/README.md)

## Workstation verification

The checks use integrated code revision `50c55c98bb74677c021dc26cfcae7233bb022b1c` and Go 1.27.4 on 2026-11-26. Independent review compared the runtime and engine fixture with their checked worktree trees and audited cancellation, connection disposition, bounded fixture waits and safe evidence. The frozen provider execution SPI and Go module pins match the baseline.

- On Windows/amd64, module verification, `go mod tidy -diff`, `go build -mod=readonly ./...`, `go vet -mod=readonly ./...` and `go test -mod=readonly -count=1 -shuffle=on ./...` exited 0 for the root and driver lab modules. The repository validator, seven bootstrap tests and 63 Python lab tests also passed.
- `go test -mod=readonly -shuffle=on -count=1 -coverprofile=<profile> ./goeferr ./transaction` exited 0. The run covered 110 of 114 transaction statements and 20 of 20 error-package statements, for 130 of 134 overall.
- The baseline `39ef95b448888ed1bb7d4edbd22b8f7d7afc0a29` failed to compile the added fixture before the API existed. A controlled mutation that reused a connection after an unknown commit made `TestCommitFailureReportsUnknownAndDiscards` exit 1. Restoring discard returned exit 0. Additional controlled tests cover cancellation, conflicting use, a native rollback failure and a broken connection. [Unit tests](../../../transaction/transaction_test.go) · [Failure tests](../../../transaction/failure_test.go)
- The complete guide example was extracted and run with `go test -mod=readonly -count=1 -v` against a temporary SQLite file through the pinned driver. It returned the worker connection after both completions, and an independent connection saw only `A`.

The workstation built one test binary per host target with `driver_run.build_binary`, called `driver_run.run_provider(..., model_types=True, owned_tx=True)` on all configured profiles and ran `sqlite_compat.main(["--model-types", "--owned-tx"])` for both linked SQLite runtimes and the file suite. The local invocation exited 0. It produced 12 distinct transaction profile results, 12 distinct model type profile results, 84 distinct driver scenario results and four bidirectional SQLite file results. The current SQLite profile ran in both paths, giving 13 transaction rows, 13 type rows and 91 driver rows. Every report passed on the first run. The selectable PowerShell equivalent adds `-OwnedTx -ModelTypes -Compatibility` and the existing SQL Server consent flags. [Local lab command](../../../test/conformance/driverlab/README.md)

| Provider | Verified `TX-OWN-001-S02` profiles | Go driver |
| --- | --- | --- |
| PostgreSQL | 16.16, 17.12, 18.7 and 19.0 | pgx v5.13.0 |
| SQLite | 3.51.3 and 3.54.1 | modernc.org/sqlite v1.47.0 and v1.63.0 |
| SQL Server Developer | 14.0.3570.2 at 140, 15.0.4510.2 at 150, 16.0.4325.4 at 140 and 160 | go-mssqldb v1.11.0 |
| MySQL | 8.0.46 and 8.4.13.1 | go-sql-driver/mysql v1.10.0 |

The same revision passed `go test -mod=readonly -race -count=1 -shuffle=on ./...` for the five root packages and `go test -mod=vendor -race -count=1 -shuffle=on ./...` for the driver lab. Both ran on Go 1.27.4 in a network-isolated Linux/amd64 container on the workstation, with a fresh temporary vendor snapshot from the pinned driver module. The engine tests above ran separately against the actual drivers and databases.
