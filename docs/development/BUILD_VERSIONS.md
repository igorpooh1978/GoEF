# Build and preview versions

[Issue #38](https://github.com/igorpooh1978/GoEF/issues/38) tracks build identification and preparation of the next preview.

## Identify a build

From a Git checkout, inspect the generator with Go 1.27.4 and explicitly enable Git stamping.

```sh
go run -buildvcs=true ./cmd/goef-linqgen -version
```

A clean checkout of a tagged commit reports its module version, full Git revision and `dirty=false`. Between tags, Go supplies a pseudo-version containing the commit time and abbreviated revision. A checkout with uncommitted changes also reports `dirty=true`. An installed generator reports the module version selected by `go install ...@<version>`. The Git fields appear when the Go toolchain records them. A local module replacement with no recorded version reports `(devel)`.

When using `go build -o <binary>`, put the output outside the checkout to keep Git status unchanged. Inspect the resulting binary with `go version -m <binary>`. Inspect the selected modules in a consuming application with `go list -m all` or in its binary with `go version -m <binary>`. Version reporting uses Go's embedded metadata and leaves generated bindings independent of the build revision. [Go module versions](https://go.dev/ref/mod#versions) · [Go build information](https://pkg.go.dev/runtime/debug@go1.27.4#ReadBuildInfo)

## Prepare an immutable preview

The candidate series for the adoption follow-up is `v0.2.0-alpha.1`. It remains a prerelease while wider query parity, provider maturity and production support are in progress. Increment the prerelease number for subsequent candidates in the series. A Git commit hash or its Go pseudo-version identifies an untagged build.

| Go module suffix | Candidate Git tag |
| --- | --- |
| Root module | `v0.2.0-alpha.1` |
| `providers/postgres` | `providers/postgres/v0.2.0-alpha.1` |
| `providers/sqlite` | `providers/sqlite/v0.2.0-alpha.1` |
| `providers/sqlserver` | `providers/sqlserver/v0.2.0-alpha.1` |
| `providers/mysql` | `providers/mysql/v0.2.0-alpha.1` |

1. Complete the [release gates](RELEASE_GATES.md) for the exact core candidate, including independent review and clean source. Publish the approved root tag under the distribution gate. Keep existing tags immutable.
2. Verify the core in a fresh consumer with the Go proxy and checksum database, substituting the approved full commit SHA.

   ```sh
   python -B test/architecture/public-module/verify.py <core-commit> --version v0.2.0-alpha.1 --generated-bindings --generator-version
   ```

3. Update each provider module's core requirement to the verified core tag. Provider checkout builds use a local core replacement, so verify the public requirement and both dependency sums in a clean consumer without replacements. Keep the provider release commit and observed checksums in the release evidence.
4. Complete the provider gates and publish each prefixed tag at its reviewed adapter commit. Resolve each adapter version independently in a fresh consumer. Check that `go mod download -json` reports the expected adapter tag commit and both checksums, then use `go list -m -json` to confirm the selected adapter and core versions and the absence of replacements. Repeat the corresponding driver quickstart. Update installation examples to the verified versions after publication. [Adapter preview evidence](../evidence/W002/PRODUCTION_ADAPTERS.md#tagged-adapter-previews)

Go selects root and provider versions using their distinct tag names. A root tag alone leaves adapter installations at their previous tagged versions. [Go module-to-commit mapping](https://go.dev/ref/mod#vcs-version)
