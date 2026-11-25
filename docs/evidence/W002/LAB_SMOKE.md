# W002 four-engine smoke evidence

[W002 #13](https://github.com/igorpooh1978/GoEF/issues/13) · [Runner #19](https://github.com/igorpooh1978/GoEF/issues/19)

The first executable slice passed on four real, disposable engines on 2026-11-25. The verified [lab subtree](../../../test/conformance/lab/) is `dc3befa4f804b6584641e23633ffa00a9cb42a90`. Use `git rev-parse HEAD:test/conformance/lab` to compare the runner and fixtures. Host execution used Python 3.14.4 on Windows and Docker Desktop 29.4.0 with a Linux/amd64 daemon. No host connection string or production resource was used.

| Cell | Observed identity | Real smoke result |
| --- | --- | --- |
| [PostgreSQL #20](https://github.com/igorpooh1978/GoEF/issues/20) | 18.7, server version number 180007, pinned Linux/amd64 image | Exact database and run marker verified. Duplicate key produced SQLSTATE 23505 for the expected constraint. A fresh client read the intact committed seed after rollback. |
| [SQLite #23](https://github.com/igorpooh1978/GoEF/issues/23) | 3.54.1, sources identified by `2026-10-16 14:45:20 555e07ef33f69a0957d24d8e555b702464a131d6f1607f4c3c1e4551e8e6ca76` | DELETE and WAL passed. The valid foreign-key insert succeeded. The invalid insert exited 1 with the exact foreign-key diagnostic. Read-only reopen confirmed the seed after explicit rollback and process exit following the failed transaction. |
| [SQL Server #22](https://github.com/igorpooh1978/GoEF/issues/22) | SQL Server 2022 CU28 Developer, 16.0.4325.4, compatibility 160, Latin1_General_100_BIN2 | Exact build, database GUID and per-run marker checks passed. Duplicate and foreign-key codes were 2627 and 547. A fresh sqlcmd query confirmed the restored baseline after rollback. The local run used explicit Developer edition license acceptance. |
| [MySQL #21](https://github.com/igorpooh1978/GoEF/issues/21) | Oracle MySQL Community 8.4.13.1, InnoDB, READ-COMMITTED, utf8mb4_bin, UTC offset +00:00 | The fixture login verified the provisioner's marker and target. The duplicate raised 1062/23000. A fresh client read the committed seed after rollback. |

The three server image digests are recorded in the [artifact inventory](TEST_LAB_DESIGN.md). The Windows SQLite x64 archive `sqlite-tools-win-x64-3540100.zip` was verified against the SQLite download record with SHA3-256 `e171f3993a992edf076746926dd52b5e2ebfabaee38818e74987e1d8112ab987`. The CLI was used from temporary storage. [Official download information](https://www.sqlite.org/download.html)

## Reproduce and interpret

Run the commands listed in the [lab README](../../../test/conformance/lab/README.md). The four cells ran as separate commands and each returned exit 0 and `PASS`. The SQL Server command without the explicit per-run license flag returned exit 1 and `BLOCKED` before Docker. The harness ran 22 unit tests, including unsafe endpoint, ownership/cleanup and expected diagnostic controls. A deliberately wrong target marker was also exercised with isolated provider fakes before fixture dispatch.

Each successful server cell verified Docker cleanup by the owned container identity and checked attached anonymous volumes. The final label-filtered inventory contained zero lab containers. SQLite's file and journal data lived in a temporary directory removed at the end of its run. Image caching is independent of container cleanup.

This evidence covers engine identity, lab isolation and the specific native CLI smoke checks. It carries no GoEF adapter or Go driver results. The larger W002 issue remains open for driver approval, automated scenario bindings, per-provider permissions and setup profiles, CI and actual GoEF conformance. The four GoEF runtime cells remain `NOT RUN`.
