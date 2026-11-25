# Plan a first Code First workflow

Status: Proposed. The example is a planned acceptance case and has no runnable GoEF API yet.

Propose registering a customer with a unique key and optional address, inspecting a migration plan, and reading it back. Use a synthetic customer key so the same fixture can be replayed on each provider. The negative case checks a duplicate key without printing the supplied value.

Feature IDs: MODEL-MAP-001, MIG-API-001, ERR-CONSTRAINT-001.

- [Design or source evidence](../architecture/PUBLIC_API.md)
- [Planned examples and evidence requirements](../../examples/README.md)
