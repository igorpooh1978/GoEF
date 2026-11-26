# W002 cancellation while waiting for a pool connection

[W002 #13](https://github.com/igorpooh1978/GoEF/issues/13) · [Executable feature](../../../test/conformance/driverlab/features/driver.feature) · [Provider execution contract](../../architecture/PROVIDER_SPI.md)

`LAB-DRIVER-001-S07` begins with a real driver connection held on a one-slot test pool. A second SPI acquisition starts with a live context and no available slot. The test requires the pool's wait count to increase by one before it cancels the caller. In Go 1.27.4 the counter increments under the pool lock as the pending acquisition is enqueued. [Pinned Go queue implementation](https://github.com/golang/go/blob/go1.27.4/src/database/sql/sql.go#L1277-L1295)

The queued call must finish with `context.Canceled` and return no lease while the holder still owns the connection. The test then releases the holder, acquires a lease for a two-row control query, reads to clean end of stream and releases the result and connection. The successful control and zero occupied slots establish pool recovery on each real driver. Server-side query cancellation has its own test schedule.

## Local run on 2026-11-26

The new outline first failed discovery because it had no step binding. In a focused SQLite 3.54.1 control, a temporary overlay detached the queued call from its cancellation context. The test detected that the waiter stayed queued, failed with exit 1 and joined the worker after releasing the held slot. With ordinary cancellation restored, the focused lane passed ten repeats each on SQLite 3.54.1 and 3.51.3.

The full matrix ran at executable revision `3733fa7309484b9645b984536a3ec49989201fbd` on the Windows/amd64 workstation with Go 1.27.4 and the local Docker Desktop Linux/amd64 daemon. It exited 0.

To select the full matrix with the current Python runner, run the following command from the repository root.

```sh
python -B scripts/ci/goef_w002.py --compatibility --accept-sqlserver-eula --accept-legacy-sqlserver-eula
```

The 12 profiles passed 84 distinct driver cases. The log contains 91 passing driver rows because the file suite also runs the seven driver cases on SQLite 3.54.1. All four SQLite file cases passed. Every profile reported `context.Canceled`, exactly one queued acquisition and zero occupied connections after the control query. The repository validator, seven bootstrap tests, 47 Python lab tests and Go build, vet, test and module checks passed. Tests and vet also passed with the SQLite minimum lockfile and build tag.

Both Go modules passed `go test -race -count=1 -shuffle=on ./...` in the workstation's Linux/amd64 Go 1.27.4 container with networking disabled. The driver module used its existing locked vendor snapshot. With `GOEF_DRIVERLAB_PROVIDER=sqlite` and `GOEF_DRIVERLAB_PROFILE=sqlite-3.54`, `go test -mod=vendor -race -count=10 -v -run 'TestDriverGherkin/sqlite/LAB-DRIVER-001-S07' .` passed ten actual SQLite repeats and emitted ten passing scenario records.

The verified driver module tree is `ebd75f40eb7242947ca2232093ee519cb75d2c2f`, lab tree `a9a3bba54532538d06d13da3c4123fd02b6457b7`, local CI blob `8745f0683c4b21a46a1e3eef24b86a608397cd71` and provider SPI tree `c335b07d4a32061204c22af28efdafb8d4ffb49d`. The evidence revision preserves the executable artifacts.
