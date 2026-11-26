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

The follow-up `d99a262e5d6eebde2aea28e08a341c65e3761ed1` reran all six SQLite driver rows and four file rows after binding the steps and examples. The tests reject changed assertions, missing or duplicate journal examples, the wrong version or engine role, and invalid worker markers. The PowerShell gate independently rejects using the originating runtime for the middle step.

## Checked source

The driver module tree is `1b5f50d53c1ad81af7d974e53598b316e52d65d9`, the lab tree is `782042056947a5b779aeaacfb9591812e9a89b57`, and the PowerShell script blob is `df10e824da0d6a0bb105c6bc4f72b6cb3e51d702`. The primary Go test module also passed `go test -mod=vendor -race -count=1 -shuffle=on ./...` on Go 1.27.4 Linux/amd64 in an isolated local container using the prior verified module graph. This was a unit and parser run. SQLite file and server outcomes were measured separately.

## Integrated local run

On 2026-11-25 the command below passed the repository validator, seven bootstrap tests, 32 Python lab tests, Go build, vet, test and module checks, all executable driver cells, and the four SQLite file cells. The report contains 27 successful driver rows covering 24 distinct profile and scenario pairs. The file suite repeats the three current SQLite rows before exchanging files. The two SQL Server legacy cells reported `BLOCKED` pending explicit authorization for those Developer editions. Neither legacy server was started, and the full matrix exited 1.

```powershell
pwsh -NoProfile -File scripts/ci/goef-w002.ps1 -Compatibility -AcceptSqlServerEula
```

### Complete run on 2026-11-26

The authorized run below tested `77f432f03695bdb247289ba25743d3e274a4c81d` on the Windows workstation with Go 1.27.4 and the local Docker Desktop 29.4.0 Linux/amd64 daemon. It exited 0. All ten engine profiles passed 30 distinct driver cases and the four SQLite file cases passed. The driver report contains 33 passing rows because the SQLite file suite also reruns the three current SQLite driver cases. The repository validator, seven bootstrap tests, 32 Python lab tests and Go build, vet, test and module verification passed. No lab-owned containers remained after the run.

```powershell
pwsh -NoProfile -File scripts/ci/goef-w002.ps1 -Compatibility -AcceptSqlServerEula -AcceptLegacySqlServerEula
```

| SQL Server Developer profile | Observed build | Database compatibility level | Commit and rollback | Unique code | Foreign-key code |
| --- | --- | --- | --- | --- | --- |
| 2017 | 14.0.3570.2, CU31 | 140 | PASS | 2627 | 547 |
| 2019 | 15.0.4510.2, CU32 | 150 | PASS | 2627 | 547 |

Both cells used go-mssqldb v1.11.0 and observed TCP, `READ COMMITTED`, `XACT_ABORT ON` and `Latin1_General_100_BIN2`. The driver module tree, lab tree and PowerShell blob listed above are identical to those used for this run. Application adapter conformance and SQL Server 2022 with compatibility level 140 retain separate gates.

## PostgreSQL 19 and SQL Server 2022 at level 140

Both profiles run the same three [native driver scenarios](../../../test/conformance/driverlab/features/driver.feature). The PostgreSQL 19 selector was rejected with exit 2 at baseline `9ce89d78437219984cf5669db720625a384818bd`. It passed all three cases with exit 0 on `2f6e261363af8298061ab5a3a53fd0c280897e90` through `python -B test/conformance/lab/driver_run.py postgres --profile postgres-19`. The fixture observed PostgreSQL 19.0, `server_version_num` 190000, UTF8, C collation, UTC, read committed, `public` search path and pgx v5.13.0 in exec mode. Native uniqueness and foreign-key checks reported `23505` and `23503`. [PostgreSQL 19.0 release](https://www.postgresql.org/docs/release/19.0/)

The PostgreSQL Linux/amd64 manifest is `sha256:3fb624f672a1cb628d0bdc0f533cffad31345421eedf16ab1f85ed1ea0d07d2e`. The SQL Server 2022 profiles both use the existing CU28 Developer image, Linux/amd64 manifest `sha256:4525e3efac9f3e241cb003bb2ff2a82818b502f37e12049d788b1a9fa7e13b7f`, with go-mssqldb v1.11.0. Only the database compatibility level differs between the two profiles. [SQL Server database compatibility levels](https://learn.microsoft.com/en-us/sql/t-sql/statements/alter-database-transact-sql-compatibility-level)
