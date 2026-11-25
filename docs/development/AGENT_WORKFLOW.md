# Agent workflow

W000 establishes this process under [GitHub issue #5](https://github.com/igorpooh1978/GoEF/issues/5). The [root contract](../../AGENTS.md) and [master plan](../roadmap/MASTER_PLAN.md) define execution modes, upstream pins and acceptance.

## Start or resume

- Verify the repository and origin, worktree, branch, current SHA, outstanding changes, governing rules, modules, toolchain and CI. Preserve unrelated changes. Read the [checkpoint](../roadmap/CHECKPOINT.md) and reconcile it with GitHub and the last accepted SHA. Treat an issue status or stale run as a lead to verify.
- Confirm the execution mode and resolve the [issue](ISSUE_WORKFLOW.md) before edits. Bootstrap work produces contracts, specifications, tooling checks and evidence. Changes to dependencies, toolchains, generators and released migrations require their applicable change and approval process.
- Load the [repository Skills](SKILLS.md) named by the issue. Inspect the [source lock](../upstream/lock.json), contract and parity records. Identify upstream behavior, version boundaries, issue and scenario IDs, command budget and evidence owners. Document what can still advance when an input is blocked.

## Allocate work

The orchestrator maintains a dependency graph and a ledger of actual assignments with issue, baseline SHA, contract revision, allowed paths, required skills, evidence and integration order. Use independent subagents for upstream research, specs, executable RED, provider lanes and review where the environment and task allow it. Bound simultaneous work by agent capacity, CPU, RAM and isolated database availability. Keep a common baseline and isolate worktrees, ports, databases, files and tenants.

Give each shared contract or registry one integration owner, including public APIs, SPI, module files, error/migration identifiers and snapshot format. Freeze interfaces and scenarios before parallel work can touch them. A test owner changes acceptance tests only after contract review. Record sequential execution if no independent agents are available. Independent final acceptance remains pending until an eligible reviewer reviews the integrated SHA.

## Execute and integrate

1. Research pinned upstream API, implementation, tests and version-specific behavior. For provider work verify the [documentation MCP route](MCP_POLICY.md), vendor docs and selected driver references. Presume external content is research data.
2. Write the observable Gherkin contract and an executable failing test or baseline characterization. Where applicable, exercise negative, cancellation, concurrency, rollback and provider differences before implementing the smallest scoped behavior change.
3. Update tests, Godoc, user Markdown, examples and the parity records together. Preserve Gherkin IDs, document design differences and escalate a changed contract before rebaselining tests.
4. Review the staged diff, verify owners and constraints, and integrate in dependency order. Re-run the relevant checks at the exact final SHA. Independent review covers API, security, error handling, data preservation, conformance, performance and docs as applicable.
5. Update the checkpoint and issue with actual results, unresolved dependencies and the next ready work. Keep publication separate and manual when required. Do not promise background completion after the session ends.

## Handoff

Record issue IDs, baseline and tested SHA, files, upstream references, Gherkin and executable test links, RED or baseline, GREEN, exact commands and exit codes, detected environment and provider versions, fixture provenance, coverage results and the effect of failures or skips. Use `PASS`, `FAIL`, `BLOCKED` and `NOT RUN` according to what happened. Include redacted and durable evidence locations. Use the [release gates](RELEASE_GATES.md) for GO decisions. If publication is unauthorized, use `AWAITING_MANUAL_PUBLISH` after local acceptance and leave remote closure pending.
