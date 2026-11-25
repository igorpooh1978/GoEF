# Own a transaction explicitly

Status: Proposed. The example is a planned acceptance case and has no runnable GoEF API yet.

Run a write and a read on the same physical transaction and decide whether to commit. The planned example rolls back and verifies durable state in a separate context. When GoEF borrows a transaction, the caller retains the commit and rollback decision. A lost response at commit requires outcome reconciliation.

Feature IDs: TX-OWN-001, TX-AFFINITY-001, TX-SAVEPOINT-001, TX-OUTCOME-001.

- [Design or source evidence](../architecture/TRANSACTION_ENGINE.md)
- [Planned examples and evidence requirements](../../examples/README.md)
