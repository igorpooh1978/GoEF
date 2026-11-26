# SQLite table rebuild interrupted before commit

[Issue #13](https://github.com/igorpooh1978/GoEF/issues/13) · [Gherkin](../../../test/conformance/driverlab/features/sqlite_rebuild.feature) · [Worker](../../../test/conformance/driverlab/sqlite_rebuild_test.go) · [Compatibility runner](../../../test/conformance/lab/sqlite_compat.py)

An owned local child process rebuilds a synthetic parent table inside a native SQLite transaction. It copies the writable columns, preserves the AUTOINCREMENT high-water mark, swaps the table, restores its index and trigger, updates the retained row, and inserts a row with the next identity. The child sees the changed schema, values and trigger result while the original history remains in place. The transaction stays open through the interruption point, with a deadline on individual operations and a parent watchdog on the child process. After all worker assertions pass, it emits a phase marker and exits at code 68 before writing history or committing. The runner requires that exact exit and marker.

The next process verifies the original schema, row bytes, generated value, sequence, foreign-key relationship, dependent row, index, trigger, zero audit amount for the retained row and pre-existing history. It runs the same rebuild and commits the new history in the same transaction. Foreign-key enforcement is restored on the migration connection and rejects an orphan insert with SQLite code 787. A fresh read-only process verifies the rebuilt schema, preserved payload, generated results, new identity, the retained row’s audit amount, one new history entry and the integrity checks. Each case owns its disposable directory, file and marker. The runner binds all Gherkin steps and exactly one example per pinned engine and mode before building binaries.

This is native driver characterization for the GoEF migration executor work planned in [MIG-RECOVERY-001](../../../specs/features/migrations/mig-recovery-001.feature) and [MIG-SQLITE-001](../../../specs/features/migrations/mig-sqlite-001.feature). The fault boundary is an application process exit before commit. Runtime executor recovery, lock competition, conflicting postconditions, fresh process interruption after a dispatched commit, and other providers retain separate acceptance. The complementary two-version WAL fixture covers an acknowledged commit before driver close. [Committed WAL recovery](SQLITE_WAL_RECOVERY.md)

## Upstream and version evidence

The fixture follows SQLite's documented create, copy, drop and rename sequence inside one transaction, and restores foreign-key enforcement outside that transaction. It implements the procedure independently using synthetic SQL. At 15:40 UTC on 2026-11-26, [Context7](https://mcp.context7.com/mcp) advertised server version 4.7.3 and negotiated protocol `2025-06-18`. Its `query-docs` tool returned the generalized rebuild section and rollback-journal commit documentation for `/websites/sqlite_docs`. The response gave topic evidence. The release notes and the binaries' exact module versions, `sqlite_version()` and `sqlite_source_id()` supplied the pinned runtime evidence. [SQLite ALTER TABLE](https://www.sqlite.org/lang_altertable.html#otheralter) · [SQLite atomic commit](https://www.sqlite.org/atomiccommit.html) · [SQLite WAL](https://www.sqlite.org/wal.html) · [Pinned builds and source IDs](LEGACY_COMPATIBILITY.md#sqlite-minimum-runtime)

## Local verification

Tested executable revision `6899cbfd7e3a01acae4b902b86bc2c298e0b63c6` on 2026-11-26 with Go 1.27.4 on Windows/amd64.

| SQLite and driver | DELETE | WAL |
| --- | --- | --- |
| 3.51.3 and modernc.org/sqlite v1.47.0 | PASS | PASS |
| 3.54.1 and modernc.org/sqlite v1.63.0 | PASS | PASS |

- `python -B test/conformance/lab/sqlite_compat.py --model-types --owned-tx --savepoint-tx --save-changes --tx-options --decimal-binding --commit-outcome` exited 0. It reported four interrupted rebuild checks, four compatible file checks, and 14 baseline driver scenarios on the two pinned runtimes.
- `python -B test/conformance/lab/sqlite_wal_recovery.py` exited 0 on `6a6e8b8ed50139d89f0a330af831eec410d6ac65` with both committed-WAL directions passing. The earlier committed recovery and this pre-commit process exit exercise distinct boundaries.
- `python -B -m unittest discover -s test/conformance/lab -p 'test*.py'` exited 0 with 83 tests. The new tests reject wrong or missing worker markers and reject missing, duplicate, unsupported or unbound Gherkin examples before building binaries. `python -B test/bootstrap/verify.py` exited 0 with 84 product scenarios.
- In the driver lab module, `go build -mod=readonly ./...`, `go vet -mod=readonly ./...` and `go test -mod=readonly -shuffle=on -count=1 ./...` exited 0.
- In a separate worktree at the tested revision, a temporary control committed the physical rebuild before the interruption marker while leaving history unchanged. `python -B test/conformance/lab/sqlite_compat.py` exited 1 at the SQLite 3.51.3 DELETE resume stage. A separate mutation introduced a nonfatal Go test assertion failure before the interruption marker. The same command exited 1 at the interruption stage because the expected exit and marker were absent. Both mutations were restored and the control worktree was clean.
- The PowerShell compatibility gate accepted the live SQLite report. A replay through `Invoke-SqliteCompatibility` rejected array-valued scenario, status and engine version fields, an incorrect exit code, profile or journal mode, and a duplicated case.
