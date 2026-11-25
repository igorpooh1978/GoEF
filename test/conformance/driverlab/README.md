# W002 Go driver characterization

Use one local PowerShell command from the repository root:

```powershell
pwsh -NoProfile -File scripts/ci/goef-w002.ps1 -AcceptSqlServerEula -Compatibility
```

The workstation needs Go 1.27.4, Python 3.11 or later with the repository validator's approved dependencies, PowerShell 7, and a local Linux/amd64 Docker daemon. Go verifies the locked test-only modules through the public Go checksum database. Docker retrieves pinned server images if the workstation cache does not contain them. GitHub Actions and remote runners are outside this pipeline.

The standard run checks the four reference profiles. Add `-Compatibility` for PostgreSQL 16.16 and MySQL 8.0.46. Pass `-AcceptSqlServerEula` only for an authorized local SQL Server 2022 Developer run. A missing flag blocks that cell and fails the full matrix. [Microsoft container setup](https://learn.microsoft.com/sql/linux/quickstart-install-connect-docker)

To run one profile without the other CI checks:

```powershell
python -B test/conformance/lab/driver_run.py postgres --profile postgres-16
python -B test/conformance/lab/driver_run.py mysql --profile mysql-8.0
```

The runner builds temporary Go test binaries. The server binary runs inside the owned database container. The container has no published ports, bridge network, daemon socket mount or host data mount. Provisioner credentials create the fixture identities and a random protected marker. Database writes occur only after runtime identity and marker validation. SQLite uses the embedded driver against a private on-disk database. Cleanup and the four-engine status are part of acceptance.

The executable [Gherkin feature](features/driver.feature) uses a strict English subset with tagged scenario outlines, bound Given/When/Then steps and a provider examples table. Each profile must emit exactly three validated scenario results. The cases verify commit and rollback, native uniqueness and foreign-key codes, and unchanged rows through an independent observer connection. A scenario passes only after its cleanup. Unbound steps, duplicate or missing examples, wrong engine versions and missing reports fail the gate.

See the [engine compatibility profiles](../../../docs/parity/ENGINE_COMPATIBILITY.md), [W002 driver evidence](../../../docs/evidence/W002/DRIVER_GHERKIN.md) and [remaining work](https://github.com/igorpooh1978/GoEF/issues/13). These are native driver characterizations. Full GoEF application scenarios retain their own acceptance requirements.
