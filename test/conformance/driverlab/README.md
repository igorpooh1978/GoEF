# W002 Go driver characterization

Use one local PowerShell command from the repository root:

```powershell
pwsh -NoProfile -File scripts/ci/goef-w002.ps1 -Compatibility -AcceptSqlServerEula -AcceptLegacySqlServerEula
```

The workstation needs Go 1.27.4, Python 3.11 or later with the repository validator's approved dependencies, PowerShell 7, and a local Linux/amd64 Docker daemon. Go verifies the locked test-only modules through the public Go checksum database. Docker retrieves pinned server images if the workstation cache does not contain them. GitHub Actions and remote runners are outside this pipeline.

The standard run checks the four reference profiles. `-Compatibility` adds PostgreSQL 16.16, 17.12 and 19.0, MySQL 8.0.46, SQL Server 2017 and 2019, SQL Server 2022 at level 140, and the two SQLite builds. Pass `-AcceptSqlServerEula` only for an authorized local SQL Server 2022 Developer run at levels 140 and 160. Pass `-AcceptLegacySqlServerEula` only for an authorized local SQL Server 2017 and 2019 Developer run. A missing consent flag blocks the affected cells and fails the selected matrix. [Microsoft container setup](https://learn.microsoft.com/en-us/sql/linux/containers/deploy?view=sql-server-2017)

To run one profile without the other CI checks:

```powershell
python -B test/conformance/lab/driver_run.py postgres --profile postgres-16
python -B test/conformance/lab/driver_run.py postgres --profile postgres-17
python -B test/conformance/lab/driver_run.py postgres --profile postgres-19
python -B test/conformance/lab/driver_run.py mysql --profile mysql-8.0
python -B test/conformance/lab/driver_run.py sqlserver --profile sqlserver-2017 --accept-legacy-sqlserver-eula
python -B test/conformance/lab/driver_run.py sqlserver --profile sqlserver-2019 --accept-legacy-sqlserver-eula
python -B test/conformance/lab/driver_run.py sqlserver --profile sqlserver-2022-140 --accept-sqlserver-eula
python -B test/conformance/lab/sqlite_compat.py
```

The runner builds temporary Go test binaries. The server binary runs inside the owned database container. The container has no published ports, bridge network, daemon socket mount or host data mount. Provisioner credentials create the fixture identities and a random protected marker. Database writes occur only after runtime identity and marker validation. SQLite uses the embedded driver against a private on-disk database. Cleanup and the four-engine status are part of acceptance.

The executable [Gherkin feature](features/driver.feature) uses tagged scenario outlines, bound Given/When/Then steps and a provider examples table. Each profile must emit four validated scenario results. The first three verify commit and rollback, native uniqueness and foreign-key codes, and unchanged rows through an independent observer connection. The fourth exercises cancellation after the first row through the GoEF provider execution interface using a test adapter. It verifies the cancellation cause, closes and releases the stream lease, and completes another query with a fresh context on a one-connection pool. A scenario passes only after its cleanup. Unbound steps, duplicate or missing examples, wrong engine versions and missing reports fail the gate.

The SQL Server 2022 profiles also check that `GENERATE_SERIES` returns the expected values at compatibility level 160 and is unavailable at 140 under the fixture's database settings.

The [SQLite file scenarios](features/sqlite_files.feature) run both directions between SQLite 3.51.3, the W002 minimum, and the 3.54.1 reference. The peer opens the existing file, verifies that an invalid `STRICT` generated value returns native code `3091`, exercises rollback and commits an additive migration. The original runtime then reopens the same file and checks schema, rows and integrity. The suite covers `DELETE` and `WAL` journaling with a schema shared by both versions. The local matrix requires the exact minimum profile and the observed `3091` in each file result.

See the [engine compatibility profiles](../../../docs/parity/ENGINE_COMPATIBILITY.md), [driver evidence](../../../docs/evidence/W002/DRIVER_GHERKIN.md), [expanded compatibility evidence](../../../docs/evidence/W002/LEGACY_COMPATIBILITY.md), [cancellation evidence](../../../docs/evidence/W002/SPI_CANCELLATION.md) and [remaining work](https://github.com/igorpooh1978/GoEF/issues/13). Full GoEF application scenarios retain their own acceptance requirements.
