# Explicit transaction isolation and read-only options

[Issue #2](https://github.com/igorpooh1978/GoEF/issues/2) · [Isolation scenario](../../../specs/features/transactions/tx-isolation-001.feature) · [Executable fixture](../../../test/conformance/driverlab/transaction_options_test.go) · [Transaction guide](../../user/TRANSACTIONS.md#choose-isolation-and-read-only-mode)

`BeginWithOptions` bounds the supported options using the declared dialect. `Scan` reads one row on the bound native transaction. SQLite checks `read_uncommitted` on the leased connection before beginning Serializable mode. SQL Server explicitly configured isolation discards the physical session after completion because the TDS reset can preserve it. PostgreSQL and MySQL request native read-only transactions and verify permanent-table writes are rejected. [PostgreSQL isolation](https://www.postgresql.org/docs/19/transaction-iso.html) · [SQLite isolation](https://www.sqlite.org/isolation.html) · [SQL Server isolation](https://learn.microsoft.com/en-us/sql/t-sql/statements/set-transaction-isolation-level-transact-sql?view=sql-server-ver16) · [SQL Server TDS reset](https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-tds/ce398f9a-7d47-4ede-8f36-9dd6fc21ca43) · [MySQL transaction characteristics](https://dev.mysql.com/doc/refman/8.4/en/set-transaction.html)

The scenario observes a writer's committed value from Read Committed, stable snapshots from MVCC modes and blocked writes from lock-based modes. It also compares the pool's default after explicit options and checks typed rejection before writes. The unsupported nesting scenario has a separate acceptance gate.

## Workstation checks

The integrated source revision is `c9a438928cc25b0c8ee8fde7bf78e27d037d6e65`. Verification ran on 2026-11-26 using Go 1.27.4 on the workstation. The new fixture failed to compile at baseline `85799b679601610a1fb72971b74c38a40d9f7224` because `BeginWithOptions` and `Scan` were absent. The Python runner acceptance test failed before the new selector and verifier existed. Both passed after implementation. [Unit tests](../../../transaction/options_test.go) · [Runner acceptance](../../../test/conformance/lab/test_driver_run.py)

On Windows/amd64, `go mod verify`, `go mod tidy -diff`, `go build -mod=readonly ./...`, `go vet -mod=readonly ./...` and `go test -mod=readonly -count=1 -shuffle=on ./...` exited 0 in the root and driver lab modules. The repository validator found 74 Gherkin scenarios. All seven bootstrap and 74 Python lab tests passed. The integrated Go coverage run reported 457/478 transaction statements (95.6%), 23/23 error statements (100%), 345/349 model statements (98.9%) and 41/42 LINQ Objects statements (97.6%). The Windows/386 LINQ Objects run reached 100% and exercised actual index and count overflow.

The exact revision passed `go test -mod=readonly -race -count=1 -shuffle=on ./...` for the five root packages and `go test -mod=vendor -race -count=1 -shuffle=on ./...` for the driver lab with Go 1.27.4 in a network-isolated Linux/amd64 container on the workstation.

## Real-engine matrix

The local helper run built host and Linux test binaries once, then called `driver_run.run_provider(..., model_types=True, owned_tx=True, savepoint_tx=True, save_changes=True, tx_options=True)` and the SQLite compatibility launcher with the same selectors. It exited 0. Scenario `TX-ISOLATION-001-S01` passed on 12 profiles, with 13 rows because SQLite 3.54.1 ran twice. The same run passed 84 distinct driver cases, four SQLite file cases, 12 model type profiles, 12 owned transaction profiles, 24 explicit savepoint/enlistment cases and 36 `SaveChanges` cases. PostgreSQL and MySQL exercised read-only enforcement with explicit Read Committed on a permanent table. Read/write fixtures separately exercised the advertised isolation levels.

| Provider | Verified profiles | Go driver |
| --- | --- | --- |
| PostgreSQL | 16.16, 17.12, 18.7 and 19.0 | pgx v5.13.0 |
| SQLite | 3.51.3 and 3.54.1 | modernc.org/sqlite v1.47.0 and v1.63.0 |
| SQL Server Developer | 14.0.3570.2 at 140, 15.0.4510.2 at 150, 16.0.4325.4 at 140 and 160 | go-mssqldb v1.11.0 |
| MySQL | 8.0.46 and 8.4.13.1 | go-sql-driver/mysql v1.10.0 |

The local PowerShell entry point for the same checks is `pwsh -NoProfile -File scripts/ci/goef-w002.ps1 -Compatibility -ModelTypes -OwnedTx -SavepointTx -SaveChanges -TxOptions -AcceptSqlServerEula -AcceptLegacySqlServerEula`. The independent reviewer confirmed all profile results, checked connection disposal and the SQLite preflight on the exact physical lease, and audited the captured MySQL and SQLite compatibility results. [MySQL blocked reads](../W002/MYSQL_IN_FLIGHT_READ.md) · [SQLite WAL recovery](../W002/SQLITE_WAL_RECOVERY.md)
