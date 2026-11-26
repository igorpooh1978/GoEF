# GoEF four-engine smoke lab

This first executable slice provisions disposable engines and checks their identities and baseline behavior using native clients. It checks actual databases. GoEF adapter, Go driver, Gherkin and full conformance results are separate work in [W002 #13](https://github.com/igorpooh1978/GoEF/issues/13). [Implementation #19](https://github.com/igorpooh1978/GoEF/issues/19)

Run from the repository root with Python 3.11 or newer. Server cells use a local Docker Linux/amd64 daemon and pinned image digests. SQLite uses a trusted SQLite 3.54.1 command-line executable, selected explicitly. Obtain it from [SQLite downloads](https://www.sqlite.org/download.html) and verify the vendor-published archive checksum before extracting. The runner checks the version and source ID at runtime.

```sh
python -B test/conformance/lab/run.py postgres mysql
python -B test/conformance/lab/run.py sqlite --sqlite-binary "path/to/sqlite3"
python -B test/conformance/lab/run.py sqlserver --accept-sqlserver-eula
python -B test/conformance/lab/run.py all --sqlite-binary "path/to/sqlite3" --accept-sqlserver-eula
```

Supply `--accept-sqlserver-eula` only after accepting the [Microsoft SQL Server terms for the Developer edition run](https://learn.microsoft.com/en-us/sql/linux/sql-server-linux-configure-environment-variables). The runner requires an explicit flag for each requested SQL Server run.

The server containers have no published ports, external network or host data mounts. Native clients run inside their own container. SQLite uses a private temporary database file. The runner emits safe facts per requested engine and fails the invocation if any requested cell fails or is blocked. It verifies ownership before removing resources it created, and cleanup failures remain visible.

Local runner regression tests:

```sh
python -B -m unittest discover -s test/conformance/lab -p test_runtime.py
```

See the [design](../../../docs/development/CONFORMANCE_LAB.md), [candidate artifact pins](../../../docs/evidence/W002/TEST_LAB_DESIGN.md), and [first run evidence](../../../docs/evidence/W002/LAB_SMOKE.md).
