# SQL Server blocked-read cancellation pilot

[W002 #13](https://github.com/igorpooh1978/GoEF/issues/13) · [Existing lock-wait evidence](LOCK_TIMEOUT.md) · [Driver lab](../../../test/conformance/driverlab/README.md)

On 2026-11-26, the local pilot passed on SQL Server 2022 CU28 Developer build `16.0.4325.4` at database compatibility level `160`. It used Go 1.27.4, go-mssqldb v1.11.0 and Docker Desktop's local Linux/amd64 daemon 29.4.0. The fixture used TCP, mandatory TLS, READ COMMITTED, disabled read-committed snapshot and the existing run marker. The image was `mcr.microsoft.com/mssql/server@sha256:4525e3efac9f3e241cb003bb2ff2a82818b502f37e12049d788b1a9fa7e13b7f`. The local invocation used the existing SQL Server 2022 Developer license consent.

## Observed behavior

The holder updated one fixture row in an open transaction. A second physical connection read the same row through native `*sql.Conn.QueryContext` using a bound primary key, `READCOMMITTEDLOCK` and `MAXDOP 1`. Its session lock timeout was `-1`. The dedicated diagnostic login checked the exact waiting session, blocker and database in `sys.dm_exec_requests`. Both waits were observed while the request context was active. [Shared locking hint](https://learn.microsoft.com/en-us/sql/t-sql/queries/hints-transact-sql-table#readcommittedlock) · [Lock timeout](https://learn.microsoft.com/en-us/sql/t-sql/statements/set-lock-timeout-transact-sql?view=sql-server-ver16) · [Active requests](https://learn.microsoft.com/en-us/sql/relational-databases/system-dynamic-management-views/sys-dm-exec-requests-transact-sql?view=sql-server-ver16)

| Check | Result |
| --- | --- |
| Holder-release control | The holder rolled back without canceling the reader. The reader obtained the one expected committed row, then clean exhaustion. |
| Cancellation while the holder remained active | `QueryContext` returned an error matching `context.Canceled` before transferring rows. The test separately excluded a deadline and a typed SQL Server diagnostic. |
| Server-side termination | The reader's request disappeared from the diagnostic view while the holder transaction was still active. |
| Recovery | The test rolled back the holder, discarded the reader connection and acquired a replacement. It confirmed the marker, zero open transactions, the original lock timeout and the unchanged fixture row. |

The cancellation was observed at the native query boundary before any SPI error handling. The pinned driver's query call reads result metadata before transferring `Rows`. Coverage of cancellation inside native `Rows.Next`, wire capture of the TDS Attention acknowledgement and executable suite integration remain open. SQL Server 2017, SQL Server 2019 and SQL Server 2022 at level 140 retain their own blocked-read checks. [Driver query and row handling](https://github.com/microsoft/go-mssqldb/blob/v1.11.0/mssql.go#L789-L972) · [Driver Attention processing](https://github.com/microsoft/go-mssqldb/blob/v1.11.0/token.go#L1266-L1359) · [TDS Attention state](https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-tds/61fb4f9d-95ec-4171-8d4b-dac4eb16149f)

## Run record

The worktree was clean at `f42d0f95bfb77a15702e0c73fceaca154470e1b6`. A temporary Go overlay added `TestDriverSQLServerBlockedReadPilot` to the driver lab package. The scratch launcher remains in workstation temporary storage with SHA-256 `5d14103e02c15bf6ff56ae3899da6d178c334ad4cf85243c7c3ce516d100425f`.

| Operation | Status |
| --- | --- |
| Linux/amd64 cross-build with `go test -c -mod=readonly -overlay=<temporary-overlay> .` | Exit 0 |
| Windows compile check with `go test -mod=readonly -overlay=<temporary-overlay> -run '^$' .` | Exit 0 |
| Focused local `sqlserver-2022` run with the existing `--accept-sqlserver-eula` choice and `-test.run=^TestDriverSQLServerBlockedReadPilot$` | Exit 0 |
| `docker ps -aq --filter label=org.goef.lab.owner` after the run | Exit 0, zero lab-owned containers |
