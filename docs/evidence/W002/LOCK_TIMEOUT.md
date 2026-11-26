# W002 observed lock timeouts with Go drivers

[W002 #13](https://github.com/igorpooh1978/GoEF/issues/13) · [Executable driver feature](../../../test/conformance/driverlab/features/driver.feature) · [Fault schedule](../../development/CONFORMANCE_LAB.md)

`LAB-DRIVER-001-S05` exercises two separate waits for a blocked write. One uses a caller deadline shorter than the database lock timeout. The other uses a database lock timeout shorter than the caller's budget. The test accepts an observation only while the request context is live. The lock holder stays active until the waiting command returns.

| Provider | Positive observation and recovery | Native timeout |
| --- | --- | --- |
| PostgreSQL | `pg_blocking_pids` must link the two transaction backends. Roll back both transactions and take another lease after the deadline. [PostgreSQL 18 session settings](https://www.postgresql.org/docs/18/runtime-config-client.html#GUC-LOCK-TIMEOUT) · [Backend information](https://www.postgresql.org/docs/18/functions-info.html) | `55P03` |
| SQLite | An IMMEDIATE transaction holds a write lock on the same WAL file. The current driver calls the request context from its Go busy handler. The minimum build's explicitly identified contender must enter the default native busy callback, observed from its active stack. Roll back and verify reads and a subsequent write. [SQLite busy timeout](https://www.sqlite.org/c3ref/busy_timeout.html) | `5` (`SQLITE_BUSY`) |
| SQL Server | A dedicated diagnostic login reads the exact waiting session and blocker in `sys.dm_exec_requests`. The test retires the deadline connection and validates the replacement. After native timeout it verifies the same session, zero open transactions and the restored lock timeout before returning it to the pool. [SET LOCK_TIMEOUT](https://learn.microsoft.com/en-us/sql/t-sql/statements/set-lock-timeout-transact-sql?view=sql-server-ver16) · [Active requests](https://learn.microsoft.com/en-us/sql/relational-databases/system-dynamic-management-views/sys-dm-exec-requests-transact-sql?view=sql-server-ver16) | `1222`, class `16` |
| MySQL | `performance_schema.data_lock_waits` must link the exact InnoDB threads. Native timeout leaves a separate write visible inside the contender transaction with `innodb_rollback_on_timeout=OFF`. Both transactions are rolled back, and the deadline case uses a replacement session. [MySQL 8.4 InnoDB error handling](https://dev.mysql.com/doc/refman/8.4/en/innodb-error-handling.html) · [Wait table](https://dev.mysql.com/doc/refman/8.4/en/performance-schema-data-lock-waits-table.html) | `1205/HY000` |

PostgreSQL, SQLite and SQL Server use 1 s and 10 s budgets in opposite orders. MySQL lets the InnoDB timer run for 3 s with the fixture context still active in the engine case. Its deadline case uses a 4 s caller budget and a 12 s engine budget. Two initial rows give MySQL an independent write to check. A final ordered read verifies the exact initial state for each provider. On SQLite 3.51.3 a caller deadline may become observable only after the busy wait returns. The test requires the correct cause and positive evidence of entry into the busy handler.

The server runner uses the existing disposable databases, protected markers and container-local connections. Only the dedicated SQL Server login gets the server DMV permission for that engine version. MySQL grants access to its one lock-wait table. The report admits three bounded facts for this scenario and keeps session IDs and diagnostic output outside the result.

This is driver-lab evidence for [ERR-CANCEL-001-S02](../../../specs/features/errors/err-cancel-001.feature). The production adapter and GoEF application error classification retain their implementation and acceptance gates.

## Local run on 2026-11-26

The missing S05 binding first failed `go test -mod=readonly -count=1 -run '^TestDriverFeature$' .` with exit 1. The SQLite observation control also failed when the current busy-handler selector was deliberately disabled. First focused SQL Server runs reproduced a retired deadline connection and showed that `XACT_STATE()` in the autocommit probe could return `1`. The final check verifies `@@TRANCOUNT`, session identity and `@@LOCK_TIMEOUT` directly.

The full matrix exited 0 on `7d74a1185370d11718e8e316df18d712cf46154e` from a detached checkout on the Windows/amd64 workstation. Server cases used the workstation's Docker Desktop Linux/amd64 daemon and existing [engine and driver pins](../../parity/ENGINE_COMPATIBILITY.md).

```powershell
pwsh -NoProfile -File scripts/ci/goef-w002.ps1 -Compatibility -AcceptSqlServerEula -AcceptLegacySqlServerEula
```

| Engine | Profiles that observed both waits and correctly identified the deadline and native timeout |
| --- | --- |
| PostgreSQL | 16.16, 17.12, 18.7, 19.0 |
| SQLite | 3.51.3 and 3.54.1 |
| SQL Server | 2017/140, 2019/150, 2022/160, 2022/140 |
| MySQL | 8.0.46 and 8.4.13.1 |

The 12 profiles passed 60 distinct driver cases. The output has 65 passing rows because the file suite repeats the five cases on SQLite 3.54.1. All four SQLite file cases passed. The repository validator, seven bootstrap tests, 41 Python lab tests, and Go 1.27.4 build, vet, test and module checks passed. `go test` and `go vet` also exited 0 with `-modfile=sqlite351.mod -tags=goef_sqlite_351`. The provider and driver test packages passed `go test -race -count=1 -shuffle=on ./...` in a local Linux/amd64 container. The driver test package used the existing vendor snapshot for the locked module and the container ran with networking disabled. No lab-owned containers remained after the matrix.

The verified driver module tree is `537d85a0dc5c34108ad8ebfafab660757598a1c5`, lab tree `31a282eea7d1aba4a4fed5e56df9ac96037f1bad`, local CI blob `7a4f39a33a25e183e5c6a8098af7440f46b347a7` and provider SPI tree `c335b07d4a32061204c22af28efdafb8d4ffb49d`. Independent review checked the executable revision, including the live-context requirement and exact seed checks.
