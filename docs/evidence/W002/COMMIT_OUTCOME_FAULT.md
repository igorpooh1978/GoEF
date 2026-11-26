# Owned transaction commit outcome fault

[W002 issue #13](https://github.com/igorpooh1978/GoEF/issues/13) · [Transaction contract issue #2](https://github.com/igorpooh1978/GoEF/issues/2) · [Gherkin](../../../specs/features/transactions/tx-outcome-001.feature) · [Executable fixture](../../../test/conformance/driverlab/commit_outcome_test.go)

The two fixture scenarios write a unique operation marker and an aggregate in a real GoEF-owned transaction. The test-only connector returns the same synthetic transport failure after a successful native commit (S03) or a successful native rollback before commit dispatch (S04). It verifies the exact native operation count before checking durable state. GoEF produces `GOEF-TX-0001`, keeps the staged value unaccepted, rejects further completion, discards the affected checkout and opens a healthy replacement. The independent primary connection rechecks the isolated target and finds one matching marker and aggregate after S03 and zero rows after S04. The batch callback runs once in each case.

The fixture follows the pinned EF Core example of distinguishing native commit failure from loss of a successful acknowledgement using a verifier. GoEF exposes the uncertain state and the fixture observes durable state independently. The fault is injected after the real driver returns or after a known pre-dispatch rollback. Native transport error mapping, wire disconnects, automatic tracker reconciliation and an unavailable authoritative verifier retain separate acceptance. [Pinned EF Core strategy](https://github.com/dotnet/efcore/blob/b34c0e9d7e768aa808ded44f19e7f871bea6869f/src/EFCore/Storage/ExecutionStrategyExtensions.cs#L773-L792) · [Pinned EF Core test](https://github.com/dotnet/efcore/blob/b34c0e9d7e768aa808ded44f19e7f871bea6869f/test/EFCore.SqlServer.FunctionalTests/ExecutionStrategyTest.cs#L114-L228) · [MIT provenance](../../upstream/lock.json)

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

The feature's S01 and S02 require automatic tracker reconciliation and remain open. Migration interruption and full application/production adapter conformance retain their own #13 gates.
