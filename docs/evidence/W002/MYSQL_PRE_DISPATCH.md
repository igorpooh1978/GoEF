# W002 MySQL cancellation before query dispatch

This pilot for [W002 #13](https://github.com/igorpooh1978/GoEF/issues/13) exercised an already-acquired `provider.Lease` against MySQL Community 8.0.46 and 8.4.13.1 through the [test-only SPI reader](../../../test/conformance/driverlab/spi_read_backend_test.go). The existing [S04 scenario](SPI_CANCELLATION.md) opens a stream and reads one row before canceling. The pilot targeted the earlier boundary, before dispatching a valid query to `database/sql`. Pool admission and interruption of an in-flight read remain separate test cases.

## Procedure

1. Start one engine at a time using the existing `driver_run.mysql_container` provisioner and `runtime.Lab`. Use its owned Docker container with networking disabled, built-in version and edition checks, and an ephemeral fixture login and target marker.
2. Create two temporary Go overlays of `spi_read_backend_test.go`. Append `TestMySQLPreDispatchPilot` to each overlay and increment a test-only integer immediately before `l.conn.QueryContext(ctx, command.SQL)`. In the guarded overlay, insert this check after validating the command shape and before incrementing that integer.

   ```go
   if err := ctx.Err(); err != nil {
       return nil, err
   }
   ```

3. In the test, run `s.setupSPICancel()`, acquire the SPI lease under a live context with a five-second limit, and check one occupied pool slot. Read the MySQL `CONNECTION_ID()` and initialize the baseline count with the following statement on the connection already held by the lease. MySQL permits this status query using the fixture login. [MySQL 8.0 SHOW STATUS](https://dev.mysql.com/doc/refman/8.0/en/show-status.html) · [MySQL 8.4 SHOW STATUS](https://dev.mysql.com/doc/refman/8.4/en/show-status.html)

   ```sql
   SHOW SESSION STATUS WHERE Variable_name = 'Com_select'
   ```

4. Call `lease.Query` with the same `provider.Command{SQL: "SELECT 1", Columns: []provider.Column{{Kind: provider.KindInteger}}}` and nil bindings in each operation. Call `cancel()` before the first operation and use `context.WithDeadline(ctx, time.Now().Add(-time.Second))` before the second. Require nil rows and `errors.Is` matching `context.Canceled` and `context.DeadlineExceeded` respectively. Check the adapter counter and session counter after each call while the lease remains occupied.
5. Run the identical command with the live context. Read the integer `1` and then clean `io.EOF`. Close the rows and require a single increment in both counters. Read `CONNECTION_ID()` again after sampling the counters and require the same identity. Release with `provider.Reuse` under the live context and require zero occupied pool slots.

Build each overlay from `test/conformance/driverlab` on Go 1.27.4 and run the resulting binary inside the provisioned container. Each overlay JSON maps the absolute path of the tracked test file to the corresponding temporary replacement. Supply the lab's generated profile, password and marker through the existing container environment mechanism. Set `GOEF_PREDISPATCH_EXPECT_GATE=1` to require adapter deltas of zero, zero and one. Set it to `0` for the baseline characterization, which expects one call to `database/sql` per operation.

```powershell
$env:GOOS = 'linux'
$env:GOARCH = 'amd64'
$env:CGO_ENABLED = '0'
$env:GOWORK = 'off'
go test -c -mod=readonly -overlay $overlayFile -o $binaryFile .
```

```text
<copied-test-binary> -test.run "^TestMySQLPreDispatchPilot$" -test.v
```

## Local results

The pilot ran on 2026-11-26 at baseline `c0a85411cb5188d859f9d590d8d0c1b479eab726` with Go 1.27.4 on Windows/amd64, Docker Engine 29.4.0 and Linux/amd64 test binaries. The lab pinned `github.com/go-sql-driver/mysql` v1.10.0 at upstream commit `a065b60ab6d0c8e15468e7709c7f76acf4431647`. Both builds exited zero.

The tracked adapter, fixture setup, module pins, MySQL provisioner and resource owner used by the pilot are identical at `7d74a1185370d11718e8e316df18d712cf46154e`. Use that revision as the replay baseline.

| Engine profile | Image digest | Baseline with gate required | Baseline characterization | Guarded overlay |
| --- | --- | --- | --- | --- |
| MySQL 8.0.46 | `sha256:62fb722c78b24245ddff1796a0fcee4a49cc5b87e0aaaf20c92d1da9e0a2497b` | Exit 1, adapter count assertion | Exit 0 | Exit 0 |
| MySQL 8.4.13.1 | `sha256:f0c7a4430d773ef8433aae82e08cb02ade4a6ad806a18b1610a48a1728fbdfd5` | Exit 1, adapter count assertion | Exit 0 | Exit 0 |

On both engines the baseline reached the `database/sql` call for each operation, producing adapter deltas `1, 1, 1`. Its MySQL session count deltas were `0, 0, 1`. The guarded overlay produced `0, 0, 1` at both boundaries. Both successful runs preserved the two caller causes, completed the identical live query on the original session, and released the lease. Only the count deltas and static outcome names were retained.

This result is consistent with the pinned driver checking the context in `watchCancel` before invoking its query path. The server counter proves absence of the counted SQL command on the held MySQL session. The adapter counter separately proves absence of a call to `database/sql` with the explicit precheck. The code change and scratch test existed only in temporary overlays. This evidence leaves durable Gherkin, other providers and production adapter behavior open. [Pinned driver source](https://github.com/go-sql-driver/mysql/blob/a065b60ab6d0c8e15468e7709c7f76acf4431647/connection.go) · [Go 1.27.4 database/sql source](https://github.com/golang/go/blob/go1.27.4/src/database/sql/sql.go)
