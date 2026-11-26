# SQL Server blocked-read cancellation across compatibility profiles

[W002 #13](https://github.com/igorpooh1978/GoEF/issues/13) · [SQL Server 2022 at level 160](SQLSERVER_IN_FLIGHT_READ.md) · [Lock-wait evidence](LOCK_TIMEOUT.md) · [Driver lab](../../../test/conformance/driverlab/README.md)

On 2026-11-26, three disposable SQL Server Developer instances passed the local blocked-read cancellation pilot on the first attempt. Each used Go 1.27.4, go-mssqldb v1.11.0, Docker Desktop's local Linux/amd64 daemon, the pinned SQL Server image and the existing authenticated fixture. The lab verified the engine build and database compatibility level for each profile. The SQL Server 2022 level 160 observation was recorded in the earlier pilot.

| Lab profile | Engine build | Database compatibility level | Pinned image digest |
| --- | --- | --- | --- |
| `sqlserver-2017` | `14.0.3570.2` | `140` | `sha256:5f6fd78f8752390dbd3584b91dee463f1f4b2b30beef9af3fd35753d32940c52` |
| `sqlserver-2019` | `15.0.4510.2` | `150` | `sha256:debdc21628cddd1a4433371c601c8dee9bad3533ae8b18fe9dc8c5596368b10d` |
| `sqlserver-2022-140` | `16.0.4325.4` | `140` | `sha256:4525e3efac9f3e241cb003bb2ff2a82818b502f37e12049d788b1a9fa7e13b7f` |

## Observed behavior

A holder updated a fixture row in an open transaction. A separate physical connection attempted to read that row through native `*sql.Conn.QueryContext` with a bound primary key, `READCOMMITTEDLOCK`, `MAXDOP 1` and session lock timeout `-1`. A third connection authenticated as the lab's diagnostic login observed the exact reader, blocking holder and database in `sys.dm_exec_requests` while the reader context was live. Each profile observed the wait in both the cancellation and control arms. [Active requests](https://learn.microsoft.com/en-us/sql/relational-databases/system-dynamic-management-views/sys-dm-exec-requests-transact-sql?view=sql-server-ver16)

| Check | Result on each profile |
| --- | --- |
| Holder-release control | Rolling back the holder while the reader remained live yielded the expected committed row followed by clean exhaustion. |
| Native query cancellation | The caller canceled after the server confirmed the wait. `QueryContext` returned an error matching `context.Canceled` before transferring rows. The test separately excluded a deadline and a native SQL Server error. |
| Server-side termination | The diagnostic request disappeared while the holder transaction remained active. |
| Recovery | The test rolled back the holder, discarded the canceled reader and acquired a replacement connection. It confirmed the fixture marker, zero open transactions, the original lock timeout and the unchanged fixture row. |

The observation is at the native query boundary before SPI error handling. Direct evidence for native `Rows.Next` during a server-side wait, wire-level TDS Attention acknowledgement and integration of this pilot into the executable suite remain open. [Pinned driver query and row handling](https://github.com/microsoft/go-mssqldb/blob/v1.11.0/mssql.go#L789-L972)

## Run record

The worktree started clean at `64d8fe3aab400e52e16743180417822d66ca17ba`. A temporary overlay added `TestDriverSQLServerBlockedReadPilot` to the driver lab package. The launcher used the existing lab build and provisioning functions and ran the three profiles sequentially, each in a fresh container. It supplied the consent corresponding to `--accept-legacy-sqlserver-eula` for 2017 and 2019 and to `--accept-sqlserver-eula` for 2022 at level 140. The lab finished each container's cleanup before returning its result.

| Operation | Status |
| --- | --- |
| Linux/amd64 cross-build with `go test -c -mod=readonly -overlay=<temporary-overlay> .` | Exit 0 |
| Windows compile check with `go test -mod=readonly -overlay=<temporary-overlay> -run '^$' .` | Exit 0 |
| Focused test `-test.run=^TestDriverSQLServerBlockedReadPilot$` on all three profiles | Launcher exit 0. Each profile passed once with cleanup confirmed. |

The temporary launcher and redacted report remain in workstation temporary storage. Their SHA-256 hashes are `7ecc2bd503c0c60d4747fb87a921b5542dd2e3a8c7d700b7b043b0ba6e2e8e4d` and `1f132bf21752e3ba21bb15df9d037168121dfeb2e72023e4d34a2efbbd6fcb4a`, respectively.
