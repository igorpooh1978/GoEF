# W002 cancellation through the provider execution interface

[W002 #13](https://github.com/igorpooh1978/GoEF/issues/13) · [Executable driver feature](../../../test/conformance/driverlab/features/driver.feature) · [Provider execution contract](../../architecture/PROVIDER_SPI.md)

## Cancellation after the first row

`LAB-DRIVER-001-S04` exercises the after-first-row behavior in [ERR-CANCEL-001-S01](../../../specs/features/errors/err-cancel-001.feature). The lab adapts the four pinned drivers to the existing `provider.Backend`, `provider.Lease` and `provider.Rows` interfaces. The test adapter accepts the integer result shape used by the fixture. Both driver lab module files reference the local GoEF module at the current checkout. The pinned external dependencies and their checksums are preserved. The module files use LF on checkout so the same `go mod tidy -diff` gate is reproducible on Windows.

The fixture acquires the only slot of a private pool and starts an ordered, two-row read. It checks the first value before canceling the query context. The next SPI read yields no value and preserves `context.Canceled` as its cause. It closes the result twice and discards the lease using a fresh cleanup context. The pool must report zero occupied slots and zero open physical connections after discard. A fresh lease then reads both values to clean EOF and releases the slot. The full successful read provides a control for the second value and clean exhaustion.

The adapter checks the query context before exposing buffered data and also checks it after advancing the driver. It uses the documented `database/sql.Conn.Raw` bad-connection disposition to retire an uncertain connection. The suite proves pool recovery after the application cancels an already open stream. In-flight network reads, production adapters and ambiguous commit outcomes retain their own acceptance gates. [Go 1.27.4 database/sql source](https://github.com/golang/go/blob/go1.27.4/src/database/sql/sql.go) · [SQL Server driver token handling](https://github.com/microsoft/go-mssqldb/blob/v1.11.0/token.go#L1266-L1359)

## Local run on 2026-11-26

The scenario lane first observed a failure from the missing S04 binding. A separate negative control used SQLite 3.54.1 and a temporary adapter that returned `io.EOF` on cancellation. S04 failed with exit 1 and emitted no passing scenario record. Python and PowerShell checks also reject an omitted S04, invalid context cause, nonzero pool use or an unexpected native code.

The complete local command exited 0 on `8ca2d289acf004a67a09e624c027adbe04e318ab`. The Windows/amd64 workstation used Go 1.27.4 and its local Docker Desktop Linux/amd64 daemon for server fixtures.

```powershell
pwsh -NoProfile -File scripts/ci/goef-w002.ps1 -Compatibility -AcceptSqlServerEula -AcceptLegacySqlServerEula
```

| Engine | Verified profiles for S04 | Result |
| --- | --- | --- |
| PostgreSQL | 16.16, 17.12, 18.7, 19.0 | PASS |
| SQLite | 3.51.3 and 3.54.1 | PASS |
| SQL Server | 2017/140, 2019/150, 2022/160, 2022/140 | PASS |
| MySQL | 8.0.46 and 8.4.13.1 | PASS |

All 12 profiles reported `context.Canceled` and zero occupied connections. The full run passed 48 distinct driver cases and the four SQLite file cases. The 52 driver rows include the four repeated current SQLite cases. The repository validator, seven bootstrap tests, 38 Python lab tests and Go build, vet, test and module checks passed. A separate local Go test and vet passed with the SQLite minimum module file and build tag. The primary driver's unit tests also passed with `go test -mod=vendor -race -count=1 -shuffle=on ./...` in an isolated local Linux/amd64 container. No lab-owned containers remained after the matrix.

The exact driver module tree is `7f90e1bd88e73a7f0352c08024de2a80415006de`, lab tree `6e325cd4580e4a86686a87f8c474a66f50cf92a9`, PowerShell blob `8f77cdf96f0ee6b68ca75d0e16e6321ef3111479` and provider SPI tree `c335b07d4a32061204c22af28efdafb8d4ffb49d`. The evidence commit preserves these executable artifacts.

## Lock-wait follow-up

The [lock timeout run](LOCK_TIMEOUT.md) implements the driver-lab follow-up. The table below records the controls identified at the cancellation checkpoint.

The [fault schedule](../../development/CONFORMANCE_LAB.md) requires observed lock entry and separate runs for caller deadline and engine timeout. Provider research identified these controls for the next executable slice.

| Provider | Lock evidence and expected native timeout |
| --- | --- |
| PostgreSQL | Keep two physical leases. The first transaction updates a known row and holds the lock. Before allowing it to complete, observe its backend PID in `pg_blocking_pids` for the contender. Choose shorter `lock_timeout` for the engine case and a shorter caller deadline in the other run. Expect `55P03` with the caller context live in the engine case. [Session settings](https://www.postgresql.org/docs/18/runtime-config-client.html#GUC-LOCK-TIMEOUT) · [Backend information](https://www.postgresql.org/docs/18/functions-info.html) |
| SQL Server | Hold a row update in a separate SQL Server session. A diagnostic session can observe the exact blocked session and blocker in `sys.dm_exec_requests`. Set `LOCK_TIMEOUT` explicitly on the contending session. Expect typed error `1222` while the caller context is still live. [SET LOCK_TIMEOUT](https://learn.microsoft.com/en-us/sql/t-sql/statements/set-lock-timeout-transact-sql?view=sql-server-ver16) · [Active requests](https://learn.microsoft.com/en-us/sql/relational-databases/system-dynamic-management-views/sys-dm-exec-requests-transact-sql?view=sql-server-ver16) |
| MySQL | Hold a known InnoDB primary-key update. Give the contender an independent sentinel update in the same transaction before it attempts the conflicting update. With `innodb_rollback_on_timeout=OFF`, expect `1205/HY000` under a live context and observe that the sentinel remains visible to the same owning transaction before rollback. [InnoDB error handling](https://dev.mysql.com/doc/refman/8.4/en/innodb-error-handling.html) |
| SQLite | Use two leases on the same WAL file and retain the lock-holder transaction until the contender returns. A local scratch characterization with 3.51.3 and 3.54.1 saw native `SQLITE_BUSY` (`5`) at a 600 ms configured wait with a live caller. With a 120 ms caller deadline the newer driver returned near the caller deadline and the minimum driver's call returned after the busy wait. Subsequent writes succeeded after releasing the lock. The exact callback observation and deadline timing still need durable scenario tests. [Busy timeout](https://www.sqlite.org/c3ref/busy_timeout.html) · [Pinned current driver's busy handler](https://github.com/modernc-org/sqlite/blob/v1.63.0/busy.go) |

For the first differential fixture, [LINQ-CONFORMANCE-001-S02](../../../specs/features/linq/conformance.feature) can compare an EF Core PostgreSQL query and parameterized PostgreSQL control SQL. Use duplicate join keys, a case-sensitive key pair, `NULL` keys and two tenants. Assert ordered rows, multiplicity and parameter binding. The full comparison with the GoEF query path follows the runtime query implementation.
