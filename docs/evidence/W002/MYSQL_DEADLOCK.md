# W002 MySQL InnoDB deadlock and recovery

[Conformance #13](https://github.com/igorpooh1978/GoEF/issues/13) · [Driver lab](../../../test/conformance/driverlab/README.md) · [Focused test](../../../test/conformance/driverlab/mysql_deadlock_test.go)

Two real InnoDB transactions each change an independent sentinel and hold a distinct contested row. The lab starts the first conflicting update and uses `performance_schema.data_lock_waits` to confirm its exact requester and blocker. It then issues the second conflicting update to close the cycle. The test accepts either victim, requires exactly one typed `1213/40001` under the live caller context and verifies that the other update succeeds. [InnoDB deadlock detection](https://dev.mysql.com/doc/refman/8.4/en/innodb-deadlock-detection.html)

Before issuing an explicit rollback on the victim handle, the test reads that transaction's original sentinel from the same connection. It also checks that the survivor still sees its uncommitted change, commits the survivor and verifies all four final rows. A fresh transaction on the original victim session commits a new write that the opposite connection can read. This establishes the engine's rollback and the session's reuse in the fixture. Automatic transaction replay retains a separate acceptance case.

```sh
python -B test/conformance/lab/mysql_deadlock.py
```

The launcher builds one binary for the two pinned MySQL profiles and verifies the exact, redacted result after each owned container is cleaned up. Use `--profile mysql-8.0` or `--profile mysql-8.4` for an individual run.

## Local verification

The test ran on executable revision `05469eb1e2024b1e752cec47bc9f216ed80ddc8a` with Go 1.27.4, go-sql-driver/mysql v1.10.0 and the test profiles' digest-pinned Linux/amd64 engines on the local Docker Desktop daemon. MySQL 8.0.46 passed the first run. A lab command timed out on the first MySQL 8.4.13.1 attempt, so the combined command exited 1. The isolated `--profile mysql-8.4` retry on the same clean revision exited 0 and passed. Both passing profiles reported `1213/40001`, the observed waiter edge, the victim rollback before explicit cleanup and both successful commits. No lab-owned containers remained after the retry.

The first and retry redacted reports have SHA-256 `6e0a991d9b6559a4ede15c8dd9e8e66eaf3ecfb22551d9dcc830c63ef0bf0572` and `8420181dac99871ba4bacdbacf98b6ab32136f423320cc0a176c1495a9c40fcd`, respectively.
