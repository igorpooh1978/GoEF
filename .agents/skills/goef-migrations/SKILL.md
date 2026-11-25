---
name: goef-migrations
description: Design or review GoEF model diff and manual Go migrations, API/CLI/script entry points, dialect DDL, history, locking, data preservation and recovery.
---

# GoEF Migrations

1. Read [migration design](../../../docs/architecture/MIGRATION_ENGINE.md), [migration Go API guide](../../../docs/user/MIGRATIONS_IN_GO.md) and the relevant provider skill. Identify model, target versions, lock and history ownership.
2. Trace deterministic model snapshot diff to operations, provider SQL and execution with history/checksums. Preserve published migrations. Keep programmatic and model-driven entry points viable without the CLI.
3. Establish transaction/implicit-commit behavior, destructive-operation approval, advisory/exclusive locking, recovery, drift handling and repeatability. Provide reviewable SQL, and mark provider operations impossible to roll back precisely.
4. In a disposable engine, exercise representative data round trips, dependent constraints and indexes, rollback or recovery, concurrent migration locking, cancellation, and an interruption between DDL and history. Record pre/post state and audited evidence.

For SQLite table rebuild, copy rows and restore supported constraints, triggers and indexes. A failure-injection test must verify row/value preservation and recovery on the selected SQLite version.
