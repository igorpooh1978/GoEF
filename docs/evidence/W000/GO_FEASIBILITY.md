# Go 1.27.4 language feasibility

On November 25, 2026, an independent fixture compiled a chained generic query from an external package, observed the plan at a non-generic interface, and asserted five separate compiler rejections. This is a language and import-boundary experiment, with a synthetic plan of two field IDs. Database execution, a production model, generators, drivers and SQL translation remain unimplemented. [W000 issue #3](https://github.com/igorpooh1978/GoEF/issues/3) · [W001 review #12](https://github.com/igorpooh1978/GoEF/issues/12)

**Starting revision:** `2a886c78e622f583fa2980ea0a61a7e328a40e28`. The [fixture module](../../../test/bootstrap/go-feasibility/go.mod) uses a reserved `example.invalid` name and requires no external modules. [Facade declarations](../../../test/bootstrap/go-feasibility/shape.go) · [External caller](../../../test/bootstrap/go-feasibility/shape_test.go) · [Compiler rejection harness](../../../test/bootstrap/go-feasibility/rejection_test.go)

## Reproduce

Run the following Python snippet from the repository root. The Go test invokes the compiler separately for each negative fixture, checks its exit status and matches a diagnostic specific to that case.

```python
import os
import subprocess

env = dict(os.environ, GOTOOLCHAIN="local", GOWORK="off", GOPROXY="off")
directory = "test/bootstrap/go-feasibility"
for arguments in (
    ["version"],
    ["test", "-count=1", "-shuffle=on", "-v", "./..."],
    ["vet", "./..."],
):
    subprocess.run(["go", *arguments], cwd=directory, env=env, check=True)
```

| Command in the fixture directory | Exit | Observed result |
| --- | --- | --- |
| `go version` | 0 | `go version go1.27.4 windows/amd64` |
| `go test -count=1 -shuffle=on -v ./...` | 0 | `TestFacade` and all five `TestCompilerRejections` subtests passed. |
| `go vet ./...` | 0 | No diagnostics. |
| `go list -m all` | 0 | Only `example.invalid/goef/go-feasibility`. |
| `go mod tidy -diff` | 0 | No diff. |
| `go mod verify` | 0 | All modules verified. |
| `govulncheck ./...` | 0 | Local govulncheck v1.9.0 reported no vulnerabilities for the fixture. |
| `gofmt -l .` | 0 | No files listed. The syntax-negative interface declaration lives in a text fixture which the test sends directly to the Go compiler. |
| `go test -race -shuffle=on ./...` | 2 | **Blocked.** Host has `CGO_ENABLED=0`. Retried with `CGO_ENABLED=1` and the same command, exit 1 because the configured C compiler `gcc` was absent from PATH. Use a runner with cgo and a supported compiler for race evidence. |

## Individual compile-fail controls

These command exits were also observed directly. Paths and line numbers in diagnostics depend on the working directory and OS.

| Command | Required exit | Observed diagnostic excerpt |
| --- | --- | --- |
| `go tool compile testdata/interface_method/invalid.go.txt` | 2 | `interface method must have no type parameters` |
| `go build ./testdata/satisfaction` | 1 | `wrong type for method Write`, with `have Write[T any](T)` and `want Write(string)` |
| `go build ./testdata/recursive` | 1 | `instantiation cycle`, with `T instantiated as []T` |
| `go build ./testdata/owner` | 1 | The compiler rejects `Field[invoice, bool]` in place of `Field[account, bool]`. |
| `go build ./testdata/result` | 1 | The compiler rejects assigning `Query[string]` to `Query[int]`. |

The passing path instantiates `Context.Set[account]()`, chains `Where(active).Select(id)`, and assigns the inferred result to `Query[accountID]`. A top-level `Nest` produces `Query[[]accountID]` and dispatches the preserved plan through `Compiler.Compile(Plan) Plan`. A second compile observes the original empty plan. The compiler results agree with the Go 1.27 interface restrictions. Type ownership alone cannot distinguish two aliases of one table. Generator binding, real-provider semantics and resource cleanup each need dedicated evidence. [Official release notes](https://go.dev/doc/go1.27#language) · [Generics decision](../../adr/0003-go-linq-and-generics.md)
