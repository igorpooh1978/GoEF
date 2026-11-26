# W002 PostgreSQL in-flight query opening pilot

[Issue #13](https://github.com/igorpooh1978/GoEF/issues/13) · [Observed cancellation contract](../../development/CONFORMANCE_LAB.md) · [Open-reader characterization](SPI_CANCELLATION.md)

On 2026-11-26 a scratch Go test overlay ran against `f42d0f95bfb77a15702e0c73fceaca154470e1b6` on the local Windows/amd64 workstation. The test used Go 1.27.4, Python 3.14.4, the local Docker Desktop 29.4.0 Linux/amd64 daemon and the existing `postgres-18` profile. The server was PostgreSQL 18.7 with `server_version_num=180007`. The driver was pgx v5.13.0 through `database/sql` with the profile's `exec` mode. The pinned Linux/amd64 image was `docker.io/library/postgres@sha256:32f5fdbf4d27ebe0d4552313126a593400f40b21bff207e43617b2305838bce3`. The fixture verified the provisioned database, the scoped role and the run marker before writing its synthetic row. [Driver lab runner](../../../test/conformance/lab/driver_run.py)

## Schedule and results

1. The fixture opened two independent leases and inserted the committed parent. The holder updated that one row with a bound statement and kept its transaction open.
2. The reader issued one `SELECT ... FOR UPDATE` against the same row on its own backend. The holder observed its own PID in `pg_blocking_pids(reader_pid)` before canceling or releasing the lock. The request context was live and the reader worker had produced no result at this barrier. Only the reader worker used the waiting connection.
3. In both cancellation runs the holder kept the lock, canceled the reader context and joined the worker. The query call returned no rows and its error matched `context.Canceled` through `errors.Is`.
4. The test discarded the reader lease and polled fresh `pg_stat_activity` observations until its backend disappeared. The holder was still active and retaining its lock. After rolling back the holder, the fixture read the unchanged committed row and a fresh SPI lease returned the two expected integers followed by clean EOF.

| Path | Observed phase | Terminal result |
| --- | --- | --- |
| Native `database/sql.Conn.QueryContext` | Query opening after server dispatch | `context.Canceled`, backend exited before the holder released the lock, and fresh SPI query completed |
| Test-only `provider.Lease.Query` | Query opening after one SPI dispatch | `context.Canceled`, backend exited before the holder released the lock, and fresh SPI query completed |
| Holder-release control | Same server-observed wait with a live request | The holder rolled back without caller cancellation, the reader opened and returned the original row, and iteration reached clean EOF |

pgx stdlib preloads the first row inside its `QueryContext` implementation. The experiment observed the call opening the reader. Interrupting a blocked `Rows.Next` after a row has been delivered remains a separate acceptance case. Engine timeout discrimination and production adapter behavior retain independent evidence. [Pinned pgx query implementation](https://github.com/jackc/pgx/blob/v5.13.0/stdlib/sql.go#L486-L507) · [PostgreSQL row locks](https://www.postgresql.org/docs/18/explicit-locking.html#LOCKING-ROWS) · [Backend and blocker functions](https://www.postgresql.org/docs/18/functions-info.html#FUNCTIONS-INFO-SESSION) · [Activity observation](https://www.postgresql.org/docs/18/monitoring-stats.html#MONITORING-PG-STAT-ACTIVITY)

## Local command and evidence provenance

From the repository root, run the scratch launcher in the directory containing the two overlay inputs.

```powershell
python -B "$env:GOEF_POSTGRES_INFLIGHT_PROBE_DIR/run_postgres_inflight.py"
```

The launcher mapped `postgres_inflight_probe_test.go` into the test package with Go's overlay flag. It compiled the existing locked module with `GOOS=linux`, `GOARCH=amd64` and `CGO_ENABLED=0` using `go test -c -mod=readonly -overlay=<overlay.json> -ldflags="-s -w" -o <binary> .` inside `test/conformance/driverlab`. The local runner executed that binary with `-test.run=^TestPostgresInFlightReadProbe$ -test.v -test.count=1 -test.timeout=45s`. The launcher exited 0 after requiring all three expected markers and successful cleanup. Both inputs were scratch files kept outside the checkout.

| Scratch input | SHA-256 |
| --- | --- |
| `postgres_inflight_probe_test.go` | `a758b26bf44f280279696a01b2fe7c3906737b35c0a069039192abe507b3f688` |
| `run_postgres_inflight.py` | `e3f1a2edf8f787e3e749324800113ea9d20cf5aa8593a1e2a70996839398afa1` |
