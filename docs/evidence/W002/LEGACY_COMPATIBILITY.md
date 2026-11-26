# W002 engine and SQLite file compatibility evidence

[Compatibility issue #31](https://github.com/igorpooh1978/GoEF/issues/31) · [Compatibility matrix](../../parity/ENGINE_COMPATIBILITY.md) · [Initial driver run](DRIVER_GHERKIN.md)

## Artifacts and scenarios

The legacy server profiles use the existing pinned Go drivers and the same three [native driver scenarios](../../../test/conformance/driverlab/features/driver.feature). The Go test module uses Go 1.27.4. The pinned server images are Linux/amd64 and run on the local workstation Docker daemon with disposable storage and no external network or published ports.

| Profile | Engine identity | Driver | Artifact or runtime gate |
| --- | --- | --- | --- |
| `postgres-17` | 17.12, `server_version_num` 170012 | pgx v5.13.0 | `sha256:e711be2c7ff64c9eb8883fc87571621be6caa2f943d825a49e5d20ac5b0221d7` ([release](https://www.postgresql.org/docs/release/17.12/)) |
| `sqlserver-2017` | 14.0.3570.2, database compatibility level 140 | go-mssqldb v1.11.0 | `sha256:5f6fd78f8752390dbd3584b91dee463f1f4b2b30beef9af3fd35753d32940c52` |
| `sqlserver-2019` | 15.0.4510.2, database compatibility level 150 | go-mssqldb v1.11.0 | `sha256:debdc21628cddd1a4433371c601c8dee9bad3533ae8b18fe9dc8c5596368b10d` |
| `sqlite-3.51` | 3.51.3 | modernc.org/sqlite v1.47.0 | Isolated Go module lock and observed SQLite source ID and compile options ([release](https://www.sqlite.org/releaselog/3_51_3.html)) |
| `sqlite-3.54` | 3.54.1 | modernc.org/sqlite v1.63.0 | Main test module lock and observed SQLite source ID and compile options |

On the baseline `7b12932f0158af3dbb599755531ee86ee0ac93a2`, the `postgres-17` selector was rejected with exit 2. The first local PostgreSQL 17.12 characterization in `56e1f12d1d8669688a95469783e92251c9eeae81` exited 0. All three scenarios passed, with native SQLSTATEs `23505` and `23503`, and the server reported UTF8, C collation, UTC, read committed and `public` search path.

## SQL Server startup and consent

The 2017 image contains `sqlcmd` under `/opt/mssql-tools/bin` with ODBC 17. The 2019 image contains it under `/opt/mssql-tools18/bin` with ODBC 18. The exact image layers and local stopped containers confirmed the respective paths. The fixture pins both the engine build and the database compatibility level. Local consent is separate from the SQL Server 2022 flag. The two 2022 and legacy flag substitution checks returned `BLOCKED` before build or provisioning.

The SQL Server 2022 reference cell passed all three scenarios after this change. It reported build 16.0.4325.4, update CU28, database compatibility level 160, and native codes `2627` and `547`.

## SQLite file scope

The [file scenario specification](../../../test/conformance/driverlab/features/sqlite_files.feature) binds `LAB-SQLITE-FILE-001-S01` and `LAB-SQLITE-FILE-001-S02` to the two directions of the [dedicated runner](../../../test/conformance/lab/sqlite_compat.py). One binary creates a synthetic file and the other opens the same file and observes its existing rows. The peer rolls back a failed additive migration, commits a compatible migration, and closes the file. The creator then reopens it and verifies the committed schema, rows, audit records, migration history, integrity and foreign keys. The directions exercise `DELETE` and `WAL` journaling with a schema supported by both versions.

On the SQLite lane `274a96d42d27f7e485130931a32e674d48f017a5`, `python -B test/conformance/lab/sqlite_compat.py` exited 0 on Windows/amd64. The three native driver scenarios passed in both builds. The four file runs passed in both directions with `DELETE` and `WAL`. The fixtures used strict tables, a stored generated column, binary values, related rows, a named index and an audit trigger. The final read-only reopen rejected a write with SQLite code `8`. The isolated lock verified successfully with `GOWORK=off` and `GOFLAGS=-modfile=sqlite351.mod` set for `go mod verify`.

| SQLite runtime | Observed source ID | Compile options |
| --- | --- | --- |
| 3.51.3 | `2026-03-13 10:38:09 737ae4a34738ffa0c3ff7f9bb18df914dd1cad163f28fd6b6e114a344fe6d618` | 55, `47b1f3a94c2d870501bba4ee870f85c153a53bab8ec35eaa9c0e12ad606a8d91` |
| 3.54.1 | `2026-10-16 14:45:20 555e07ef33f69a0957d24d8e555b702464a131d6f1607f4c3c1e4551e8e6ca76` | 57, `aadb3c39a8f490667be3a9eb4c35be3fd81307074659c675ae7d2f0a07701c1a` |

The cases establish compatibility for these schemas and controlled process handoffs. Production migrations, files with features outside the shared schema, crash recovery and older SQLite runtimes require separate evidence. [SQLite file format](https://www.sqlite.org/fileformat.html) · [WAL behavior](https://www.sqlite.org/wal.html)

The follow-up `d99a262e5d6eebde2aea28e08a341c65e3761ed1` reran all six SQLite driver rows and four file rows after binding the steps and examples. The tests reject changed assertions, missing or duplicate journal examples, the wrong version or engine role, and invalid worker markers. The historical CI gate independently rejected using the originating runtime for the middle step.

## Checked source

The driver module tree is `1b5f50d53c1ad81af7d974e53598b316e52d65d9`, the lab tree is `782042056947a5b779aeaacfb9591812e9a89b57`, and the historical CI script blob is `df10e824da0d6a0bb105c6bc4f72b6cb3e51d702`. The primary Go test module also passed `go test -mod=vendor -race -count=1 -shuffle=on ./...` on Go 1.27.4 Linux/amd64 in an isolated local container using the prior verified module graph. This was a unit and parser run. SQLite file and server outcomes were measured separately.

## Integrated local run

On 2026-11-25 the local CI run passed the repository validator, seven bootstrap tests, 32 Python lab tests, Go build, vet, test and module checks, all executable driver cells, and the four SQLite file cells. The report contains 27 successful driver rows covering 24 distinct profile and scenario pairs. The file suite repeats the three current SQLite rows before exchanging files. The two SQL Server legacy cells reported `BLOCKED` pending explicit authorization for those Developer editions. Neither legacy server was started, and the full matrix exited 1. See the [recorded command](https://github.com/igorpooh1978/GoEF/blob/c26e115e2f74029a66428a456e4bb36bfd974c92/docs/evidence/W002/LEGACY_COMPATIBILITY.md#integrated-local-run).

### Complete run on 2026-11-26

The authorized run tested `77f432f03695bdb247289ba25743d3e274a4c81d` on the Windows workstation with Go 1.27.4 and the local Docker Desktop 29.4.0 Linux/amd64 daemon. It exited 0. All ten engine profiles passed 30 distinct driver cases and the four SQLite file cases passed. The driver report contains 33 passing rows because the SQLite file suite also reruns the three current SQLite driver cases. The repository validator, seven bootstrap tests, 32 Python lab tests and Go build, vet, test and module verification passed. No lab-owned containers remained after the run.

| SQL Server Developer profile | Observed build | Database compatibility level | Commit and rollback | Unique code | Foreign-key code |
| --- | --- | --- | --- | --- | --- |
| 2017 | 14.0.3570.2, CU31 | 140 | PASS | 2627 | 547 |
| 2019 | 15.0.4510.2, CU32 | 150 | PASS | 2627 | 547 |

Both cells used go-mssqldb v1.11.0 and observed TCP, `READ COMMITTED`, `XACT_ABORT ON` and `Latin1_General_100_BIN2`. The driver module tree, lab tree and historical CI script blob listed above are identical to those used for this run. Application adapter conformance and SQL Server 2022 with compatibility level 140 retain separate gates.

## PostgreSQL 19 and SQL Server 2022 at level 140

Both profiles run the same three [native driver scenarios](../../../test/conformance/driverlab/features/driver.feature). The PostgreSQL 19 selector was rejected with exit 2 at baseline `9ce89d78437219984cf5669db720625a384818bd`. It passed all three cases with exit 0 on `2f6e261363af8298061ab5a3a53fd0c280897e90` through `python -B test/conformance/lab/driver_run.py postgres --profile postgres-19`. The fixture observed PostgreSQL 19.0, `server_version_num` 190000, UTF8, C collation, UTC, read committed, `public` search path and pgx v5.13.0 in exec mode. Native uniqueness and foreign-key checks reported `23505` and `23503`. [PostgreSQL 19.0 release](https://www.postgresql.org/docs/release/19.0/)

The PostgreSQL Linux/amd64 manifest is `sha256:3fb624f672a1cb628d0bdc0f533cffad31345421eedf16ab1f85ed1ea0d07d2e`. The SQL Server 2022 profiles both use the existing CU28 Developer image, Linux/amd64 manifest `sha256:4525e3efac9f3e241cb003bb2ff2a82818b502f37e12049d788b1a9fa7e13b7f`, with go-mssqldb v1.11.0. Only the database compatibility level differs between the two profiles. [SQL Server database compatibility levels](https://learn.microsoft.com/en-us/sql/t-sql/statements/alter-database-transact-sql-compatibility-level)

The focused SQL Server runs at `ba0ae33142cbb82beb34b0027923d94215e280b1` each exited 0 with three passing cases. Both verified the CU28 build and the setting that controls table-valued function availability. With the override absent or off, `GENERATE_SERIES(2, 6, 2)` returned exactly `2, 4, 6` at level 160 and failed with native error `208` at level 140. Both profiles also observed codes `2627` and `547` for the constraint cases. [GENERATE_SERIES compatibility requirement](https://learn.microsoft.com/en-us/sql/t-sql/functions/generate-series-transact-sql?view=sql-server-ver16)

### SQL Server 2017 certificate regression

The first integrated run on `f5257ab9677a316db66bbe9711db0a87f310cb79` passed the new profiles and returned exit 1 for the existing SQL Server 2017 profile. A separate local run reproduced the failure on all three scenarios during the initial fixture query. Redacted diagnostics identified Go's TLS certificate parser rejecting the negative serial number in the server-generated certificate. The test harness supplies `GODEBUG=x509negativeserial=1` only to the SQL Server 2017 Go test process inside the owned container. The fixture keeps encrypted transport and the existing loopback endpoint guard. [Microsoft driver troubleshooting](https://learn.microsoft.com/en-us/sql/connect/golang/troubleshooting?view=sql-server-ver17) · [Go certificate parser](https://pkg.go.dev/crypto/x509#ParseCertificate)

### Complete expanded run on 2026-11-26

On `17bb049c5e7eb1e5f53da312c6bf1cc10904e244`, the focused SQL Server 2017 run passed all three scenarios. The complete run then exited 0 on the Windows workstation with Go 1.27.4 and local Docker Desktop 29.4.0 Linux/amd64. There were 36 distinct driver cases across 12 profiles and four SQLite file cases. All passed. The report contains 39 driver rows because the SQLite file suite reruns the three current SQLite driver cases.

The repository validator, seven bootstrap tests, 34 Python lab tests and Go build, vet, test and module checks passed. The primary driver's unit tests also passed separately with `go test -mod=vendor -race -count=1 -shuffle=on ./...` on the same Go test tree in a local Linux/amd64 container. No lab-owned containers remained after the complete run.

The exact driver module tree is `34d63265c8440e604d56bc1976a8dca5455e2fe3`, the lab tree is `bd6de92664eb31d5b42cca8881632674414864f1`, and the historical CI script blob is `de4baac6e9b206bb3fb5ca95aab0a985fd56f341`. The published evidence uses the same executable artifacts.

## SQLite minimum runtime

W002 selects **SQLite 3.51.3** as the minimum for the pinned pure-Go builds. The accepted minimum profile is `sqlite-3.51` with `modernc.org/sqlite` v1.47.0, and the reference is `sqlite-3.54` with v1.63.0. The minimum keeps the existing locked build on Windows/amd64. Each tested version is identified by its module version, runtime source ID and compile options. [Tagged driver platform table](https://raw.githubusercontent.com/modernc-org/sqlite/v1.47.0/doc.go)

Earlier releases can parse generated columns and `STRICT` tables. SQLite 3.51.0 added enforcement of the declared type of computed columns in `STRICT` tables. SQLite 3.51.3 is the first 3.51 patch with the WAL-reset corruption fix. SQLite also backported the WAL fix to the 3.44 and 3.50 branches. The older backports are outside the selected Go-driver builds and file contract. [SQLite 3.51.3 release](https://www.sqlite.org/releaselog/3_51_3.html) · [WAL reset fix and backports](https://www.sqlite.org/wal.html#the_wal_reset_bug) · [Tagged driver change history](https://raw.githubusercontent.com/modernc-org/sqlite/v1.47.0/CHANGELOG.md)

The [file scenarios](../../../test/conformance/driverlab/features/sqlite_files.feature) require an inserted row with an invalid generated value to fail with native `SQLITE_CONSTRAINT_DATATYPE` (`3091`). They check that the row and the rolled-back migration leave the persisted schema, values, audit and migration history intact. The accepted additive change is visible when the originating runtime reopens the same file. Library-level recovery, rebuilds and files using higher-version schema features retain separate acceptance gates. [SQLite's tagged computed-column tests](https://github.com/sqlite/sqlite/blob/version-3.51.3/test/strict1.test) · [Native result codes](https://www.sqlite.org/rescode.html#constraint_datatype)

On 2026-11-26 UTC, a standalone Context7 4.7.3 documentation query used protocol `2025-06-18`, tool `query-docs` and selector `/websites/sqlite_docs`. The response corroborated the exact 3.51.3 source ID and linked the vendor's 3.51.0 and 3.51.3 notes and WAL history. The exact SQLite and driver releases above anchor the version decision.

### Minimum-profile local run

The focused file run used `python -B test/conformance/lab/sqlite_compat.py`. The arithmetic-only fixture could produce only integers. With the new datatype assertion, that fixture failed at the first file mutation with exit 1. After the fixture deliberately generated a noncoercible value, the command exited 0 on implementation revision `50ebfcc468a451254be97e3016a56f151f0e79ef` with six passing driver rows and four passing file cases.

The integrated local CI run then exited 0 on integrated revision `07038da670a3e26871d0312d792f92d68ab72ede` on 2026-11-26. The Windows/amd64 workstation used Go 1.27.4 and Python 3.14.4. Server cases used the workstation's Docker Desktop 29.4.0 Linux/amd64 daemon.

| File sequence | DELETE journal | WAL journal |
| --- | --- | --- |
| 3.51.3 creates → 3.54.1 mutates → 3.51.3 reopens | PASS, code `3091` | PASS, code `3091` |
| 3.54.1 creates → 3.51.3 mutates → 3.54.1 reopens | PASS, code `3091` | PASS, code `3091` |

The report identified `sqlite-3.51` as the minimum, observed the same source IDs and compile options listed above, and recorded 36 distinct driver cases across 12 profiles and four file cases. The 39 reported driver rows include the three repeated current SQLite cases. The repository validator, seven bootstrap tests, 35 Python lab tests and Go build, vet, test and module checks passed. The primary driver's unit tests also passed with `go test -mod=vendor -race -count=1 -shuffle=on ./...` in a local Linux/amd64 container. The lab removed its containers after the run.

The exact driver module tree is `951054e5683db030a5e20e31cc399cff79ad0fce`, lab tree `0806db6023667fca2090f55495d971ad8b3b1a85` and historical CI script blob `38b02e2f725808f251cffeab274d68b1db9a0125`. The evidence commit preserves these executable artifacts.

## Repeat the compatibility matrix

Use the current Python runner from the repository root. Both consent flags require authorization for the selected SQL Server Developer editions. The [historical command records](https://github.com/igorpooh1978/GoEF/blob/c26e115e2f74029a66428a456e4bb36bfd974c92/docs/evidence/W002/LEGACY_COMPATIBILITY.md#integrated-local-run) preserve the original invocations and tool versions.

```sh
python -B scripts/ci/goef_w002.py --compatibility --accept-sqlserver-eula --accept-legacy-sqlserver-eula
```

### Python runner verification

On 2026-11-26, the Python runner (Git blob `7336776be215668c13f8e7090e5f522027fc3774`) exited 0 on Windows/amd64 with Python 3.14.4, Go 1.27.4, and Docker 29.4.0 on Linux/amd64.

```sh
python -B scripts/ci/goef_w002.py --providers postgres sqlite mysql --compatibility --production-adapters --model-types --owned-tx --savepoint-tx --save-changes --tx-options --decimal-binding --commit-outcome
```

The run covered PostgreSQL 16.16, 17.12, 18.7 and 19.0, SQLite 3.51.3 and 3.54.1, and MySQL 8.0.46 and 8.4.13.1. It passed 102 Python unit tests, Go module checks, and the selected live scenarios, including SQLite file compatibility and interrupted rebuild. A separate SQL Server Developer selection without consent reported all four SQL Server profiles as blocked before provisioning. The Windows LocalDB quickstart and its typed-value, uniqueness and cancellation probes passed separately with the published adapter.

## Issue #31 acceptance

The detached source revision `dc13ff64370f7e7348f0f7958dbf996819177a90` was verified on 2026-11-26 with Go 1.27.4 and Python 3.14.4 on Windows/amd64. The server scenarios used the local Docker 29.4.0 Linux/amd64 daemon.

| Evidence | Profiles | Results |
| --- | --- | --- |
| `python -B test/conformance/lab/driver_run.py postgres --profile <profile>` | `postgres-16`, `postgres-17`, `postgres-18`, `postgres-19` | Each invocation exited 0. All 28 driver scenario rows passed. |
| `python -B test/conformance/lab/driver_run.py mysql --profile <profile>` | `mysql-8.0`, `mysql-8.4` | Each invocation exited 0. All 14 driver scenario rows passed. |
| `python -B test/conformance/lab/sqlite_compat.py` | `sqlite-3.51`, `sqlite-3.54` | Exit 0. All 14 driver rows and four clean-close file cases passed. All file cases observed native code `3091`. |
| `python -B test/conformance/lab/sqlite_wal_recovery.py` | `sqlite-3.51` → `sqlite-3.54` and the reverse | Exit 0. Both peer runtimes recovered and checkpointed the committed WAL after the originating process exited. The originating runtime then reopened the checkpointed file in each direction. [Recovery evidence](SQLITE_WAL_RECOVERY.md) |
| [Published authorized matrix run](../W003/DECIMAL_SQL_BINDING.md#real-engine-catalog-and-value-checks) | `sqlserver-2017`, `sqlserver-2019`, `sqlserver-2022-140`, `sqlserver-2022` | All 28 SQL Server driver scenario rows passed at `acd3c5a6250b80dcd716630291174c4dfb93dcce`. |

`git diff --name-status acd3c5a dc13ff6` lists only two Markdown files. Both revisions have the same driver module tree (`74c7f0445a8a6d0a01b627438a9e36d884f9c513`), lab tree (`2a6c7254c6837aa241b7527a097b194e71b7af58`) and historical local CI script blob (`7214dda39ac437513827123f15bde41031d1ea04`). The prior run therefore exercised the same executable source and pins. The current review used the recorded SQL Server build, compatibility-level and feature-gate evidence. A fresh SQL Server invocation requires separate local consent.

`python -B test/bootstrap/verify.py` exited 0 and validated 76 Gherkin scenarios. `python -B -m unittest discover -s test/conformance/lab -p 'test*.py'` exited 0 with 78 tests.

The evidence covers the 12 selected driver compatibility profiles, the four SQLite clean-close cases, and the two committed WAL recovery directions. [Issue #13](https://github.com/igorpooh1978/GoEF/issues/13) tracks GoEF adapter and application conformance. MySQL 5.7 remains a candidate with a separate compatibility scope.
