# W002 PostgreSQL in-flight row iteration pilot

[Issue #13](https://github.com/igorpooh1978/GoEF/issues/13) · [Cancellation contract](../../development/CONFORMANCE_LAB.md) · [Portable open-reader characterization](SPI_CANCELLATION.md)

On 2026-11-26 the native row iteration pilot ran against baseline `f42d0f95bfb77a15702e0c73fceaca154470e1b6` with a scratch Go test overlay. The Windows/amd64 workstation used Go 1.27.4, Python 3.14.4 and the local Docker Desktop daemon 29.4.0 on Linux/amd64. The existing `postgres-18` profile pinned PostgreSQL 18.7 with `server_version_num=180007` and image `docker.io/library/postgres@sha256:32f5fdbf4d27ebe0d4552313126a593400f40b21bff207e43617b2305838bce3`. The test used pgx v5.13.0 stdlib in `exec` mode and `database/sql.Rows`. [Pinned image](../../../test/conformance/lab/postgres.py) · [PG fixture](../../../test/conformance/driverlab/postgres_test.go)

## Observed schedule

1. The holder acquired a transaction advisory lock. The independent reader used an instrumented copy of the pinned pgx settings with the same scoped role. It verified the run marker and installed a temporary PL/pgSQL function under `pg_temp`. The function raised a fixed `NOTICE` on its second invocation and then waited on the holder's lock.
2. One ordered SELECT evaluated the volatile output expression for input rows 1 and 2. PostgreSQL flushed its pending protocol output when emitting the notice before the second row. The reader's `QueryContext` completed, and the first `Rows.Next` and `Scan` returned 1. The notice callback had not run at that point. [Output evaluation timing](https://www.postgresql.org/docs/18/sql-select.html#SQL-SELECT-LIST) · [PostgreSQL 18.7 notice flush](https://github.com/postgres/postgres/blob/REL_18_7/src/backend/utils/error/elog.c#L3494-L3502)
3. The holder observed its PID in `pg_blocking_pids(reader_pid)`. A single worker entered the second native `database/sql.Rows.Next` on the same result. pgx invoked its `OnNotice` callback for the fixed marker from inside this call. The holder checked the same blocker again while the caller context remained live and the worker had yet to return. [pgx first-row preload](https://github.com/jackc/pgx/blob/v5.13.0/stdlib/sql.go#L486-L507) · [pgx notice dispatch](https://github.com/jackc/pgx/blob/v5.13.0/pgconn/pgconn.go#L697-L700) · [Backend blocker function](https://www.postgresql.org/docs/18/functions-info.html#FUNCTIONS-INFO-SESSION)
4. In the cancellation run the caller canceled and joined the worker. Its second `Next` returned false and `errors.Is(rows.Err(), context.Canceled)` succeeded. The test closed the reader, discarded the connection and observed zero open or in-use reader pool connections. Fresh activity observations showed the old reader backend absent while the holder still retained the lock. After holder rollback, a newly borrowed connection with a different backend read the original committed fixture row. [Activity observation](https://www.postgresql.org/docs/18/monitoring-stats.html#MONITORING-PG-STAT-ACTIVITY)
5. In the control run the holder instead rolled back while the request was live. The same second `Next` returned row 2, and the following `Next` reached clean EOF.

This pilot observed entry into native row iteration on a single open result. It used the driver's protocol notice callback as the entry observation. It leaves transport-syscall observation, production adapter semantics and other engine versions or drivers to separate cases.

## Re-execution and evidence

From the repository root, run the launcher with its adjacent Go scratch input.

```python
import os
from pathlib import Path
import subprocess
import sys

launcher = Path(os.environ["GOEF_POSTGRES_NEXT_PROBE_DIR"]) / "run_postgres_next.py"
subprocess.run([sys.executable, "-B", str(launcher)], check=True)
```

The launcher used Go's overlay flag to add the scratch test to the existing locked module. It compiled `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go test -c -mod=readonly -overlay=<overlay.json> -ldflags="-s -w" -o <binary> .` from `test/conformance/driverlab`. The owned local PostgreSQL container executed `-test.run=^TestPostgresNativeNextProbe$ -test.v -test.count=1 -test.timeout=45s`. The launcher exited 0 after checking both expected results and cleanup. The two source files stayed outside the checkout.

| Scratch input | SHA-256 |
| --- | --- |
| `postgres_next_probe_test.go` | `d0fd22ce6f6c07121ca1a2478b7a83bfe36b1da2bfcc58650da9b8f03a556481` |
| `run_postgres_next.py` | `467164013d7e1c59d4bc72c161091de3893d82d955de54330542824a2d135764` |

## Checked-in test across all four PostgreSQL profiles

The pilot is now available as `TestPostgresNativeNext` with the dedicated [local launcher](../../../test/conformance/lab/postgres_next.py). The launcher requires explicit pinned profile names, builds one Go test binary, runs only the focused test in owned containers and validates both outcome records after each cleanup.

```sh
python -B test/conformance/lab/postgres_next.py --profile postgres-16 --profile postgres-17 --profile postgres-18 --profile postgres-19
```

On local executable revision `05469eb1e2024b1e752cec47bc9f216ed80ddc8a`, the command exited 0. Both cancellation and holder-release controls passed on PostgreSQL 16.16, 17.12, 18.7 and 19.0 with pgx v5.13.0. All eight results confirmed entry into the second native `Rows.Next` while the lock wait was observable. The canceled call returned `context.Canceled`. The test discarded the connection, observed the backend exit before unlock and recovered with the original row. The control returned row 2 followed by clean EOF. No retries were needed and no GoEF lab containers remained after the run. The redacted report SHA-256 is `d7baa8f2138a152c1d8a2bb24884435e516f84146a94c77c34cbe92cfd53fda3`.
