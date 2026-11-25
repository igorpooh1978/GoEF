# GitHub issue workflow

Use English for contributions, including issues, comments, PRs, commit messages, evidence and all repository artifacts. Preserve source quotations and fixture values when they are necessary. The [GoEF issue tracker](https://github.com/igorpooh1978/GoEF/issues) is authoritative. [W000 #1](https://github.com/igorpooh1978/GoEF/issues/1) tracks the bootstrap and [#5](https://github.com/igorpooh1978/GoEF/issues/5) tracks this workflow.

## Open or reuse an issue

1. Verify the Git origin points to the intended GoEF repository. Search open and closed issues using authorized GitHub tooling. Reuse and clarify an existing matching ticket. Create or reuse a wave parent, then link one independently verifiable issue per behavior or bounded body of work. Use the GoEF tracker for work based on EF Core, dotnet/runtime and go-linq research.
2. Record objective, non-goals, upstream pin/permalinks for API, implementation and tests, relevant database/driver documentation, feature and error IDs, provider versions, required Skills, owner and allowed paths, parent and dependencies. Include scenarios, observable acceptance criteria, RED/GREEN or characterization commands, documentation and examples, migration/security impact, evidence and target release. Include exact affected versions and a minimal reproduction for a bug. Use `N/A` with a reason where a field is inapplicable and cite missing evidence as pending.
3. If GitHub access is unavailable, draft numbered English files under `docs/roadmap/issue-drafts/` and record `ISSUE_SYNC_BLOCKED` in the checkpoint. Keep draft numbers distinct from GitHub IDs. Reconcile and deduplicate against GitHub before implementation. Safe bootstrap documentation can continue with the explicit blocker.

## Track work and synchronize

| State | What warrants it |
| --- | --- |
| `OPEN` | Authoritative issue exists and is scoped for triage. |
| `READY` | Dependencies, contracts, paths and acceptance are understood. |
| `IN_PROGRESS` | Owner has started and reports scope, blockers and evidence. |
| `REVIEW` | Integrated candidate and test evidence are ready for review. |
| `ACCEPTANCE` | Independent reviewer verifies applicable gates on the integrated SHA. |
| `VERIFIED` | Orchestrator has accepted complete evidence on the integrated SHA. |
| `CLOSED` | Accepted SHA and evidence are verifiable remotely. A parent also requires all mandatory child issues accepted and linked. |
| `BLOCKED` / `DEFERRED` | Record a reason, next action and owner. Keep unsupported required provider cells open or explicitly deferred. |
| `AWAITING_MANUAL_PUBLISH` | Local acceptance is recorded and the remote SHA is pending separately authorized publication. |

Apply canonical labels by dimension using the [actual repository label inventory](https://github.com/igorpooh1978/GoEF/labels). Use the available labels or request their maintenance under the tracking issue. Reserve `wave:wNNN` for the scheduled wave, `type:*` for work categories such as feature/bug/research/docs, `subsystem:*` for the affected contract, `provider:*` for each affected engine, `severity:*` for triaged defects, and `status:*` for the current lifecycle state. W000 activated and read back the initial remote labels. Reconcile existing remote labels before adding or replacing them. Labels support navigation; record the current state, accepted SHA and unresolved blockers explicitly in the issue and checkpoint.

Use an issue-named branch and link it in commits, ADRs, Gherkin/feature/error records, evidence, changelog, docs, and PRs when authorized. For example, a branch for issue #5 can be named `w000/issue-5-skills` and its commit `docs: establish contributor workflow and skills (#5)`. Before integration confirm the issue resolves in the verified origin, scope and IDs agree across artifacts, and dependencies are reflected in the checkpoint. Before closure verify remote SHA and accepted evidence, confirm every required child for a parent, and re-read the issue state after a permitted mutation. Avoid automatic closing keywords in issue references while acceptance or publication is pending.

## Example issue

The real [repository Skills issue #5](https://github.com/igorpooh1978/GoEF/issues/5) is a W000 child of [#1](https://github.com/igorpooh1978/GoEF/issues/1). Its scoped work is to define the contributor workflow and author the 18 reusable repository Skills. Follow [master plan §§14–15 and §21](../roadmap/MASTER_PLAN.md). Record the issue's actual owner and baseline in the issue/checkpoint. Acceptance includes installed-client discovery from the repo root, skill metadata and links, task-shaped positive and negative evals, and GitHub issue form syntax checks. Runtime feature/error IDs and engine tests do not apply to this documentation issue. Link check and evaluation results from the issue, and disclose unavailable execution or publication explicitly.

GitHub Issue Forms collect and require the fields marked in their YAML. They do not check cross-artifact semantics, do not enforce PR Markdown checkboxes, and can be bypassed by write-access maintainers. Verify required links, issue states and evidence during triage and acceptance. Local form checks verify structure. Hosted rendering needs a separate preview after authorized publication. See [GitHub issue form syntax](https://docs.github.com/en/communities/using-templates-to-encourage-useful-issues-and-pull-requests/syntax-for-issue-forms), [form schema](https://docs.github.com/en/communities/using-templates-to-encourage-useful-issues-and-pull-requests/syntax-for-githubs-form-schema) and [template chooser configuration](https://docs.github.com/en/communities/using-templates-to-encourage-useful-issues-and-pull-requests/configuring-issue-templates-for-your-repository).
