# Repository Skills

The 18 GoEF skills live in `.agents/skills/<name>/SKILL.md`. They describe how to perform work against repository contracts. Completion and parity are established by issue-linked evidence.

## Discover and load

Open Codex from the repository root or worktree. Use `/skills` to see available skills or explicitly invoke `$goef-query-linq` (replace the name for your issue). Where a client has no menu or invocation UI, open the exact linked `SKILL.md` and follow the instructions and links in that worktree. Keep one canonical copy and read only the references relevant to the issue. The [W000 skill evidence](../evidence/W000/SKILLS_REPORT.md) records the local discovery result.

Read the issue's required skill list first. Use upstream research for behavior work, a subsystem skill for the affected contract, provider-specific and error skills for each affected boundary, and testing/docs skills when those artifacts change. Objects work typically also uses LINQ parity when operator contracts change. A database translation change uses query LINQ, LINQ parity and the affected provider skills. Use acceptance independently after integration.

## Skill index

| Skill | Use it to |
| --- | --- |
| [`goef-orchestrator`](../../.agents/skills/goef-orchestrator/SKILL.md) | Resume and integrate issue-linked waves, assign worktrees, and make acceptance decisions. |
| [`goef-upstream-research`](../../.agents/skills/goef-upstream-research/SKILL.md) | Establish API, source, test and license evidence before a behavioral change. |
| [`goef-model-engine`](../../.agents/skills/goef-model-engine/SKILL.md) | Design model metadata, keys and relationships, and prove deterministic snapshots. |
| [`goef-query-linq`](../../.agents/skills/goef-query-linq/SKILL.md) | Translate typed expressions and generated query plans through IR and provider SQL. |
| [`goef-linq-objects`](../../.agents/skills/goef-linq-objects/SKILL.md) | Specify and test typed local sequences, laziness, ordering and resource lifetime. |
| [`goef-linq-parity`](../../.agents/skills/goef-linq-parity/SKILL.md) | Inventory operator families, provenance, execution modes and provider support. |
| [`goef-tracking`](../../.agents/skills/goef-tracking/SKILL.md) | Specify identity, snapshots, graph changes, SaveChanges and rollback state. |
| [`goef-migrations`](../../.agents/skills/goef-migrations/SKILL.md) | Design model diff, migration entry points, history, locking and recovery. |
| [`goef-transactions`](../../.agents/skills/goef-transactions/SKILL.md) | Verify connection affinity, ownership, savepoints, retry and uncertain outcomes. |
| [`goef-provider-postgres`](../../.agents/skills/goef-provider-postgres/SKILL.md) | Research and validate PostgreSQL and pgx behavior. |
| [`goef-provider-sqlite`](../../.agents/skills/goef-provider-sqlite/SKILL.md) | Research and validate SQLite and selected driver behavior. |
| [`goef-provider-sqlserver`](../../.agents/skills/goef-provider-sqlserver/SKILL.md) | Research and validate SQL Server and selected driver behavior. |
| [`goef-provider-mysql`](../../.agents/skills/goef-provider-mysql/SKILL.md) | Research and validate MySQL/InnoDB and selected driver behavior. |
| [`goef-errors`](../../.agents/skills/goef-errors/SKILL.md) | Maintain typed errors, redaction, stable IDs and native mappings. |
| [`goef-gct-testing`](../../.agents/skills/goef-gct-testing/SKILL.md) | Link Gherkin to executable RED/GREEN, conformance and coverage. |
| [`goef-docs`](../../.agents/skills/goef-docs/SKILL.md) | Update English docs, Godoc, examples, links and generated references. |
| [`goef-security-review`](../../.agents/skills/goef-security-review/SKILL.md) | Review trust boundaries, credentials, tenant isolation and migration safety. |
| [`goef-acceptance`](../../.agents/skills/goef-acceptance/SKILL.md) | Independently audit integrated SHA evidence and decide GO or NO-GO. |

## Optional user-local Go resources

When available, use `go-best-practices` for relevant workflow, architecture, interface, database, engineering and testing guidance. Other opt-in topic references include [samber/cc-skills-golang](https://github.com/samber/cc-skills-golang/tree/d68d80aec298e1b74b37c2a472c002b5217248b3) (`golang-testing`, `golang-database`, `golang-structs-interfaces`) and [justxor/golang-agent-skills-ru](https://github.com/justxor/golang-agent-skills-ru/tree/c9708a5bc65b537a7db99e9bb597739a05e70153) (`go-testing`, `go-postgres`, `go-production`). The repository works without these packs. Keep the repository's architecture, documented versions, permission boundaries and source pins authoritative. Record conflicts and keep canonical outputs in English.

## Maintain and evaluate

Change affected skills with the same issue and behavior that changed. Keep each entrypoint concise, with matching directory/frontmatter name, a description that distinguishes related skills, links to canonical files and a bounded workflow. Put longer material behind links only when useful. Add client metadata only if needed by the supported client.

Run structural discovery/frontmatter/link checks and the [behavior exercises](SKILL_EVALS.md). Report a structural result separately from behavioral review and automatic skill selection. Test explicit and implicit activation separately if claiming both, and retain responses and scoring evidence for behavioral runs. W000 Skills are tracked in [#5](https://github.com/igorpooh1978/GoEF/issues/5).
