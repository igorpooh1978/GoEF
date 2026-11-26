# W002 local driver Gherkin evidence

[W002 #13](https://github.com/igorpooh1978/GoEF/issues/13) · [Gherkin binding #29](https://github.com/igorpooh1978/GoEF/issues/29) · [Local CI #26](https://github.com/igorpooh1978/GoEF/issues/26) · [Version profiles #31](https://github.com/igorpooh1978/GoEF/issues/31)

This report preserves the initial six-profile run. See the [legacy compatibility evidence](LEGACY_COMPATIBILITY.md) for the expanded local matrix and SQLite file checks.

The first driver lab is an isolated Go module under [test/conformance/driverlab](../../../test/conformance/driverlab/README.md). Its direct SDK pins and Go checksum database identities match the [W002 artifact research](TEST_LAB_DESIGN.md). Go 1.27.4 downloaded and verified the test module graph. Core's module remains driver-free.

| Driver | Direct module pin |
| --- | --- |
| PostgreSQL | github.com/jackc/pgx/v5 v5.13.0 |
| SQLite | modernc.org/sqlite v1.63.0 |
| SQL Server | github.com/microsoft/go-mssqldb v1.11.0 |
| MySQL | github.com/go-sql-driver/mysql v1.10.0 |

## Execution scope

The [embedded Gherkin feature](../../../test/conformance/driverlab/features/driver.feature) has three scenario outlines. Its strict step binding checks committed and rolled-back data, the actual native unique and foreign-key error codes and unchanged rows from a distinct observer connection. Results are emitted after fixture cleanup. The overview and reports retain these driver observations separately from GoEF application parity and stable error kinds.

## Real-engine profiles

The initial local driver runner exited 0 on all four reference profiles with 12 passing scenario rows on 2026-11-25. The server fixtures ran on the workstation Docker Linux/amd64 daemon with no external network or published ports. SQLite used the embedded Go driver on Windows/amd64, with WAL and foreign keys. SQL Server used an explicitly authorized local Developer edition run.

| Profile | Engine | Commit and rollback | Unique code | Foreign-key code |
| --- | --- | --- | --- | --- |
| `postgres-18` | PostgreSQL 18.7 | PASS | 23505 | 23503 |
| `sqlite-3.54` | SQLite 3.54.1 | PASS | 2067 | 787 |
| `sqlserver-2022` | SQL Server 2022 CU28, 16.0.4325.4, level 160 | PASS | 2627 | 547 |
| `mysql-8.4` | MySQL Community 8.4.13.1 | PASS | 1062/23000 | 1452/23000 |
| `postgres-16` | PostgreSQL 16.16 | PASS | 23505 | 23503 |
| `mysql-8.0` | MySQL Community 8.0.46 | PASS | 1062/23000 | 1452/23000 |

The additional PostgreSQL 16.16 and MySQL 8.0.46 runs exited 0 for six more rows. Their Linux/amd64 manifests are `sha256:e8a7960781a4aeb21928aa4565124f0e469bd27e0bcc4321d8b9102145312f84` and `sha256:62fb722c78b24245ddff1796a0fcee4a49cc5b87e0aaaf20c92d1da9e0a2497b` respectively. PostgreSQL observed server version number 160016 and SCRAM over local TCP. MySQL observed InnoDB, READ COMMITTED, UTC and utf8mb4_bin. SQLite observed 57 compile options with SHA-256 `aadb3c39a8f490667be3a9eb4c35be3fd81307074659c675ae7d2f0a07701c1a` and source ID `2026-10-16 14:45:20 555e07ef33f69a0957d24d8e555b702464a131d6f1607f4c3c1e4551e8e6ca76`. SQL Server observed explicit compatibility level 160 with separate engine build validation.

The first MySQL 8.0.46 attempt failed during readiness. The image's client reported an unknown `--no-login-paths` option. Provisioning now selects that option for 8.4 only and gives the isolated client an empty login-file path. The three 8.0 scenarios then passed. A local CI regression rejects substituted baseline profile IDs even if all three scenarios claim success. Consult the [engine compatibility matrix](../../parity/ENGINE_COMPATIBILITY.md) for pending SQL Server 14.x and older SQLite cells and feature requirements.

## Checked source and negative cases

The Go test module tree is `1c5c69aa84d61f99a7dc3fefb4e117d13a0b2755`. The local CI script and driver runner blobs are `94e4d51c418d917dc11cf289c128f2bdde6bb064` and `ab2cf3e248b57cfbb3a755fcac0ff4f0cdc849de`. The independent review accepted the complete baseline result and observed rejection of PostgreSQL 16 substituted for 18, MySQL 8.0 substituted for 8.4, SQL Server 2017 substituted for 2022, and a missing SQLite profile. It also observed the marker being omitted from all three result rows.

The Go test module's parser and unit suite passed `go test -mod=vendor -race -count=1 -shuffle=on ./...` on Go 1.27.4 Linux/amd64 using the locked graph in a disposable vendor copy. The real-engine results above were collected separately. This module contains test code only and adds zero owned production statements to the GoEF coverage gate.

## Reproduce

The historical local CI run on 2026-11-25 used the six-profile compatibility selection and SQL Server 2022 consent on the Windows workstation. It exited 0. It passed the repository validator, 7 bootstrap tests, 27 lab tests, the Go build, vet, test and tidy checks, the test module checksum and driver pin checks, and 18 scenario rows across the six real profiles. [Recorded command](https://github.com/igorpooh1978/GoEF/blob/c26e115e2f74029a66428a456e4bb36bfd974c92/docs/evidence/W002/DRIVER_GHERKIN.md#reproduce) · [Current Python runner](../../../test/conformance/driverlab/README.md)

A prior run reported a SQL Server timeout during fixture setup. The runner now strips debug sections from the temporary test binaries and reports copying and permission setup as separate phases. The successful run reached and passed all three SQL Server scenarios.

Output records each profile, its exact runtime identity, scenario status and safe native codes. A wrong target, skipped or missing scenario, or incorrect code fails the run. Local CI uses the workstation toolchain and owned disposable containers. GitHub Actions, a hosted runner and a separately deployed runner are outside this execution path.

The future W002 work in [#13](https://github.com/igorpooh1978/GoEF/issues/13) retains GoEF adapter integration, full ownership and fault coverage, and Gherkin scenario bindings for library behavior.
