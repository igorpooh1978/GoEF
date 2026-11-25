# Skill behavior exercises

Use this packet to check decision quality for the 18 [GoEF Skills](SKILLS.md). It contains 36 fictional, read-only decision tasks. These tasks require neither a production database nor publication. A skill evaluator may read the relevant project instructions, skill, linked policies and task. Give the evaluator the scenario prompts without the scoring criteria. Use a clean session for each skill pair if feasible. Ask for a concise decision, the action or evidence to request, and what remains pending. Do not edit code, files, issues or infrastructure during evaluation.

For explicit activation, select the indicated skill with `$skill-name` or give the exact path when evaluating the manual fallback. For an implicit activation claim, use the raw prompt with no skill name and verify which entrypoint the client selected. Score responses using the independently held rubric after the run. Count a case as passed only when the observable decision and critical criterion are both correct. Preserve the unmodified transcript, client/model, explicit or implicit selection, exact SHA and outcome; mark unevaluated cases `NOT RUN`. A source or syntax review alone cannot establish model behavior.

## Scenario prompts

### goef-orchestrator

- E01-P. W006 has two provider tickets ready for research, both touching the same unsettled SPI. You have two write lanes and one disposable database. Plan the next handoff.
- E01-N. A child passed at B. Integration advanced to C, publication is unauthorized and GitHub asks for closure. What states and checks are warranted?

### goef-upstream-research

- E02-P. A provider returns a different result for null keys in a join. Draft the smallest upstream research record you need before changing its behavior.
- E02-N. An operator appears only at the go-linq research commit and a dependency proposal cites the stable release. What evidence and action do you record?

### goef-model-engine

- E03-P. A composite alternate key comes from tags, one FK override from Fluent options, and columns from Database First arrive in varying order. Plan a contract and fixture.
- E03-N. A snapshot golden becomes stable if the generator sorts away two contradictory declarations before validation. Is that acceptable? How do you test?

### goef-query-linq

- E04-P. A query creates a typed projection in an approved generated plan and runs with different bound values on successive executions. Plan proof that the types and parameters stay correct.
- E04-N. A filter uses an arbitrary local callback. Its generator fingerprint is also stale. A teammate proposes applying the callback to all fetched rows. What running behavior is warranted?

### goef-linq-objects

- E05-P. A replayable typed iterator records pulls. An early filter discards one value, then the first match is requested. Design observations for construction, consumption and cleanup.
- E05-N. A channel closes after one pass. A maintainer promises that two enumerations, including concurrent consumers, yield the identical sequence. What contract do you recommend?

### goef-linq-parity

- E06-P. GroupBy joins an Objects implementation, an Expression representation and a PostgreSQL translation. SQLite has only a planned translation. How should the operator record change?
- E06-N. A differential Objects fixture passes with null keys. All four provider cells are marked verified although none ran. Decide what evidence can be credited.

### goef-tracking

- E07-P. Two tracked entities change. An owned SaveChanges transaction reaches a concurrency conflict after writing the first. Define observable state and a test.
- E07-N. A borrowed transaction reports an uncertain commit. A callback calls AcceptAllChanges to make retry clean. How would you review this?

### goef-migrations

- E08-P. An approved SQLite schema change needs a table rebuild with a trigger and a unique index. Describe the isolated acceptance run for success and interruption.
- E08-N. MySQL drops the connection between implicit-commit DDL and the journal record. The operator plans to edit the released migration and rerun. What path is justified?

### goef-transactions

- E09-P. Two contexts borrow a caller-owned transaction. One operation cancels after creating a savepoint. State ownership, connection and cleanup decisions.
- E09-N. The connection fails after the client sends COMMIT. A retry classifier labels the network failure transient. Should the ORM repeat the whole operation automatically?

### goef-provider-postgres

- E10-P. A pgx adapter adds NUMERIC, nullable arrays and RETURNING support. Define version, round-trip and native-error evidence.
- E10-N. A 40001 occurs during a transaction. The adapter retries only the last statement on a fresh pooled connection. Review the proposed handling.

### goef-provider-sqlite

- E11-P. A test needs to distinguish transaction contention across connections from foreign-key failure. Identify setup and observed codes/settings.
- E11-N. A test issues PRAGMA foreign_keys on one pool connection, tests on another, and claims the setting is global. What proof is missing?

### goef-provider-sqlserver

- E12-P. A query uses OUTPUT identity values and paginates under snapshot isolation. Specify a real engine evidence row and relevant settings.
- E12-N. SQL Server server version matches CI, but the database compatibility level and MARS settings differ. CI compares SQL text only. What can be accepted?

### goef-provider-mysql

- E13-P. MySQL stores decimal and nullable time values with a configured session collation and sql_mode. Plan capability and error tests.
- E13-N. A test relies on wrapping MySQL DDL and history in a transaction, with ROLLBACK proving preservation. What is the test gap?

### goef-errors

- E14-P. Wrapping a native driver error still exposes unique violation and a safe constraint identifier. Add a stable error mapping and plan tests.
- E14-N. An unknown native code has a similar localized message and includes a DSN. The translator picks retryable and logs the raw message. Review it.

### goef-gct-testing

- E15-P. Define a Gherkin/RED/GREEN handoff for explicit translation failure, demonstrating that no unfiltered command is sent.
- E15-N. The test harness cannot load its database driver. CI also skips a required engine. The author calls that RED and provider conformance. What do you record?

### goef-docs

- E16-P. The explicit client/server conversion API changes. Map the example, reference, error, Godoc and skill changes in the same issue.
- E16-N. A copied Markdown sample uses a method renamed in Go, and generated output is older than implementation. The pinned compiler is unavailable. What status is honest?

### goef-security-review

- E17-P. A tenant-filtered query also accepts a caller sort field. Draft defensive checks and fixture observations for trust boundaries.
- E17-N. A migration plan exposes a real DSN in a review artifact and previews a destructive action without confirmed target ownership. How do you handle review?

### goef-acceptance

- E18-P. A candidate has independently reviewed integrated SHA A, all applicable real provider gates, source and issue links, checked docs and evidence. Define how to issue the decision.
- E18-N. GoLINQ Objects passes, Queryable has SQL goldens only, and current docs and coverage reports were generated at previous SHA B. Can the candidate receive GO?

## Scoring rubric

| Cases | Positive decision | Negative decision |
| --- | --- | --- |
| E01-P / E01-N | Assign isolated research, single-owner SPI, freeze contracts before parallel implementation and plan the shared database constraint. | Require checks/review at C and a verifiable remote SHA before closure. Use AWAITING_MANUAL_PUBLISH only for locally accepted work. |
| E02-P / E02-N | Require exact upstream API, implementation, tests and provider/driver version references, plus issue and parity gap. | Keep research and stable evidence separate, identify the proper provenance and require license and dependency approval for reuse. |
| E03-P / E03-N | Cover defined precedence, key order and normalized round-trip/deterministic snapshots. | Require a documented contradiction diagnostic or explicit precedence and a negative fixture. Stable output alone is insufficient. |
| E04-P / E04-N | Prove compile pass/fail, stale-plan detection and parameter rebinding, plus matching portable and provider results. | Reject stale generation and unsupported SQL before any fetching, using the applicable typed failure. Permit local filtering only after an explicit client boundary. |
| E05-P / E05-N | Require zero construction pulls, termination at the first match and cleanup if owned. | Distinguish a one-shot source from a replayable one. State ownership, cancellation and reuse limits without adding a SQL dependency. |
| E06-P / E06-N | Record separate linked execution modes and per-provider/version verification state with exact provenance and gaps. | Reject unsupported parity credit, identify provider null semantics and request actual conformance or explicit unverified cells. |
| E07-P / E07-N | Exercise row rollback and both entries' tracker state, keys/tokens and documented conflict handling. | Protect external ownership and do not accept tracker state prematurely or blindly replay. |
| E08-P / E08-N | Verify copied row values, trigger/index preservation, history and recovery with failure injection on the selected SQLite version. | Preserve the released migration. Detect actual schema/history state and use the documented repair/recovery path and new forward migration if needed. |
| E09-P / E09-N | Keep transactional work on the same connection, use a supported savepoint, and leave caller-owned finalization with the caller. | Classify unknown commit outcome and require the documented reconciliation path. Do not replay solely based on the transport classification. |
| E10-P / E10-N | Require versioned docs/MCP, real pgx round trips, typed binding, result cleanup and capability evidence. | Reject statement-level replay and broken connection affinity. Use operation phase and replay rules for fresh whole-unit retry, when safe. |
| E11-P / E11-N | Use appropriate disposable file, controlled connections/PRAGMAs, exact version and primary/extended native codes. | Require per-connection constraint enforcement and validated setup on the actual SQLite driver. |
| E12-P / E12-N | Record engine and compatibility versions, driver and isolation settings, ordering and returned values from real execution. | Keep runtime conformance pending, check compatibility and MARS-sensitive capability behavior on relevant settings. |
| E13-P / E13-N | Capture selected MySQL/InnoDB and driver versions, session settings, exact round trips and native error phases. | Check statement-class implicit commits in the real engine and verify history, schema and restart recovery independently. |
| E14-P / E14-N | Use the existing/stable ID if applicable, typed errors, observed native fields, version/phase and linked reproduction. | Redact the DSN and use unknown-code policy, with no guessed retry from a localized message. |
| E15-P / E15-N | Require stable issue/scenario IDs and failing observable assertion, command counts/parameters and integrated evidence. | Treat startup as a blocker and the skipped engine gate as unproven. No behavioral RED or conformance credit. |
| E16-P / E16-N | Update canonical example and linked source/registry/boundary/docs, expected results and caller error handling. | Repair or mark proposed/pending, record unavailable compile evidence and refresh generated output from its source when possible. |
| E17-P / E17-N | Prove tenant isolation across composition and independently validate/quote identifiers while binding values. | Protect or redact the disclosure, require authorized target/guard and isolated preservation/recovery evidence. |
| E18-P / E18-N | Tie each applicable gate to evidence at A, issue a bounded decision and route closures/publication to the owner. | Keep query conformance, docs and coverage gates unverified; return NO-GO or partial with precise reasons. |
