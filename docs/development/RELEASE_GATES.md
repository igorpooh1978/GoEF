# Acceptance and release gates

| Gate | Bootstrap | Implementation or release |
| --- | --- | --- |
| Work scope | Verified issue, source lock, approved mode, clean baseline | Same, including reviewed design or deviations |
| Documentation and contracts | Validate links, registries, English guides and planned-example labels | Rebuild generated reference twice, compile examples, run documented outcomes |
| Behavior | Gherkin and planned traceability plus compiler probe when applicable | Executable RED or characterization, GREEN, race, fuzz, security, fault and differential evidence |
| Providers | Document exact MCP lookup and version status, and a real test plan | Observe every engine and driver, actual data/history/negative suite, versioned coverage |
| License and independent review | Record source licenses and unresolved GoEF decision | Approved project license, source notice review, no unreviewed high/critical findings |
| Distribution | Local documentation checkpoint | Manual approval, compatible history, reproducible artifacts and final SHA |

For each applicable module run gofmt on changed Go files, then go build ./..., go vet ./..., go test -race -shuffle=on -count=1 ./..., and coverage with the owned package set. Run configured lint, vulnerability, license, generator freshness, docs and real-engine conformance on the same commit. Inspect the staged diff with git diff --cached --check. Save commands, exit codes, versions and artifact links. Keep missing tools and required proof explicitly blocked. Acceptance establishes the tested guarantees in scope.

See [testing](TESTING.md), [coverage](COVERAGE.md), [waves](../roadmap/WAVES.md) and [bootstrap evidence](../evidence/W000/BOOTSTRAP_REPORT.md).
