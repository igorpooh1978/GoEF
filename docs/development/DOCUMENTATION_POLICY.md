# Documentation policy

Write canonical repository artifacts in English. Update the Gherkin contract, implementation, tests, Godoc, user Markdown, examples, feature and operator registries, capability/error references and affected skills under the same issue. Include documentation impact in every task plan. Obtain an explicit reviewer-approved `Docs impact: none` explanation when a behavior change needs no docs.

## Write for GoEF users

Start each supported guide with the smallest runnable example. State minimum Go, provider, driver and engine versions, prerequisites, dependencies/imports, expected output and a typed failure with caller handling. Explain resource ownership, cancellation, concurrency, security, provider differences, limitations and upgrade behavior. Link a version-controlled example and tests. Use fictional fixtures and disposable local databases or SQLite files. Label any shown SQL with its dialect and parameterization.

Separate bootstrap proposals from verified APIs. Mark snippets `Proposed` until they compile and run at the pinned Go version. State actual provider support and open gaps using the [parity sources](../parity/README.md). Keep object LINQ, expressions and database-backed query instructions discoverable with the [explicit client/server boundary](../user/LINQ_CLIENT_SERVER_BOUNDARY.md). For migrations include model-driven, Go-defined and programmatic paths, safety checks, and the optional CLI workflow.

## Keep references aligned

- [Architecture](../architecture/OVERVIEW.md) describes design and invariants. Use [ADRs](../adr/README.md) for decisions and tradeoffs. Avoid maintaining several copies of a contract.
- The [docs index](../README.md), [user guides](../user/README.md) and [reference](../reference/README.md) provide navigation. Use durable repository-relative links. Put runnable examples in Go source and join published snippets to their source.
- For public packages and exported symbols with public semantics, maintain Godoc that explains ownership, lifecycle, thread safety, cancellation, typed errors and compatibility as relevant. Explain why a complex internal algorithm is correct and what invariants constrain it. Update generators at their source and run the pinned generator before verifying generated docs and code.
- Publish redacted [wave evidence](../evidence/W000/BOOTSTRAP_REPORT.md) with commands, exit codes and exact SHAs. Record the environment needed to repeat a run and where outputs are available. Never include private paths, real DSNs, tokens or customer fixture data.

## Acceptance

Review links and navigation, canonical language, synchronized Godoc and Markdown, example compile and execution, generated-document freshness, scenario-to-test-to-doc traceability and provider/version matrices. Verify the actual API and outputs at the integrated SHA. Record `BLOCKED` or `NOT RUN` and the missing prerequisite for any unavailable check. Independent review includes documentation and examples. Stale or missing required documentation blocks verification and release.
