# Transaction guarantees

Status: Proposed. This reference describes the intended contract. Its usage examples are planned.

The proposed states are active, committed, rolled back, failed and outcome unknown. Commands preserve physical connection affinity. A borrowed transaction retains caller ownership. A transient failure may permit bounded whole-transaction replay after verified rollback and an explicit replay contract.

Feature IDs: TX-OWN-001, TX-AFFINITY-001, TX-RETRY-001, TX-OUTCOME-001.

- [Canonical design or registry](../architecture/TRANSACTION_ENGINE.md)
- [Example acceptance criteria](../../examples/README.md)
