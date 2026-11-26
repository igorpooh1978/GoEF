# Generated LINQ binding checks

[Issue #24](https://github.com/igorpooh1978/GoEF/issues/24) · [Generated binding guide](../../user/LINQ_GENERATED.md) · [LINQ-EXPRESSION-001-S03](../../../specs/features/linq/expressions.feature)

Generator implementation revision `fc39279fa8cc7450bd8f717c64694423af1bb924`. Filename portability correction and generator verification revision `d951295d364fe1140777b39c70432caacbb844a8`. Baseline `ba3d033a155de704f8dfa0e442710979b36f9860`.

## Source behavior

The pinned .NET v10.0.0 Queryable `Where` builds a provider expression and its tests validate the source and predicate. The Expressions equality factory distinguishes operator selection and lifting. EF Core v10.0.15 extracts parameters before obtaining the compiled query, and `Where_simple` compares a customer city with a value. The pinned go-linq v5.0.0 `Where` applies a local iterator callback. GoEF uses an independently authored marked declaration for a static field equality and a single scalar projection. The generated shape contains declaration names and receives the invocation argument separately. [Queryable source](https://github.com/dotnet/runtime/blob/60629d14374c56f1cb51819049ad1fa529307f8d/src/libraries/System.Linq.Queryable/src/System/Linq/Queryable.cs#L48) · [Queryable tests](https://github.com/dotnet/runtime/blob/60629d14374c56f1cb51819049ad1fa529307f8d/src/libraries/System.Linq.Queryable/tests/WhereTests.cs) · [Equality factory](https://github.com/dotnet/runtime/blob/60629d14374c56f1cb51819049ad1fa529307f8d/src/libraries/System.Linq.Expressions/src/System/Linq/Expressions/BinaryExpression.cs) · [EF compiler](https://github.com/dotnet/efcore/blob/b34c0e9d7e768aa808ded44f19e7f871bea6869f/src/EFCore/Query/Internal/QueryCompiler.cs) · [EF city filter](https://github.com/dotnet/efcore/blob/b34c0e9d7e768aa808ded44f19e7f871bea6869f/test/EFCore.Specification.Tests/Query/NorthwindWhereQueryTestBase.cs) · [go-linq local filter](https://github.com/ahmetb/go-linq/blob/6ebc43b7c971a8a4cb9e38742f6cb491d4032cbb/where.go) · [Pinned licenses](../../upstream/lock.json)

## RED and GREEN

- Before implementation, adding the marked customer declaration and its example to the baseline made `go test -count=1 ./examples/linq/generated` exit 1 because `generated.FindCustomer` was undefined. After generation the same example exercised the two independent city arguments and projected `Ada`. [Example](../../../examples/linq/generated/query_test.go)
- Independent review identified that implicit GOOS and GOARCH filename suffixes could restrict the source and generated file to different platforms. `go test -run '^TestPortableGoFileNames$' -count=1 ./cmd/goef-linqgen` on the pre-fix implementation with the new regression test exited 1 for Windows, AMD64, Linux/AMD64 and JS/WASM suffixes. The corrected verifier uses the pinned Go toolchain's filename matcher, and both input and output rejection pass. [Filename checks](../../../cmd/goef-linqgen/main_test.go)
- `TestFreshness` generates two independent output files, compares their bytes and checks missing, manually edited and stale output. A source change from the `Name` projection to `City` restores freshness only after regeneration. Checks leave missing or stale outputs untouched. The unit suite also checks LF and CRLF equivalence and preservation of a hand-written output file. [Freshness checks](../../../cmd/goef-linqgen/main_test.go)
- `TestRejectedDeclarations` diagnoses a valid Go callback used as a predicate at `query.go:7:9`. Other negative cases cover ambient arguments, foreign or promoted fields, methods, type mismatches and unsupported scalars. `TestGeneratedTypesRejectedByCompiler` requires Go compiler exit 1 for the wrong invocation argument, projection result and row owner. [Generator diagnostics](../../../cmd/goef-linqgen/generator_test.go) · [Compile-fail cases](../../../examples/linq/generated/testdata/wrong_types/invalid.go)

## Verification

Executed with Go 1.27.4, on Windows amd64 or Ubuntu Linux amd64 as indicated. Projection and argument values are synthetic.

| Command | Platform | Exit and result |
| --- | --- | --- |
| `gofmt -l` on Go files changed from the baseline, `go build ./...`, `go vet ./...` | Windows | Each 0. No formatting changes. |
| `go test -shuffle=on -count=1 -coverpkg "$owned" -coverprofile "$profile" ./...` | Windows | 0. The owned set is `go list ./...` excluding packages under `/examples/`, with test execution across the root module. The generator covered 177/191 statements (92.7%), the binding package 4/4 (100%), and the root production set 1,189/1,229 (96.7%). |
| `go test -race -shuffle=on -count=1 ./...` | Linux | 0. |
| `go generate ./examples/linq/generated` twice, then `go run ./cmd/goef-linqgen -source examples/linq/generated/query.go -output examples/linq/generated/query_gen.go -check` | Windows | Each 0. Both generated outputs had SHA-256 `482d505c865b22c79372fc73b002196681feca42cd085e6730cef8f960ab59a7`. |
| `go test -run '^Example$' -count=1 -v ./examples/linq/generated` | Windows | 0. The example printed equal shapes, `London Berlin` and `Ada`. |
| `go test -run '^$' -fuzz '^FuzzGenerate$' -fuzztime 5s -parallel 2 ./cmd/goef-linqgen` | Windows | 0. 38,669 fuzz executions. Valid output parsed as Go and was byte-identical for another input path. |
| `govulncheck ./...` | Windows | 0. Scanner v1.9.0 reported no vulnerabilities using the database updated at 2026-11-24 15:56 UTC. |
| `python -B test/bootstrap/verify.py` | Windows | 0. 84 product scenarios and their references checked. |

At the implementation revision a fresh external module using a local replacement and `GOPROXY=off` ran `go generate ./...`, `go build -mod=readonly ./...` and `go run -mod=readonly .`. Each exited 0. A source edit made the documented `go run github.com/igorpooh1978/GoEF/cmd/goef-linqgen ... -check` command exit 1 with a stale diagnostic, and the generated file's hash stayed fixed. After regeneration the program projected `Berlin` from the same customer. The unchanged `test/architecture/linq-compile` module also passed build, vet and shuffled tests at that revision.

The generator evidence covers typed declaration validation, argument rebinding and source freshness for this frontend subset. The [public consumer verifier](../../../test/architecture/public-module/verify.py) is the separate check for release-path resolution. [Query architecture](../../architecture/QUERY_ENGINE.md)

## Coverage follow-up

At test revision `f80d3ab722a83eb5fb81fad4a2c94c0ca7a3acf9`, the generator covered 183/191 statements (95.8%) on both Windows and Ubuntu Linux with Go 1.27.4. On Linux the permission checks ran as a non-root user. They removed read permission from the source and output in turn and confirmed that each file retained its original bytes. Windows exercised the same failure paths with exclusive native file handles. The tests also verify the CLI entry point, an invalid output path, preservation of the previous output after an invalid declaration, and rejection of comparison between an interface-typed field and a scalar argument.

| Command | Platform | Exit and result |
| --- | --- | --- |
| `go test -shuffle=on -count=1 -coverprofile "$profile" ./cmd/goef-linqgen ./examples/linq/generated` | Windows | 0. Unit cases, native file-sharing failures, sample and Go compiler rejections passed. |
| `go test -race -shuffle=on -count=1 -coverprofile "$profile" ./cmd/goef-linqgen ./examples/linq/generated` | Linux | 0. Unit cases, two permission-denied paths, sample and Go compiler rejections passed. |
| `go vet ./cmd/goef-linqgen ./linq/bindings ./examples/linq/generated`, followed by the standalone freshness command above | Windows | Each 0. |
| `go test -run '^$' -fuzz '^FuzzGenerate$' -fuzztime 5s -parallel 2 ./cmd/goef-linqgen` | Windows | 0. 119,918 fuzz executions. |

[Generator command checks](../../../cmd/goef-linqgen/main_test.go) · [Windows file-sharing checks](../../../cmd/goef-linqgen/main_windows_test.go) · [Validation and fuzz cases](../../../cmd/goef-linqgen/generator_test.go)
