# Owned transaction commit outcome fault

[W002 issue #13](https://github.com/igorpooh1978/GoEF/issues/13) · [Transaction contract issue #2](https://github.com/igorpooh1978/GoEF/issues/2) · [Gherkin](../../../specs/features/transactions/tx-outcome-001.feature) · [Executable fixture](../../../test/conformance/driverlab/commit_outcome_test.go)

The two fixture scenarios write a unique operation marker and an aggregate in a real GoEF-owned transaction. The test-only connector returns the same synthetic transport failure after a successful native commit (S03) or a successful native rollback before commit dispatch (S04). It verifies the exact native operation count before checking durable state. GoEF produces `GOEF-TX-0001`, keeps the staged value unaccepted, rejects further completion, discards the affected checkout and opens a healthy replacement. The independent primary connection rechecks the isolated target and finds one matching marker and aggregate after S03 and zero rows after S04. The batch callback runs once in each case.

The fixture follows the pinned EF Core example of distinguishing native commit failure from loss of a successful acknowledgement using a verifier. GoEF exposes the uncertain state and the fixture observes durable state independently. The fault is injected after the real driver returns or after a known pre-dispatch rollback. Native transport error mapping, wire disconnects, automatic tracker reconciliation and an unavailable authoritative verifier retain separate acceptance. [Pinned EF Core strategy](https://github.com/dotnet/efcore/blob/b34c0e9d7e768aa808ded44f19e7f871bea6869f/src/EFCore/Storage/ExecutionStrategyExtensions.cs#L790-L813) · [Pinned EF Core test](https://github.com/dotnet/efcore/blob/b34c0e9d7e768aa808ded44f19e7f871bea6869f/test/EFCore.SqlServer.FunctionalTests/ExecutionStrategyTest.cs#L114-L228) · [MIT provenance](../../upstream/lock.json)

## Evidence

The baseline is `dc13ff64370f7e7348f0f7958dbf996819177a90`. The fixture revision is `4bb07e2a1a0986be771669e6d3589beddc13f754`. Verification ran on 2026-11-26 with Go 1.27.4 on Windows/amd64 and isolated Linux/amd64 server containers.

| Profile | Engine and Go driver | S03 | S04 |
| --- | --- | --- | --- |
| `postgres-18` | PostgreSQL 18.7, pgx v5.13.0 | PASS | PASS |
| `mysql-8.4` | MySQL 8.4.13.1 InnoDB, go-sql-driver/mysql v1.10.0 | PASS | PASS |
| `sqlite-3.54` | SQLite 3.54.1 WAL, modernc.org/sqlite v1.63.0 | PASS | PASS |
| `sqlite-3.51` | SQLite 3.51.3 WAL, modernc.org/sqlite v1.47.0 | PASS | PASS |
| `sqlserver-2022` | Required SQL Server 2022 CU28 and go-mssqldb v1.11.0 | BLOCKED | BLOCKED |

- `python -B test/conformance/lab/driver_run.py postgres mysql sqlite --commit-outcome` exited 0. The three rows reported both completion cases and passed the seven existing driver scenarios for each profile.
- `python -B test/conformance/lab/sqlite_compat.py --commit-outcome` exited 0. Both linked runtimes reported both completion cases, passed their existing driver scenarios and passed the four file exchange cases.
- `python -B test/conformance/lab/driver_run.py sqlserver --commit-outcome` exited 1 with the expected EULA block before provisioning. The driver completion fixture needs its own SQL Server run. Other optional compatibility profiles were not selected.
- `python -B test/bootstrap/verify.py` exited 0 with 78 scenario specifications. `python -B -m unittest discover -s test/conformance/lab -p 'test_*.py'` exited 0 with 79 tests. The opt-in runner rejects an omitted, duplicate or wrong-version completion report.
- `go build -mod=readonly ./...`, `go vet -mod=readonly ./...`, and `go test -mod=readonly -shuffle=on -count=1 ./...` exited 0 in the root module. In the driver module, `go mod verify`, `go vet -mod=readonly ./...` and `go test -mod=readonly -shuffle=on -count=1 ./...` exited 0. The race command `go test -race -shuffle=on -count=1 ./...` exited 2 because the host Go environment has CGO disabled.
- The SQLite negative control temporarily changed the production unknown-commit path to retain the affected connection. With `GOEF_COMMIT_OUTCOME=1`, `GOEF_DRIVERLAB_PROVIDER=sqlite` and `GOEF_DRIVERLAB_PROFILE=sqlite-3.54`, `go test -count=1 -run '^TestCommitOutcome$' -v` in the driver module exited 1 and both cases identified the unreleased physical connection. The mutation was restored before the candidate commit.

This run covers S03 and S04. A subsequent run covers explicit receipt reconciliation for S01, S02 and S05. Migration interruption and full application and production adapter conformance retain their own #13 gates. [Commit reconciliation](COMMIT_RECONCILIATION.md)

## Integrated verification

The authorized workstation matrix passed at source commit `32a79ef3a39a6ea6c33f32389264d1b9e472f8e2` with Go 1.27.4 on Windows/amd64 and disposable Linux/amd64 server containers. All 24 distinct commit-outcome results passed. The four SQL Server profiles were 2017 at level 140, 2019 at level 150, and 2022 at levels 140 and 160. The other eight profiles covered PostgreSQL 16–19, MySQL 8.0.46 and 8.4.13.1, and SQLite 3.51.3 and 3.54.1. [Exact engine and driver profiles](LEGACY_COMPATIBILITY.md)

```powershell
pwsh -NoProfile -File scripts/ci/goef-w002.ps1 -Compatibility -ModelTypes -OwnedTx -SavepointTx -SaveChanges -TxOptions -DecimalBinding -CommitOutcome -AcceptSqlServerEula -AcceptLegacySqlServerEula
```

The command exited 0 with 228 distinct scenario and profile results and four SQLite file compatibility checks. SQLite 3.54.1 ran twice, producing 247 reported scenario rows. The bootstrap validator covered 84 specifications, seven validator tests and 79 lab tests passed, and the local suite verified the driver module and ran build, vet and shuffled Go tests. The root and driver lab race commands `go test -mod=readonly -race -shuffle=on -count=1 ./...` exited 0 on Go 1.27.4 Linux/amd64 in an isolated container with cgo. Real-engine cases ran separately from race detection.

Independent review verified that the fixture observes both durable outcomes and rules out an automatic replay and reuse of the affected connection. The focused `go test -mod=readonly -count=1 -run '^TestCommitFailureReportsUnknownAndDiscards$' ./transaction` exits 0 with the additional public message check. A temporary negative control flattened the synthetic private error into the public message without exposing its identity in the error chain. The same test exited 1 on the redaction assertion. The temporary change was restored. [Redaction test](../../../transaction/transaction_test.go)
