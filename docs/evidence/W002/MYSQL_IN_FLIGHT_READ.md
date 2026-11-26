# MySQL in-flight read cancellation through the test execution SPI

[Issue #13](https://github.com/igorpooh1978/GoEF/issues/13) · [Driver lab](../../../test/conformance/driverlab/README.md) · [Execution SPI](../../architecture/PROVIDER_SPI.md) · [Executable fixture](../../../test/conformance/driverlab/mysql_in_flight_read_test.go)

## Schedule and acceptance

The fixture seeds one committed row in an isolated InnoDB table. A separate holder transaction updates that row and retains its lock. The reader starts an explicit transaction on the physical connection leased through the test SPI. Its locking read targets the seeded primary key. The holder observes that exact waiting thread and blocking thread in `performance_schema.data_lock_waits` while the request context remains live. The wait must occur before the worker returns a row or reports an error. [InnoDB locking reads](https://dev.mysql.com/doc/refman/8.4/en/innodb-locking-reads.html) · [MySQL 8.0 wait observations](https://dev.mysql.com/doc/refman/8.0/en/performance-schema-data-lock-waits-table.html) · [MySQL 8.4 wait observations](https://dev.mysql.com/doc/refman/8.4/en/performance-schema-data-lock-waits-table.html)

1. The live-context control releases the holder. The reader returns integer `1` and reaches clean EOF. Its transaction rolls back and the lease returns to the one-slot pool.
2. The cancellation uses a second observed wait. The caller cancels before the holder releases its lock. The worker returns no row and reports a cause matching `context.Canceled`. The result marker records whether the error surfaced during `Query` or the first `Next`.
3. The fixture closes any transferred rows and discards the canceled SPI lease while the holder is still active. The reader pool reports zero open and zero in-use connections. The holder rolls back, then a physically independent replacement verifies the isolated target marker, the unchanged committed row and a successful two-row SPI query.

The pinned driver closes the connection when the query context expires before the driver receives its result. The fixture proves the client's cancellation cause and lease cleanup. Server-side termination timing while the holder remains active has a separate evidence gate. [Pinned driver cancellation contract](https://github.com/go-sql-driver/mysql/blob/v1.10.0/README.md#contextcontext-support) · [Driver wait and close implementation](https://github.com/go-sql-driver/mysql/blob/v1.10.0/connection.go)

## Workstation procedure

```powershell
python -B test/conformance/lab/mysql_in_flight_read.py
```

Add `--profile mysql-8.0` or `--profile mysql-8.4` to select one pinned profile. The launcher builds the exact module for Linux/amd64, uses the isolated local driver-lab provisioner and requires one allowlisted evidence marker per engine. The marker retains version, phase and outcomes. The run marker, connection and thread identities, SQL and credentials stay out of the report. [Launcher](../../../test/conformance/lab/mysql_in_flight_read.py) · [Evidence validation tests](../../../test/conformance/lab/test_mysql_in_flight_read.py)

The fixture worktree started at `85799b679601610a1fb72971b74c38a40d9f7224`. On Windows/amd64 with Go 1.27.4, the driver lab passed `go vet -mod=readonly ./...` and `go test -mod=readonly -count=1 -shuffle=on ./...`. The focused evidence validation test exited 1 before the launcher existed and 0 afterward. The integrated revision `c9a438928cc25b0c8ee8fde7bf78e27d037d6e65` ran the procedure on the workstation on 2026-11-26 and exited 0. Both profiles observed the exact InnoDB threads, interrupted `Query` with `context.Canceled`, returned the reader pool to zero open and zero in-use connections, and observed the unchanged committed row on a fresh connection. The release control returned the row and reached EOF on both versions.

| Profile | Server | Go driver | Engine result |
| --- | --- | --- | --- |
| mysql-8.0 | 8.0.46 | go-sql-driver/mysql v1.10.0 | PASS |
| mysql-8.4 | 8.4.13.1 | go-sql-driver/mysql v1.10.0 | PASS |
