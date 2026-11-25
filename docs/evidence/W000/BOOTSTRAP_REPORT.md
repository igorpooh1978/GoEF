# W000 bootstrap report

Mode: BOOTSTRAP_ONLY. Result: PARTIAL. Tracking: [issue #1](https://github.com/igorpooh1978/GoEF/issues/1). Baseline: 2a886c78e622f583fa2980ea0a61a7e328a40e28.

The repository contains the English architecture, documentation, skills, specs, and evidence required to resume development. PostgreSQL and MySQL still lack exact documentation-version confirmation through MCP. W001 and W002 have tracked handoffs. See the [checkpoint](../../roadmap/CHECKPOINT.md).

## Integrated gate results

| Area | Observed result |
| --- | --- |
| Sources | The [lock](../../upstream/lock.json) pins EF Core v10.0.15, .NET runtime v10.0.0, the separate go-linq release and research revisions, and two EF provider reference trees. Six commit objects, five tags and six license blobs were checked. The Pomelo reference targets EF Core 9, so an EF Core 10 MySQL differential comparison remains blocked. |
| Inventory | 50 feature records, 31 specified and 19 discovered. The research survey identifies 21 EF families, of which 18 have at least one seed. The feature matrix includes 168 unrun and 32 explicitly inapplicable provider cells. Full callable behavior enumeration remains open. |
| LINQ | 627 source/member groups and 1,287 overload declarations. The groups comprise 257 Objects, 200 Expressions and 170 Queryable. Separate go-linq release/research groups preserve provenance. See the [operator inventory and gap study](../../upstream/LINQ_RESEARCH.md). No group has executable GoEF evidence. |
| Errors and transactions | 41 proposed portable error IDs and 24 proposed native mappings. The [error contract](../../errors/README.md) and [transaction contract](../../architecture/TRANSACTION_ENGINE.md) cover ownership, connection affinity, isolation, savepoints, bounded replay and ambiguous commit. Runtime conformance and native-code reproduction remain open. |
| Gherkin | 64 specified scenarios and zero with executable GoEF tests. The [traceability index](../../../specs/traceability.json) records an issue, feature, spec and documentation per scenario. No behavioral outcomes were marked passed. |
| Compiler feasibility | The [isolated probe](GO_FEASIBILITY.md) built, passed vet and passed its positive and five negative tests on Go 1.27.4. The race test is blocked on this host because cgo is disabled and the configured C compiler is missing. |
| Skills | Codex CLI discovered all 18 repository skills and an explicit skill attachment loaded the intended body in an ephemeral turn. A content-assisted exercise run satisfied 36 of 36 criteria after correction and retest. Implicit skill selection and hosted issue-form rendering remain unverified. See the [skills report](SKILLS_REPORT.md). |
| Documentation | Linked English user and reference guides, proposed workflows, and [an example catalog](../../../examples/README.md). No runnable GoEF examples are claimed. Relative links, schemas and traceability are checked locally. |
| Issue sync | 15 actual issues were created and read back on the intended repository. The [checkpoint](../../roadmap/CHECKPOINT.md) lists owners, work and blockers. All descriptions and lifecycle labels were reconciled with the live issues. Keep the issues open until evidence can be verified on GitHub. |
| Four-provider behavior | Versions and driver combinations in the [matrix](../../parity/PROVIDER_MATRIX.md) are research candidates. No real provider test ran. GoEF production coverage has no denominator at this stage. |
| Project license | Source research licenses are documented. The new GoEF project's license and attribution require a maintainer decision in [issue #15](https://github.com/igorpooh1978/GoEF/issues/15). |

## Documentation MCP

Two public documentation endpoints successfully negotiated the protocol. All four provider documentation lookups returned results. [Protocol details and authoritative sources](MCP_REPORT.md).

| Provider | Exact documented candidate | MCP version decision |
| --- | --- | --- |
| PostgreSQL | 18.7 | BLOCKED. The Context7 corpus returned 18.6 as its newest entry. [Issue #10](https://github.com/igorpooh1978/GoEF/issues/10) |
| SQLite | 3.54.1 | Exact source match in the bounded documentation check. [Issue #8](https://github.com/igorpooh1978/GoEF/issues/8) |
| SQL Server | 2022 CU28 / 16.0.4325.4 | Exact source match in the bounded Microsoft Learn check. [Issue #6](https://github.com/igorpooh1978/GoEF/issues/6) |
| MySQL | 8.4.13.1 | BLOCKED. Context7 returned 8.4 family materials without confirming the exact update. [Issue #11](https://github.com/igorpooh1978/GoEF/issues/11) |

Official manuals provide fallback research. No MCP route was permanently configured or granted database access.

## Execution and verification

- Repository validator: python test/bootstrap/verify.py, exit 0 on the integrated candidate. Negative controls rejected a missing Markdown file, an orphan scenario, a false PASS and a stale evidence revision. They were exercised in a disposable worktree. The final seven regression checks in test/bootstrap/test_verify.py also cover false PASS at the current SHA, native-code promotion, invalid operator references and stale reference counts.
- Compiler probe: go build ./..., go vet ./..., and go test -count=1 -shuffle=on -v ./..., exit 0 in the isolated module with GOWORK off, GOPROXY off and local Go 1.27.4. The exact negative and race-test diagnostics are in the [probe report](GO_FEASIBILITY.md).
- Tooling for the local validator: Python 3.14.4, jsonschema 4.26.0 and PyYAML 6.0.3. No application dependencies or CI deployment were added.
- Repository checks: staged git diff --cached --check, issue metadata readback, and final revision recheck. The integrated revision is the commit containing this report. Resolve it with git log -1 --format=%H -- docs/evidence/W000/BOOTSTRAP_REPORT.md.

Six independent worktrees supplied upstream research (837f2f3), LINQ (891ec46), architecture and compiler proof (b7f1118), transactions, migrations and errors (f31c9b6), provider documentation (c29096d), and skills and workflow (e93f60a and 0c4d031). The integrator added guides, specs, traceability, audit and validation.

User-global Go skills were installed and discovered independently of repository skills. The new testing policies apply named table-driven cases and separate integration evidence from both the [samber collection](https://github.com/samber/cc-skills-golang/tree/d68d80aec298e1b74b37c2a472c002b5217248b3) and the [justxor collection](https://github.com/justxor/golang-agent-skills-ru/tree/c9708a5bc65b537a7db99e9bb597739a05e70153). Transaction evidence applies their resource ownership, driver-native errors and cancellation guidance. The collections are optional user tooling.

## Independent review

Two independent reviewers checked candidate 68ebbd7ebf8b4398492cecf718f64adcd4d045cb. The technical reviewer reproduced fabricated runtime acceptance and upstream LINQ provenance defects. The integrated checker now rejects runtime promotion until the W002 harness exists, cross-checks LINQ source/profile/provider keys with the lock and registry, and validates the reference counts. Architecture text now identifies provider modules as the consumer of both SPI and drivers. Both lanes rechecked the integrated fixes. A remaining SQL Server catalog link found in the technical recheck was also updated to the verified 2022 documentation view.

The documentation reviewer confirmed the master plan, required files, English status labels, relative links and skill content digests. Review findings were addressed by linking project licensing to issue #15, retaining upstream provenance in issue #7, reading back owners and labels, and adding per-issue evidence and dependencies on GitHub.

The local CodeGraph index remains outside the tracked change. No existing local files were cleaned to force a zero-line status.

## Next work

Start the architecture and compile-fixture work in [issue #12](https://github.com/igorpooh1978/GoEF/issues/12) after the mode changes. Keep the exact MCP-version work, license decision and [real-engine infrastructure](https://github.com/igorpooh1978/GoEF/issues/13) open. Record local acceptance without publishing or deploying in this mode.
