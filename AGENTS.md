# GoEF repository instructions

Read the [master plan](docs/roadmap/MASTER_PLAN.md), especially sections 14 through 21, and the [checkpoint](docs/roadmap/CHECKPOINT.md) before work. Use English for repository artifacts, issues, PRs, commits, code comments, diagnostics, and evidence. Every change must link to an actual GitHub issue or a documented blocked issue draft under the [issue workflow](docs/development/ISSUE_WORKFLOW.md).

Use Python for repository scripts and command orchestration. Invoke native tools directly or from Python subprocesses.

## Before editing

1. Confirm issue scope, baseline, mode, worktree, affected contracts, and acceptance criteria. Keep unrelated changes intact. Apply [repository skills](docs/development/SKILLS.md) and relevant Go guidance. Use independently owned worktrees and parallel agents when available. The integrator owns shared schemas, SPI, module files, and root rules.
2. Work upstream first. Read the pinned behavior and tests, check licensing, record exceptions, and update the [feature and operator registries](docs/parity/README.md). Confirm engine facts using the [MCP policy](docs/development/MCP_POLICY.md) and exact vendor and driver references. Keep secrets and private fixtures out of the repository, logs, and requests.
3. Use Go 1.27.4 and idiomatic typed contracts. Prove language and API claims by compilation. Use context for I/O and explicit resource ownership. Approve dependency, generator, or toolchain changes separately. Preserve released migrations and require explicit approval for destructive operations.

## Required boundaries

- Keep the core independent of database drivers and PostgreSQL assumptions. Each required provider has separate version and capability evidence.
- Share one Model Engine and Query IR. Preserve separate GoLINQ Objects, Expressions, and Queryable contracts. Read the Objects and LINQ parity skills for relevant work. Run a distinct object suite and four SQL suites. Cross the database-to-objects boundary explicitly.
- Treat programmatic migrations and transactions as public subsystems. Keep the CLI optional. Preserve migration history, recoverability, physical connection affinity, caller ownership, and tracker consistency. An unknown commit outcome must surface as a typed error with no blind replay.
- Keep the stable [error registry](docs/errors/README.md), typed errors, native mappings, retry and commit outcomes, safe metadata, and scenario evidence aligned.
- Use [Gherkin specifications](specs/README.md), executable RED or recorded characterization, implementation, GREEN, and independent acceptance for behavior changes. Require actual engines for provider claims, including negative and recovery cases. Apply the [coverage gates](docs/development/COVERAGE.md) per module and provider.

## Definition of done

Update contracts, specs, tests, registries, Godoc, user docs, executable examples, and changelog together as applicable. Follow the [documentation policy](docs/development/DOCUMENTATION_POLICY.md), [agent workflow](docs/development/AGENT_WORKFLOW.md), [testing policy](docs/development/TESTING.md), and [release gates](docs/development/RELEASE_GATES.md). Record exact commit, versions, commands, exit status, exceptions, and independent review. Keep unsupported, unrun, and blocked results visible. Check the staged diff and keep the worktree clean. Use separately authorized push, deployment, and release operations. Never force-push or rewrite other contributors' history.
