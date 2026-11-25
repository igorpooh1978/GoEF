# Typed GoLINQ compiler fixture

On 2026-11-25, Go 1.27.4 accepted an external caller's typed filter and type-changing projection through an ordinary compiler interface. It rejected four invalid caller programs. Distinguishing two aliases of the same Go type required a runtime scope check. [Issue #17](https://github.com/igorpooh1978/GoEF/issues/17) · [W001 architecture review](https://github.com/igorpooh1978/GoEF/issues/12)

The [isolated module](../../../test/architecture/linq-compile/go.mod) starts from repository commit `c789602e46c7063e6c0f1ddbfec68bc22c2dbb75`. Its tested Git subtree is `2002bcd05ea5cbd745d969ea4b7724ceed28e9f9`. Run `git rev-parse HEAD:test/architecture/linq-compile` on an integrated checkout to compare the fixture independently of the integration commit.

## What passed

- **Typed composition.** Two filtered branches retain different typed parameter slots. The compiler infers the named result type from a field descriptor. Assignments to `Query[orderID]` and `Query[amount]` compile. An external recorder implements `Compile(Plan) error`, receives each branch's expected ordered steps and can edit its copy without changing subsequent compilations. The root, shared prefix and intermediate filters remain usable. [Facade](../../../test/architecture/linq-compile/query.go) · [TestProjectedBranches](../../../test/architecture/linq-compile/query_test.go)
- **Alias identity.** Two roots of type `Query[order]` compile with the same Go signatures. A filter or projection using the other root's field reports a typed `ScopeError` with the operator and field before any compiler invocation. The same check rejects a previous row's field after projecting to the same Go type. A downstream projection preserves the first binding error. [TestForeignAlias](../../../test/architecture/linq-compile/query_test.go)
- **Negative controls.** With the scope comparison omitted, `go test -count=1 -run TestForeignAlias -v ./...` exited 1. The foreign filter, foreign projection and previous-row cases each delivered one invalid plan. Restoring the comparison made the three cases pass. Allowing a plain slice append to reuse the shared prefix's backing array made `go test -count=1 -run TestProjectedBranches ./...` exit 1. The left filter and projection both received the right branch's slot. Restoring isolated appends made the final suite pass.

Each command below ran from the fixture module and exited 1 as expected. The [rejection harness](../../../test/architecture/linq-compile/rejection_test.go) checks the compiler exit status, the offending source filename and both sides of each type mismatch.

| Caller program | Observed diagnostic |
| --- | --- |
| [`go build ./testdata/foreign_owner`](../../../test/architecture/linq-compile/testdata/foreign_owner/invalid.go) | `Predicate[invoice]` cannot serve as `Predicate[order]` in `Where`. |
| [`go build ./testdata/slot_type`](../../../test/architecture/linq-compile/testdata/slot_type/invalid.go) | `Slot[quantity]` does not match the inferred `Slot[amount]`, although both named types have underlying type `int64`. |
| [`go build ./testdata/result_type`](../../../test/architecture/linq-compile/testdata/result_type/invalid.go) | `Query[orderID]` cannot be assigned to `Query[int]`. |
| [`go build ./testdata/local_callback`](../../../test/architecture/linq-compile/testdata/local_callback/invalid.go) | `func(order) bool` cannot serve as `Predicate[order]` in `Where`. |

## Reproduce

Start in the repository root using PowerShell.

```powershell
Set-Location test/architecture/linq-compile
$env:GOWORK = 'off'
$env:GOPROXY = 'off'
$env:GOTOOLCHAIN = 'local'
go version
gofmt -l .
go list -m all
go build ./...
go vet ./...
go test -count=1 -shuffle=1795629201175056800 -cover ./...
go test -race -count=1 -shuffle=on ./...
```

| Observed command | Exit | Result |
| --- | ---: | --- |
| `go version` | 0 | `go1.27.4 windows/amd64` |
| `gofmt -w .`, then `gofmt -l .` | 0 each | No files listed after formatting. |
| `go list -m all` | 0 | Only `example.invalid/goef/linq-compile`. |
| `go build ./...`, `go vet ./...` | 0 each | Package builds and vet passes. |
| `go test -count=1 -shuffle=on -cover -v ./...` | 0 | Three tests, 15 named subtests, 95.5% fixture statement coverage. The fixed-seed command above reproduced the result. |
| `go test -race -count=1 -shuffle=on ./...` | 2 | The integrated Windows recheck stopped before tests with the host's `CGO_ENABLED=0`. A second run with `$env:CGO_ENABLED = '1'` exited 1 because the configured `gcc` was absent from PATH. Race evidence needs a compatible runner. |

## Scope of the proposal

The .NET reference types a `Where` by its input and a `Select` by its result. The upstream operator tests also exercise returned values and argument validation. This Go experiment independently exercises the facade and provider dispatch only. Its symbolic fields stand in for model-generated descriptors. The released API, model binding, source spans, multi-alias joins and parameter rebinding remain open. [Pinned Queryable source](https://github.com/dotnet/runtime/blob/60629d14374c56f1cb51819049ad1fa529307f8d/src/libraries/System.Linq.Queryable/src/System/Linq/Queryable.cs) · [Where tests](https://github.com/dotnet/runtime/blob/60629d14374c56f1cb51819049ad1fa529307f8d/src/libraries/System.Linq.Queryable/tests/WhereTests.cs) · [Select tests](https://github.com/dotnet/runtime/blob/60629d14374c56f1cb51819049ad1fa529307f8d/src/libraries/System.Linq.Queryable/tests/SelectTests.cs) · [Source and license lock](../../upstream/lock.json)

The tests cover the alias rejection and branching portions of `LINQ-EXPRESSION-001-S01` and `LINQ-QUERYABLE-001-S01`. Source-position diagnostics, actual command counts, Objects behavior, generated code and all four SQL suites remain unverified by this module. Independent acceptance belongs to the integration review. [Expression specifications](../../../specs/features/linq/expressions.feature) · [Queryable specifications](../../../specs/features/linq/queryable.feature) · [W000 language evidence](../W000/GO_FEASIBILITY.md)
