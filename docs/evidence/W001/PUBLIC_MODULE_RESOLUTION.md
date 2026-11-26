# Public Go module resolution at a published commit

[Issue #24](https://github.com/igorpooh1978/GoEF/issues/24) · [W001 SPI review](SPI_FREEZE.md)

On 2026-11-26, `git ls-remote origin refs/heads/main 'refs/tags/*'` reported [`7a08d90b215d11904109b109c1381ba19c881f35`](https://github.com/igorpooh1978/GoEF/commit/7a08d90b215d11904109b109c1381ba19c881f35) on public `main` with zero tags. The GitHub releases endpoint reported an empty list.

A fresh temporary consumer module outside the GoEF checkout used Go 1.27.4 on Windows/amd64, a new module cache and workspace mode off. The public Go proxy and checksum database were enabled. The selected dependency had no replacement.

| Module download field | Observed value |
| --- | --- |
| Path | `github.com/igorpooh1978/GoEF` |
| Version | `v0.0.0-20261126150900-7a08d90b215d` |
| Origin | Git at `https://github.com/igorpooh1978/GoEF`, hash `7a08d90b215d11904109b109c1381ba19c881f35` |
| Module archive checksum | `h1:RhREJVJfD6Z1vsluN/dDGkiR1zjGXgFRCAPQPBMGqSg=` |
| Module file checksum | `h1:ABeTL0NnLBe42NYSr8liTicW7AUWHsszrL7EzWFpFF0=` |

## Consumer check

From the empty consumer directory, these commands initialized the module and resolved the SHA. Each exited 0.

```powershell
$env:GOWORK = 'off'
$env:GOTOOLCHAIN = 'local'
$env:GOPROXY = 'https://proxy.golang.org'
$env:GOSUMDB = 'sum.golang.org'
$env:GOPRIVATE = ''
$env:GONOPROXY = 'none'
$env:GONOSUMDB = 'none'
$env:GOMODCACHE = Join-Path (Get-Location) 'modcache'
go mod init example.com/goef-issue24-consumer
go mod download -json github.com/igorpooh1978/GoEF@7a08d90b215d11904109b109c1381ba19c881f35
go get github.com/igorpooh1978/GoEF@7a08d90b215d11904109b109c1381ba19c881f35
```

The consumer imported `github.com/igorpooh1978/GoEF/linq/objects` and called `objects.First(objects.Where(slices.Values(values), isEven))` from an ordinary Go source file. `TestFirstEven` checked that `[3, 8, 10]` returns `(8, true)` and `[3, 7]` returns `(0, false)`.

With those files present, `go mod tidy`, `go mod verify`, `go build -mod=readonly ./...`, `go vet -mod=readonly ./...` and `go test -mod=readonly -count=1 -v ./...` all exited 0. `go list -m -json github.com/igorpooh1978/GoEF` confirmed the expected version, Go 1.27.4 and an unset replacement.

## Remaining acceptance

- The MIT license is present at the verified commit and [issue #15](https://github.com/igorpooh1978/GoEF/issues/15) is closed.
- Binding reproducibility and stale-output rejection require the planned generator and the generated-binding fixtures. The checked commit contains compiler fixtures. The generation workflow and changed-capture and stale-output checks are specified in the design. [Generator wave](../../roadmap/MASTER_PLAN.md#9-roadmap-dependency-driven-waves) · [LINQ generation contract](../../architecture/LINQ_EXPRESSIONS.md)
- Release module resolution requires an approved public release revision and a consumer check pinned to that revision. The observed result establishes public module resolution and execution for the checked commit. [Release gates](../../development/RELEASE_GATES.md)
