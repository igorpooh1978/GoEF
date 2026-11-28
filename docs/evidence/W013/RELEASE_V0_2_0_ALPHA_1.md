# GoEF v0.2.0-alpha.1 release verification

The core and four provider releases were published as GitHub prereleases on 2026-11-28. The adoption follow-up branch was fast-forwarded into `main`. The core tag uses the exact reviewed runtime and generator build commit `493db45c51d9a2acd1998fe413e1d9ae241aff2a`. The provider tags use `fe6efee80f9c4b1bdc5eb472a31e588354188d00`, which pins all four adapters to the published core tag. No runtime code, third-party driver version, or toolchain changed in the provider pin commit. [Issue #38](https://github.com/igorpooh1978/GoEF/issues/38) · [Integrated runtime evidence](../W002/ADOPTION_FOLLOW_UP.md)

## Public GitHub releases

| Module | GitHub prerelease | Tag commit |
| --- | --- | --- |
| Core | [v0.2.0-alpha.1](https://github.com/igorpooh1978/GoEF/releases/tag/v0.2.0-alpha.1) | `493db45c51d9a2acd1998fe413e1d9ae241aff2a` |
| PostgreSQL | [providers/postgres/v0.2.0-alpha.1](https://github.com/igorpooh1978/GoEF/releases/tag/providers/postgres/v0.2.0-alpha.1) | `fe6efee80f9c4b1bdc5eb472a31e588354188d00` |
| SQLite | [providers/sqlite/v0.2.0-alpha.1](https://github.com/igorpooh1978/GoEF/releases/tag/providers/sqlite/v0.2.0-alpha.1) | `fe6efee80f9c4b1bdc5eb472a31e588354188d00` |
| MySQL | [providers/mysql/v0.2.0-alpha.1](https://github.com/igorpooh1978/GoEF/releases/tag/providers/mysql/v0.2.0-alpha.1) | `fe6efee80f9c4b1bdc5eb472a31e588354188d00` |
| SQL Server | [providers/sqlserver/v0.2.0-alpha.1](https://github.com/igorpooh1978/GoEF/releases/tag/providers/sqlserver/v0.2.0-alpha.1) | `fe6efee80f9c4b1bdc5eb472a31e588354188d00` |

`git ls-remote` confirmed the tag commits. `gh release view` confirmed that all five releases were published as prereleases. The Windows/amd64 and Linux/amd64 generator binaries were downloaded back from the core release and independently matched its `SHA256SUMS`.

## External module resolution

On Go 1.27.4, the core verification ran in a fresh application module and cache using the public Go proxy and checksum database. It exited 0 after verifying the archive origin, building and running the consumer, generating the same bindings twice, rejecting stale bindings, and installing a generator that reported `goef-linqgen v0.2.0-alpha.1`.

```sh
python -B test/architecture/public-module/verify.py 493db45c51d9a2acd1998fe413e1d9ae241aff2a --version v0.2.0-alpha.1 --generated-bindings --generator-version
```

All four provider modules passed build, vet, and shuffled unit tests after the pin change. Separate temporary copies of the provider modules removed the checkout's relative core replacement, selected the public core tag, and passed module verification, build, vet, and tests. Linux/amd64 race tests passed for the core and all four provider modules.

`go mod download -json <module>@v0.2.0-alpha.1` confirmed the public origin and matching archive and manifest digests for each module with `GOSUMDB=sum.golang.org`. An initial proxy-only lookup for SQL Server returned a cached upstream GitHub connection failure. The normal Go configuration `GOPROXY=https://proxy.golang.org,direct` resolved the same public tag and passed its signed checksum verification in a fresh module cache.

| Module | Archive checksum | `go.mod` checksum |
| --- | --- | --- |
| Core | `h1:71WixM169tQUjCBsf4PjDY6D8fYQIj1i21ZoJdU51ys=` | `h1:ABeTL0NnLBe42NYSr8liTicW7AUWHsszrL7EzWFpFF0=` |
| PostgreSQL | `h1:Ka4c61PqEgl5sZ7RS6psXy0E+F5pAuITpuhvmPhMc7o=` | `h1:oSqAR3iweFEfoV4dVykP+1Vwu0R+XfYGCQl2hIjhFaI=` |
| SQLite | `h1:WITmpE0zeOduLz+fUKP6m7XnOvI0siCQElGDwin0KfE=` | `h1:ZU/pUvG1i7PFlIGwCx05vSWJ1ZxXMPzq/Jimk8z3Om8=` |
| MySQL | `h1:nUbxMD1r9Z59TbDESmLWUNz5dsHjqu+LWkQbN3t5U9E=` | `h1:+EediZMl5OH0jhwXRyCnEICST7PnpCr8y5eWSLrLr5I=` |
| SQL Server | `h1:TOTa28bxcgYLtUCYdkP71wU40KZGO5457DLpXnj1VoE=` | `h1:mS0NIkPNj9+PC+WPyDtnPwckfmEIVvAzV9tlj0ILI4w=` |

A single fresh Windows/amd64 Go 1.27.4 application selected GoEF core and all four adapters at `v0.2.0-alpha.1` with `GOWORK=off` and no replacement directives. The check initialized a module, downloaded the exact versions, pinned them with `go mod edit`, and verified selections with `go list -mod=readonly -m all`. `go mod tidy`, `go mod verify`, `go build -mod=readonly ./...`, `go vet -mod=readonly ./...`, and `go test -mod=readonly -count=1 -v ./...` all exited 0. The tests exercised a reviewed SQLite read with EOF and physical reuse and PostgreSQL typed compilation and preflight. The fixture compiled the four adapter constructors and the PostgreSQL migration and transaction entry points. A fresh versioned generator install also exited 0 and reported `v0.2.0-alpha.1`.

## Published quickstart execution

An independent check created one isolated application per database. Each pinned only the published core and its adapter, confirmed the module digests and origin commits, and passed `go mod tidy`, `go mod verify`, `go list -mod=readonly -m all`, `go vet -mod=readonly .`, and `go build -mod=readonly`. The server examples ran in pinned Linux/amd64 containers with isolated fixtures. SQLite used a temporary file on Windows/amd64. All four commands executed successfully and the isolated processes and databases were cleaned up. [Quickstarts and source](../../../examples/README.md#database-quickstarts)

| Engine | Driver | Output | Exit |
| --- | --- | --- | ---: |
| PostgreSQL 18.7 | pgx v5.13.0 | `42` | 0 |
| SQLite 3.51.3 | modernc.org/sqlite v1.47.0 | `hello from SQLite` | 0 |
| MySQL 8.4.13.1 | go-sql-driver/mysql v1.10.0 | `42` | 0 |
| SQL Server 2022 Developer CU28, 16.0.4325.4, compatibility level 160 | go-mssqldb v1.11.0 | `42` | 0 |

The SQL Server example used TCP in the isolated Developer container. The LocalDB named-pipe walkthrough remains a separate runtime configuration.
