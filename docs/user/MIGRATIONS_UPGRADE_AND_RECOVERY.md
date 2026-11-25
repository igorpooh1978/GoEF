# Recover an interrupted migration

Status: Proposed. The example is a planned acceptance case and has no runnable GoEF API yet.

Inspect the recorded phase, history and actual schema before deciding who can resume or repair a migration. The planned fixture interrupts a SQLite rebuild and a MySQL multi-operation sequence separately. Compare observed behavior with the engine-specific strategy before replay.

Feature IDs: MIG-RECOVERY-001, MIG-SQLITE-001, MIG-MYSQL-001.

- [Design or source evidence](../architecture/MIGRATION_ENGINE.md)
- [Planned examples and evidence requirements](../../examples/README.md)
