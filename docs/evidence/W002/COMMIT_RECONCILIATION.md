# Verify an ambiguous owned transaction

[Issue #13](https://github.com/igorpooh1978/GoEF/issues/13) · [Gherkin](../../../specs/features/transactions/tx-outcome-001.feature) · [Executable fixture](../../../test/conformance/driverlab/commit_outcome_test.go) · [User guide](../../user/TRANSACTIONS.md#verify-an-unknown-owned-commit)

The owned transaction discards its physical connection before reporting an ambiguous commit. An application verifier can resolve that outcome on a fresh connection to the authoritative primary. Verified commit allows one explicit acceptance of valid saved changes. Verified rollback preserves the original snapshot and pending edits. Verification failure or inconclusive evidence keeps the receipt frozen. Neither verification nor acceptance sends another commit or replays writes.

The fixture writes an operation marker and an aggregate using the same GoEF transaction. It stages a snapshot with the persisted ID and amount. The test-only connector hides the result after successful native commit or successful rollback before commit dispatch. A separate native connection checks the marker and payload. The rollback case first checks that absence alone keeps the outcome unknown. It then repeats on a fresh primary connection with proof that the fixture's prior rollback succeeded. The unavailable case attempts to acquire a connection from a closed observer pool and verifies that the receipt stays unapplied. An additional independent primary read confirms durable state in both fault paths.

Pinned EF Core 10.0.15 verifies possible success after commit failure before retry classification and defers state acceptance until completion. GoEF uses explicit reconciliation and explicit receipt acceptance without automatic replay. The implementation and tests are original and reference EF Core under its MIT license. [Pinned implementation](https://github.com/dotnet/efcore/blob/b34c0e9d7e768aa808ded44f19e7f871bea6869f/src/EFCore/Storage/ExecutionStrategyExtensions.cs#L790-L813) · [Pinned commit fault tests](https://github.com/dotnet/efcore/blob/b34c0e9d7e768aa808ded44f19e7f871bea6869f/test/EFCore.SqlServer.FunctionalTests/ExecutionStrategyTest.cs#L114-L232) · [Source lock](../../upstream/lock.json)

## Focused runs

Baseline **ba3d033a155de704f8dfa0e442710979b36f9860**. Executable source **e1aef3f7be31998675fa86beebb3c9154fef8c9e**. The direct lab runner used the same source with a local adjustment to require the five result IDs. The integrated matrix must exercise its checked-in result validation. The fixture uses synthetic operation ID 7 and amount 31 on disposable targets.

| Profile | Engine and driver | S01–S05 |
| --- | --- | --- |
| **postgres-18** | PostgreSQL 18.7, pgx v5.13.0 | PASS |
| **mysql-8.4** | MySQL 8.4.13.1 InnoDB, go-sql-driver/mysql v1.10.0 | PASS |
| **sqlite-3.51** | SQLite 3.51.3 WAL, modernc.org/sqlite v1.47.0 | PASS |
| **sqlite-3.54** | SQLite 3.54.1 WAL, modernc.org/sqlite v1.63.0 | PASS |
| **sqlserver-2022** | SQL Server 2022 CU28, 16.0.4325.4, compatibility level 160, go-mssqldb v1.11.0 | PASS |

On 2026-11-26, Go 1.27.4 Windows/amd64 and disposable Linux/amd64 database containers:

- **python -B test/conformance/lab/driver_run.py postgres mysql sqlite sqlserver --accept-sqlserver-eula --commit-outcome** exited 0. It reported 20 distinct outcome results and passed seven existing driver cases per profile.
- **python -B test/conformance/lab/sqlite_compat.py --commit-outcome** exited 0. It reported ten outcome results across the two linked runtimes, including the same five cases on SQLite 3.54.1, plus four file compatibility and four native rebuild interruption checks.
- Root **go build -mod=readonly ./...**, **go vet -mod=readonly ./...**, **go test -mod=readonly -shuffle=on -count=1 -coverprofile <file> ./...** and **govulncheck ./...** exited 0. Root statement coverage was 97.4%, transaction coverage was 95.8%, and **ReconcileCommit** had 100% statement coverage.
- Root **go test -mod=readonly -race -shuffle=on -count=1 ./...** exited 0 on Go 1.27.4 Linux/amd64 with cgo. A concurrent close returned while verification was still blocked, and overlapping verification returned **ErrContextBusy**.
- The default driver lab **go mod verify**, **go vet -mod=readonly ./...** and **go test -mod=readonly -shuffle=on -count=1 ./...** exited 0 before the follow-up for concurrent close.
- The behavioral negative control temporarily removed the transition to committed. **go test -mod=readonly -count=1 -run '^TestReconcileCommitConfirmsOneOutcome/committed$' ./transaction** then exited 1 and observed the transaction still in the unknown state. The source was restored before commit.

The fixture proves caller-prepared snapshot reconciliation through the transaction and receipt APIs. Model-driven change detection, identity-map and graph updates, wire-level disconnects, legacy profile evidence for this change, and full production-adapter scenarios retain their own acceptance.
