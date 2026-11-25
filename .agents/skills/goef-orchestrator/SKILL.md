---
name: goef-orchestrator
description: Resume a GoEF wave, schedule issue dependencies, assign bounded parallel lanes, integrate worktrees, and decide acceptance from the exact tested SHA.
---

# GoEF Orchestrator

1. Read [workflow](../../../docs/development/AGENT_WORKFLOW.md) and the [checkpoint](../../../docs/roadmap/CHECKPOINT.md). Verify the mode, target origin, issues, baseline SHA and working tree. Reconcile stale status against current GitHub and evidence.
2. Build a dependency graph. Assign allowed paths, decision ownership, baseline and integration order. Keep shared API/SPI, registry, module files, migration IDs and snapshots single-owned. Separate implementation from acceptance. Bound lanes by available agent slots and isolated resources. Record actual execution.
3. Preserve mode constraints. In W000 use isolated proofs, research, contracts, specs and documentation. For a blocked dependency, advance independent lanes and state the gate it prevents.
4. Integrate in dependency order, rerun relevant gates at the final SHA and request fresh independent acceptance. Update the checkpoint with exact SHAs, actual commands/statuses, blockers and next ready issues.
5. Close a remote issue only after orchestrator acceptance and verified remote evidence. For accepted local-only work use `AWAITING_MANUAL_PUBLISH` and retain open remote tracking.

If providers depend on an unsettled SPI, assign their research and fixtures first. Freeze SPI and scenario IDs before parallel implementation.
