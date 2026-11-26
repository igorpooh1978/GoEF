# SQLite WAL recovery across the supported file versions

This [#31](https://github.com/igorpooh1978/GoEF/issues/31) test extends the clean close and reopen cases with a committed write still present in WAL after the originating process exits. Each direction uses a separate disposable directory, a fixed file name and an ownership token. The schema uses the same shared features as the existing file suite. [Clean-close compatibility coverage](LEGACY_COMPATIBILITY.md#sqlite-minimum-runtime) · [WAL recovery scenarios](../../../test/conformance/driverlab/features/sqlite_wal_recovery.feature)

SQLite commits can remain in the WAL until checkpointing transfers them into the main file. The fixture seeds and cleanly closes the database first. A new process on the same engine disables automatic checkpointing on its writer connection, sets `synchronous=FULL`, commits an additive migration, writes a fixed completion marker and exits at code `67` without closing its SQLite handles. The runner requires a main-file digest identical to the seed and a WAL sidecar larger than its 32-byte header. [SQLite WAL](https://www.sqlite.org/wal.html) · [File format](https://www.sqlite.org/fileformat.html#the_write_ahead_log)

The other runtime opens the same database and WAL together and reads the migrated schema, exact rows, generated values, audit entries, and migration history before checkpointing. It confirms integrity and foreign-key checks and requires `PRAGMA wal_checkpoint(TRUNCATE)` to return three zero counters. After it closes, the launcher requires the main file to have changed and the WAL to contain no frames. The original runtime then opens the checkpointed file read-only and verifies the same state. This reproduces an application process exit after acknowledged commit on a local filesystem. [Checkpoint PRAGMA](https://www.sqlite.org/pragma.html#pragma_wal_checkpoint) · [Worker](../../../test/conformance/driverlab/sqlite_wal_recovery_test.go)

On 2026-11-26 at 14:36 UTC, the approved public [Context7 endpoint](https://mcp.context7.com/mcp) (server 4.7.3, protocol `2025-06-18`) returned the SQLite implementation description of `SQLITE_CHECKPOINT_TRUNCATE` from `query-docs` with `/websites/sqlite_docs`. The result corroborates truncation to zero bytes on success. The release identities were verified independently through the release notes, compiled driver module metadata and SQLite version and source-ID queries in each stage. [Context7 source artifact](https://www.sqlite.org/src/artifact/648fdc66f52433fc) · [SQLite 3.51.3](https://www.sqlite.org/releaselog/3_51_3.html) · [SQLite 3.54.1](https://www.sqlite.org/releaselog/3_54_1.html) · [Driver v1.47.0](https://pkg.go.dev/modernc.org/sqlite@v1.47.0) · [Driver v1.63.0](https://pkg.go.dev/modernc.org/sqlite@v1.63.0)

## Local verification

Tested source revision `39ed1ba62385b580b578874f158a640cc23a8837` on Windows/amd64 with Go 1.27.4 on 2026-11-26.

| Scenario | Writer | Checkpointing runtime | Result |
| --- | --- | --- | --- |
| `LAB-SQLITE-WAL-001-S01` | 3.51.3 with modernc.org/sqlite v1.47.0 | 3.54.1 with modernc.org/sqlite v1.63.0 | PASS |
| `LAB-SQLITE-WAL-001-S02` | 3.54.1 with modernc.org/sqlite v1.63.0 | 3.51.3 with modernc.org/sqlite v1.47.0 | PASS |

- `python -B test/conformance/lab/sqlite_wal_recovery.py` exited 1 before the worker ran any stages. A controlled mutation enabling automatic checkpoints after one frame also exited 1 after the writer committed, because the main file no longer matched the clean seed. The restored worker exited 0 on repeated local runs in both directions. [Launcher](../../../test/conformance/lab/sqlite_wal_recovery.py)
- `python -B -m unittest discover -s test/conformance/lab -p 'test*.py'` ran 70 tests with exit 0, including negative checks for early checkpoints, missing WAL frames, invalid completion markers and an unchanged main file after checkpointing. [Launcher tests](../../../test/conformance/lab/test_sqlite_wal_recovery.py)
- `go -C test/conformance/driverlab vet -mod=readonly .`, `go -C test/conformance/driverlab test -mod=readonly -count=1 .`, `python -B test/bootstrap/verify.py` and `git diff --check 85799b6 HEAD` exited 0.
- The original SQLite matrix passed with `python -B test/conformance/lab/sqlite_compat.py --model-types --owned-tx --savepoint-tx --save-changes` and exit 0. It reported 14 driver rows, four clean-close file cases, two model type rows, two owned transaction rows, four savepoint rows and six save changes rows.
- The standalone launcher also exited 0 on integrated revision `c9a438928cc25b0c8ee8fde7bf78e27d037d6e65`. Both profiles reported their pinned SQLite source IDs. Each committed writer exited with the required code 67 and the peer completed recovery and checkpointing.
