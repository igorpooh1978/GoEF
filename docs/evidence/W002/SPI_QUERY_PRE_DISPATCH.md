# W002 query cancellation before dispatch

[W002 #13](https://github.com/igorpooh1978/GoEF/issues/13) · [Executable feature](../../../test/conformance/driverlab/features/driver.feature) · [Provider execution contract](../../architecture/PROVIDER_SPI.md)

`LAB-DRIVER-001-S06` acquires a real driver connection through the test-only provider adapter and retains the same lease for three calls. A context canceled before the first call yields no rows and preserves `context.Canceled`. A context whose deadline has already passed yields no rows and preserves `context.DeadlineExceeded`. The same two-row query under the live fixture context returns both integers, reaches clean EOF and closes. The pool reports zero occupied connections after release.

The serial lease counts each call at its `database/sql.Conn.QueryContext` boundary. The expected changes are `0,0,1`. The live control checks that the instrumented path runs on each real engine. The [MySQL pilot](MYSQL_PRE_DISPATCH.md) also measured the native session's command count on both pinned MySQL builds. An already-ended context and a context that ends during dispatch or a read are different test schedules.

A control overlay on SQLite 3.54.1 removed only the context check. The focused test exited 1 because the canceled call reached `database/sql`. With the check restored, the same targeted test exited 0 on SQLite 3.54.1 and 3.51.3. The local report gate requires the two exact causes, the three dispatch counts and zero occupied connections before accepting each profile.

## Local run on 2026-11-26

The Windows/amd64 workstation used Go 1.27.4 and its local Docker Desktop Linux/amd64 daemon. The driver tests and reports were generated from `990737d1b93b814fa56d2de7c12cbe09d78199fd`. The historical matrix run exited 1 because the SQL Server 2022 level 160 fixture timed out during provisioning. The other profile and SQLite file results passed. The SQL Server 2022 replay at the same revision exited 0 and returned all six passing scenarios.

Use these commands to select the full matrix and the focused SQL Server 2022 replay with the current Python tools.

```sh
python -B scripts/ci/goef_w002.py --compatibility --accept-sqlserver-eula --accept-legacy-sqlserver-eula
python -B test/conformance/lab/driver_run.py sqlserver --profile sqlserver-2022 --accept-sqlserver-eula
```

Across the two runs, 12 profiles passed 72 distinct driver cases and four SQLite file cases. The 78 passing driver rows include six repeats on SQLite 3.54.1. Every profile reported the two exact causes, `0,0,1` calls and zero occupied connections. The historical CI gate accepted the four real baseline reports with the SQL Server replay. It rejected an altered baseline report with dispatch counts `1,0,1`. No lab-owned containers remained.

The repository validator, seven bootstrap tests, 44 Python lab tests and Go build, vet, test and module checks passed. The SQLite minimum test and vet also passed with `-modfile=sqlite351.mod -tags=goef_sqlite_351`. Both Go suites passed `-race -count=1 -shuffle=on` locally in Linux/amd64 with networking disabled. The driver module used its locked vendor snapshot for the race check.

The verified driver module tree is `64ad2e293b171050cb3c93a420729de97dedb8e5`, lab tree `aa5ff22ba43712ee769670ac5a881af7792fb022`, local CI blob `2f6b66aad7594c809bf7247faf9db6dac2b843b6` and provider SPI tree `c335b07d4a32061204c22af28efdafb8d4ffb49d`. The evidence revision preserves the executable artifacts.
