# W000 repository Skills evidence

This work addresses [GoEF #5](https://github.com/igorpooh1978/GoEF/issues/5). It starts from `2a886c78e622f583fa2980ea0a61a7e328a40e28`, on 2026-11-25. The evaluated entrypoint SHA-256 values below identify the skill content even when it is integrated under a different commit. Reproduce the digests from Git blob bytes rather than checkout-translated line endings.

| Check | Observed result |
| --- | --- |
| Repository discovery | PASS. Installed Windows Codex CLI 0.162.1 returned 18/18 enabled entries with repository scope, under `.agents/skills/<name>/SKILL.md`, and zero listing errors. |
| Prompt skill metadata | PASS. `codex debug prompt-input '$goef-query-linq'` listed all 18 entrypoint names. This inspected the initial model-visible prompt metadata. |
| Skill structure | PASS. The host skill creator's `quick_validate.py` exited 0 for each of 18 directories. Independent YAML parsing found 18 exact names, matching folders and two expected metadata keys. Each entrypoint is below 500 lines. |
| Relative links | All 70 distinct staged Markdown target files resolved across the W000 worktrees. Recheck on the integrated tree before final acceptance. |
| Issue forms and chooser | PASS for local syntax and structure. PyYAML 6.0.3 parsed all five forms and chooser. Checks covered unique IDs and labels, allowed field types, mandatory inputs and expected chooser behavior. PR template section and checklist content also passed. Hosted preview remains NOT RUN because the forms were not published. |
| Direct file-based CLI behavior run | BLOCKED. Nested read-only Codex sessions were denied local shell reads. The command exit alone did not detect the missing skill content. |
| Content-assisted skill behavior | PASS for the tested decisions. All 36 cases met the rubric on the current entrypoint contents after focused rechecks. |
| Explicit selection | PASS for one direct app-server skill attachment. The model quoted two exact skill body excerpts without tool calls. |
| Implicit skill selection | NOT RUN. The decision exercises supplied the full entrypoint text, and the explicit attachment check selected a named entrypoint. |
| Go and database runtime | NOT APPLICABLE to these decision exercises. |

Discovery used `codex app-server --listen stdio://` and JSON-RPC messages `initialize`, notification `initialized`, then `skills/list` with `cwds` set to the current repository worktree and `forceReload: true`. The response was filtered by repository scope and path. The optional user-local packs are outside the repository and are described in the [Skills policy](../../development/SKILLS.md).

## Explicit attachment smoke check

With the W000 worktree as the working directory, the app-server initialized a read-only ephemeral thread and used `turn/start` with text and a structured `skill` input for `goef-query-linq` pointing to its discovered path. The default configured model was rejected as unavailable to the account. The advertised catalog default `gpt-6.1-terra` completed the turn, with no tool items, and returned both the exact heading and a sentence absent from the prompt and discovery description.

```text
# GoEF Query Linq

Validate generated plan freshness and parameter rebinding.
```

## Behavior method and scoring

The [36 skill exercises](../../development/SKILL_EVALS.md) and exact UTF-8 entrypoint contents were provided to fresh CLI sessions without the rubric. The three initial batches covered six skill pairs each. Each batch used `codex exec --ephemeral --ignore-user-config --json --sandbox read-only -`, exited 0 and reported no completed tool commands. JSONL did not identify a model version. The lane author scored the returned decisions against the packet. The CLI sessions were independent of that scoring.

The initial content-assisted score was 33/36. E06-P omitted upstream provenance per changed LINQ cell, E10-P omitted provider documentation and MCP proof, and E14-P omitted explicit issue/reproduction linkage. Those three entrypoints were clarified and their six cases repeated in fresh sessions. All rechecks exited 0 without completed tool commands. The latest score is 36/36, made up of 18 positive and 18 negative decisions. It covers the decisions in these scenarios without verifying GoEF features or runtime behavior.

## Evaluated entrypoint digests

| Skill | SHA-256 |
| --- | --- |
| `goef-acceptance` | `cc16e526fce538a3b82f6360c30826e85648524f8a966643837605c4f35c2960` |
| `goef-docs` | `b89849964aa021883bbf2d8040193a1a769e26397fd3bd13933149d45ccbed28` |
| `goef-errors` | `721af3c0d25888c7c2559efd3261bcf024ecc770f55653e48c9e08c61988edba` |
| `goef-gct-testing` | `f5001ba7aca2d52290b5b74c9f982a98af970cdca3af5c8b44de05959241bf9f` |
| `goef-linq-objects` | `63d24b4d70e98096bf704d8fb73ebd28c9a30d3c2927a4dc0db2f93c57c26003` |
| `goef-linq-parity` | `84afcb069e85b3e9ff14322ef39ee42b060b859b9079165fd4ad907e87687e03` |
| `goef-migrations` | `755697f2dd68c651db98a3056c2ec0a6079e58687f46defce72fe60896577560` |
| `goef-model-engine` | `2a76cc294fd4469b55df96d73ec9554db77318a37fd3aed55cea65cfbc6d88c4` |
| `goef-orchestrator` | `6d60c6c40a1ca1af5622f47db73d7c5d307e997d9ded9b8ddb3b6f431d75271a` |
| `goef-provider-mysql` | `61534d09c0126497175b0579d9e133df8f8650de725eac2c6ae7ba95e115c78f` |
| `goef-provider-postgres` | `6998c5a9f2577e3118942c905cc0c435306e2632f65ddc0a2cda023779cf8432` |
| `goef-provider-sqlite` | `d76706dd2df1e0cd3d0a3caa6233ca1fb0e856d55fe9c406bb0e6e2e1973122b` |
| `goef-provider-sqlserver` | `11b8f33ed962895e456c955cbdadecc790c9a361c857524f00db43da0a35d8dc` |
| `goef-query-linq` | `f73df499ba12cda169012754cc0329b31785d8dee0921d4a752e597aae2c8ce9` |
| `goef-security-review` | `fef28901fc1ab96b0a0b1ca27caebcc85309199f68a2d48ddddc380d6238f383` |
| `goef-tracking` | `84c10251f16c7aff971f74f3e14574128bcf280e9587c63409323736bb680fa3` |
| `goef-transactions` | `0a1a7318d852593fdef67da764ec4754726998f98581cff9b9eaacdda7a29f06` |
| `goef-upstream-research` | `0815d13e2652abbae7689bbad00e2b6384d85080c580ee722f19d9fdd8770016` |

## Scored CLI responses

The responses below show the last evaluated decision per case and preserve the response text in case order. The three repeated pairs use their final run.

```text
E01-P. Assign bounded research and fixture work to both provider lanes, with single ownership of the SPI; freeze the SPI and scenario IDs before parallel implementation, and schedule database use so the disposable instance is isolated.

E01-N. Treat the pass at B as insufficient for C; rerun relevant gates at C and obtain fresh independent acceptance for that exact SHA. If accepted locally, record AWAITING_MANUAL_PUBLISH and keep the remote issue open until publication is authorized and remote evidence is verified.

E02-P. Record the issue and scenario IDs, pinned EF Core contract, implementation and positive/negative join tests, plus the affected provider’s exact engine and driver sources and versions; compare observable null-key results, identify the Go behavior and gap, and state any missing evidence or candidate decision.

E02-N. Record the operator as present only at the pinned research commit, with its exact source and license evidence; correct the stable-release claim and seek license and dependency approval before reuse.

E03-P. Trace tags, Fluent options and Database First columns through the validated precedence rules into keys, relationships, provider binding and a canonical snapshot; use reordered column fixtures to prove identical snapshots, the resolved FK override, round trips and relevant validation failures.

E03-N. No: validate contradictory declarations before canonical sorting, and test that conflicting keys or relationships produce the required diagnostic regardless of input order; separately prove valid models produce identical snapshots on repeated runs.

E04-P. Verify the generated plan’s freshness and typed projection with compile-pass and compile-fail cases on the pinned Go toolchain; capture IR and SQL, then execute with successive distinct values and inspect parameter and command traces, results and cache keys for correct rebinding.

E04-N. Do not fetch all rows to apply the callback implicitly; reject the stale generated plan under the freshness contract and return the registered translation or unsupported error for the callback before broad retrieval. Local processing requires an explicit boundary defining the server prefix and client conversion.

E05-P. Observe zero pulls at construction, then exactly two pulls to discard the first value and return the first match; verify no later pull occurs and early termination releases the iterator’s resources, including on a fresh replay.

E05-N. Treat the channel as a one-shot source and define reuse and concurrent consumption explicitly; identical replay requires deliberate buffering or a separate replayable source, with ownership, memory and concurrency behavior specified.

E06-P. Add separate GroupBy records or cells for Objects, Expressions, PostgreSQL and planned SQLite, with exact upstream provenance, support status, and evidence or next proof for each. Link the operator family and execution modes to feature, issue, Gherkin and evidence IDs; require equivalent EF and real-engine fixtures, negative translation and command-count evidence before claiming PostgreSQL SQL parity. Record provider/version differences and update the matrix, docs and examples.

E06-N. Credit only the passing Objects fixture for the local null-key semantics it covers. Remove the four unsubstantiated provider verification claims and mark those cells unknown or unsupported as appropriate; each needs its exact provider version, equivalent EF and real-engine evidence, negative translation and command-count evidence, with any null-join differences recorded.

E07-P. Specify a test in which the second update produces a concurrency conflict: verify the first database write rolls back, a typed concurrency error is returned, and both tracker entries retain the documented state, snapshots, keys and tokens needed for resolution and retry; record the state transitions and scenario link.

E07-N. Reject unconditional AcceptAllChanges: a borrowed transaction’s commit outcome is uncertain, so accepting state may conceal uncommitted changes; require caller-owned reconciliation and evidence of tracker and database outcomes before acceptance or replay.

E08-P. On a disposable, version-identified SQLite database with representative rows, approve and review the rebuild SQL; verify copied values, constraints, trigger and unique index after success, then inject an interruption and verify preserved data, history state and documented recovery.

E08-N. Do not edit the released migration or blindly rerun it; inspect actual schema, history and checksums, reconcile the interrupted implicit-commit operation, and use a reviewed recovery or corrective migration with locking and recorded pre/post evidence.

E09-P. Both contexts must use the caller-owned transaction and its connection; on cancellation, preserve cancellation, clean up operation resources and apply the documented savepoint rollback behavior without committing or disposing the caller’s transaction; verify both contexts’ state and ownership under controlled faults.

E09-N. Do not automatically replay: a failure after COMMIT was sent has an ambiguous outcome even if transport is transient; report it for reconciliation and permit replay only after the contract establishes a safe outcome.

E10-P. Require real round trips for NUMERIC precision, nullable arrays and RETURNING, plus parameterized SQL traces and native SQLSTATE and constraint-name checks; record exact PostgreSQL and pgx versions, versioned documentation links, documentation lookup status, commands, commit and observed results.

E10-N. Reject the proposed retry: a 40001 requires classification by SQLSTATE and transaction phase, followed by transaction-level replay-safety review; collect evidence of connection ownership, cleanup and the outcome of replay, treating a transport failure during commit separately.

E11-P. Use a disposable SQLite file with two identified connections; record SQLite and driver versions, foreign-key PRAGMA per connection, journal mode and busy settings, then separately capture primary and extended codes and phases for contention, BUSY versus LOCKED, and foreign-key failure.

E11-N. Reject the global-setting claim: record and query the foreign-key PRAGMA on each identified connection and demonstrate enforcement on each, including newly acquired pooled connections, with the SQLite and driver versions.

E12-P. On a disposable SQL Server instance, record server and driver versions, database compatibility level, snapshot isolation settings, collation and MARS setting; verify identity values returned by OUTPUT, deterministic pagination order, transaction behavior and redacted parameterized traces.

E12-N. Accept only the limited SQL-text comparison; require real-engine evidence under the supported compatibility level and MARS configuration, including OUTPUT, pagination, isolation and relevant resource behavior, before accepting behavioral compatibility.

E13-P. Plan real-engine tests for decimal and nullable time round trips, quoting, parameter binding and generated IDs; capture server variant/version, InnoDB, driver/version, session sql_mode, charset and collation, plus redacted traces, native errors, cancellation and cleanup. Record capability and mapping evidence with commands and SHA.

E13-N. ROLLBACK alone cannot prove preservation because MySQL DDL may commit implicitly; test the relevant statement classes on a real engine and record history, schema and data state separately, including recovery.

E14-P. Add the mapping only after checking the registry and contract, locating or allocating the stable ID with its owner, and reproducing the wrapped native error with the engine and driver versions; plan tests for `errors.Is` and `errors.As`, the safe constraint identifier, unknown codes and mapping collisions, with links to the issue, Gherkin, reproduction, documentation and exact evidence SHA.

E14-N. Reject the guessed retry classification and raw-message logging; classify the unknown code visibly without a guessed retry action, redact the DSN and server text, and collect sanitized evidence using stable code and fields rather than the localized message.

E15-P. Freeze a scenario for explicit translation failure with the contract owner; define observable failure and zero unfiltered commands, establish behavioral RED or characterization, then require GREEN on the integrated SHA with command-count and traceability evidence.

E15-N. Record the missing driver as an execution blocker and the skipped engine as pending provider conformance; neither is behavioral RED or real-engine evidence.

E16-P. Inventory and update the canonical conversion example and copied excerpts, local and server guides, operator reference, typed error documentation, Godoc and affected skills; verify the boundary, results, prerequisites, links and generated freshness on the integrated SHA.

E16-N. Mark the guide unverified and generated output stale; repair the canonical example, copied excerpts and generator inputs, and record compiler-dependent checks as blocked until the pinned toolchain is available.

E17-P. Plan fictional-tenant tests for parameterized values, tenant filters across joins and subqueries, and separately constrained or quoted sort identifiers; inspect redacted traces and verify explicit opt-out scope.

E17-N. Treat the exposed DSN as a security finding, avoid reproducing it in the review, follow the disclosure policy and request credential remediation; block destructive execution until target ownership, approval, preservation and recovery evidence are confirmed.

E18-P. Issue GO only after independently verifying every applicable mandatory gate on integrated SHA A; provide a gate table with evidence paths, commands, versions, exits and checked SHA, leaving publication and status changes to the authorized owner.

E18-N. NO-GO: Objects evidence does not establish Queryable or provider conformance, SQL goldens do not replace real-engine results, and docs and coverage from SHA B are stale; require the missing gates and current evidence on the integrated SHA.
```
