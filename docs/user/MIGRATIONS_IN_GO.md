# Register a migration by hand in Go

Status: Proposed. The example is a planned acceptance case and has no runnable GoEF API yet.

Declare a migration ID and ordered operations, register the definition, then ask the programmatic API for a plan. Review rename intent, rollback feasibility and checksum before applying the plan. The example will exercise both matched and mismatched history.

Feature IDs: MIG-API-001, MIG-RENAME-001, MIG-DRIFT-001.

- [Design or source evidence](../architecture/MIGRATION_ENGINE.md)
- [Planned examples and evidence requirements](../../examples/README.md)
