# Review and apply a schema change

Status: Proposed. The example is a planned acceptance case and has no runnable GoEF API yet.

Compare an accepted model snapshot with the target and inspect the generated operations, loss flags and history checks. The planned example adds a nullable column and applies the migration twice while preserving existing data and one history row. The application can run the migration through a Go API.

Feature IDs: MIG-API-001, MIG-DRIFT-001, MIG-LOCK-001.

- [Design or source evidence](../architecture/MIGRATION_ENGINE.md)
- [Planned examples and evidence requirements](../../examples/README.md)
