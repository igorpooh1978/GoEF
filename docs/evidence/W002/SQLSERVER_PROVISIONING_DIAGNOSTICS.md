# SQL Server provisioning diagnostics in local CI

[Local CI #26](https://github.com/igorpooh1978/GoEF/issues/26) · [Conformance #13](https://github.com/igorpooh1978/GoEF/issues/13) · [Driver lab](../../../test/conformance/driverlab/README.md)

Driver lab revision `14801e99f68ad5d5dd939eb9ed73306e135a0cd0` adds a bounded `stage` field to SQL Server provisioning failures. Its four values identify Docker startup, engine readiness, identity verification and fixture setup. The coarse `phase`, result status and successful scenario shape retain their existing behavior. Identity mismatch keeps its fixed message. Reasons are selected by known exception types and a cleanup failure adds a fixed indicator. SQL, connection configuration, subprocess arguments and arbitrary exception messages stay out of these diagnostics.

The focused negative control failed on all four timeout injections before the stage was added. The missing safe identity mismatch message also failed its control before the refinement. The combined candidate passed 60 tests with `python -B -m unittest discover -s test/conformance/lab -p 'test*.py'`. Synthetic failures cover timeouts at the four stages, mismatch, redaction, blocked startup and a simultaneous cleanup failure. The failure controls run with mocked subprocess boundaries.

At the same revision, the workstation ran the stock SQL Server 2022 Developer profile at compatibility level 160 against build `16.0.4325.4` in local Docker.

```powershell
python -B test/conformance/lab/driver_run.py sqlserver --profile sqlserver-2022 --accept-sqlserver-eula
```

The command exited 0 and all seven existing scenarios passed on the first attempt. The successful report retained exactly `profile`, `provider`, `scenarios` and `status`, and cleanup left zero lab-owned containers. The redacted report SHA-256 is `7a633206f70d9ed2d0976cdf04d31e35cbccf6de18cfdfec181a08f091db059b`.

At the same code revision, the root and driver lab Go suites passed `go test -race -count=1 -shuffle=on ./...` in a local Go 1.27.4 Linux/amd64 container with networking disabled. The root used `-mod=readonly`. The driver module used the locked vendor snapshot with `-mod=vendor`. The live PostgreSQL and MySQL profile results are recorded in their focused reports. [PostgreSQL row iteration](POSTGRES_NEXT_IN_FLIGHT.md) · [MySQL deadlock](MYSQL_DEADLOCK.md)
