# Upstream research

[Source lock](lock.json) | [EF Core research index](RESEARCH_INDEX.md) | [LINQ research](LINQ_RESEARCH.md) | [Third-party notices](../../THIRD_PARTY_NOTICES.md) | [Tracking issue #7](https://github.com/igorpooh1978/GoEF/issues/7)

`lock.json` records verified Git identities and license files separately from unresolved proposals. Its verification describes the artifact identity and license reading. Provider compatibility records report the package version range declared at the pinned commit. Engine compatibility and GoEF parity require executable evidence.

EF Core v10.0.15 is the behavior baseline. The .NET runtime v10.0.0 pin supplies LINQ semantics. go-linq v5.0.0 supplies Go iterator examples, and its separately identified research commit can inform possible later additions. Npgsql v10.0.4 declares a compatible EF Core dependency range. Pomelo 9.0.0 serves as a research reference and its EF Core 10 comparison remains blocked. SQLite and SQL Server EF providers are part of the EF Core pin.

To reproduce the tag resolution, run the commands under `sources[].verification.command` and compare the exact commit IDs. For an annotated tag, use the peeled `^{}` reference. For the commit-only research snapshot, inspect the GitHub Git commit object. Inspect each license at its locked path, compare the Git blob ID, and read the notice evidence independently. The two provider metadata files can be checked without restoring or building either provider.

For each feature, read the upstream public contract, implementation and positive and negative tests. Record repository, tag, commit, file, symbol, test and applicability in the [feature registry](../parity/features.json). Keep reference working copies separate and read-only. Preserve version, provider, driver and settings in any later differential run. Handle source updates through a reviewed change to this lock. If a pin or file differs unexpectedly, mark the affected work blocked until resolved.
