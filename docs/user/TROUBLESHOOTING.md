# Diagnose a failed data operation

Status: Proposed. The example is a planned acceptance case and has no runnable GoEF API yet.

Identify the operation phase, portable error kind, provider version and commit outcome before attempting recovery. Reproduce on a synthetic fixture and capture safe metadata. A migration history mismatch requires inspection, while an unknown commit outcome requires reconciliation.

Feature IDs: ERR-CONTRACT-001, TX-OUTCOME-001, MIG-DRIFT-001.

- [Design or source evidence](../architecture/ERROR_CONTRACT.md)
- [Planned examples and evidence requirements](../../examples/README.md)
