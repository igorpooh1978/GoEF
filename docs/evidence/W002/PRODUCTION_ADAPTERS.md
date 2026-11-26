# Production execution adapters

[Issue #13](https://github.com/igorpooh1978/GoEF/issues/13) · [Application scenarios](../../../specs/features/providers/provider-exec-001.feature) · [Driver module](../../../test/conformance/driverlab/README.md) · [Adapter entry points](../../../providers/README.md)

Each provider module implements the frozen execution transport over its pinned native driver. The root Go module shares connection, stream, and type handling through the standard library. The isolated test program uses the same driver factories and engine identity checks as the W002 native characterization. It binds the production adapter to each validated pool.

The application suite checks parameter binding and typed row iteration, unique and foreign-key failures, stream and queued pool cancellation, and an owned transaction with a commit-gated receipt. It checks durable rows through a separate observer or adapter lease. SQL Server code 547 remains unclassified at this boundary because it can represent both foreign-key and check violations. The safe error retains the native code. SQL Server enables savepoint recovery for the successful save path by setting `XACT_ABORT OFF` inside that isolated fixture.

The runner reports coverage for the selected adapter module and the shared transport for each live profile. Each module also has separate API and native error classification tests. The initial executables were characterized by raw Go driver and test-adapter probes. The production suite requires the concrete constructors, and the validator rejects missing, duplicate, or mislabeled results. See the [native driver report](DRIVER_GHERKIN.md) and [coverage policy](../../development/COVERAGE.md).

## Scope

The application suite exercises the SPI v1 command and row transport, using file-backed SQLite. Other native mapping branches may have unit and characterization evidence. A separate feature remains responsible for SQL translation, automatic tracking, and provider-native advanced migration or transaction scenarios. The application receipt exercise coordinates the existing explicit save API. A lost commit response is covered by the [authoritative reconciliation fixtures](COMMIT_RECONCILIATION.md). Process exit during SQLite rebuild is covered by the [migration executor fixture](SQLITE_EXECUTOR_REBUILD.md).

## Tagged adapter previews

GitHub published the four separate adapter previews at module version `v0.1.0-alpha.1` on 2026-11-26. The Git tags use the corresponding `providers/<name>/` prefix. Each local tag and its remote Git and GitHub counterparts agreed on the tag object and resolved to approved source commit `9fcd036a3927b40e26381202c7f1b5aa4fb3d858`. The public Go proxy returned that origin SHA for every module. [Go module tag mapping](https://go.dev/ref/mod#vcs-version)

| Preview | Annotated Git tag object | Go archive checksum | Go manifest checksum |
| --- | --- | --- | --- |
| [PostgreSQL](https://github.com/igorpooh1978/GoEF/releases/tag/providers/postgres/v0.1.0-alpha.1) | `911380410caf15587de602ea1db2a0298357b46c` | `h1:T0hjQGUzka9jr15WvRerucCuvXEdqrY2YoUDZGzKuFQ=` | `h1:PUoj5xUjWx2AeLBnyvraVyMsyMHnOSTvNHz0gDE/Y3w=` |
| [SQLite](https://github.com/igorpooh1978/GoEF/releases/tag/providers/sqlite/v0.1.0-alpha.1) | `f8879aad658225ac44e2ff3a25a5fc3ed32f5ee2` | `h1:OMU3K4nVZrW7R9p/KKVm15G7KN0V9ty5emhFgMq/k3o=` | `h1:pqLTrnM/TiDaZ0CARSD5J10qEOM0BxmAOttEP16w+Qs=` |
| [SQL Server](https://github.com/igorpooh1978/GoEF/releases/tag/providers/sqlserver/v0.1.0-alpha.1) | `11fd5bfe20d2e25da5482bcda0746ac55ee44b59` | `h1:62dDy7CB+zdW/O3JEQ+iVjosymHfRAd5mxVkxmrJiTQ=` | `h1:zuMgfkLGNBIUbBOj/pFbtLys794ItCxFGbTfBpD0Nlw=` |
| [MySQL](https://github.com/igorpooh1978/GoEF/releases/tag/providers/mysql/v0.1.0-alpha.1) | `1002df7f4b605642ca685d250ad41c66304eae54` | `h1:M/gdXcYAZJv1X7w56Wj9eAQIm+M4QDKOk2V7DYAm8yg=` | `h1:gDAEMyccPfZrPf0nNDESt/5Xy5L+saXKMV2/sQnpuhM=` |

Four separate external consumers ran with Go 1.27.4 on Windows/amd64, a fresh module cache, workspace mode disabled, and the public Go proxy and checksum database. For each adapter path, `go mod init`, `go mod download -json <module>@v0.1.0-alpha.1`, `go mod edit -require=<module>@v0.1.0-alpha.1`, `go mod tidy`, `go mod verify`, and `go list -mod=readonly -m all` exited 0. The selected graph had no replacements and contained one adapter, its pinned driver, and its pinned core `v0.0.0-20261126162303-a2b0babac18d`. Each archive included the MIT license.

The compiled consumer asserted that the concrete adapter implements `provider.Backend`. Its tests rejected a nil pool and checked the descriptor of a newly constructed adapter. The SQLite consumer also read a bound integer from a file-backed database through the execution interface. In each module, `go build -mod=readonly -p=2 ./...`, `go vet -mod=readonly -p=2 ./...`, and `go test -mod=readonly -p=2 -count=1 ./...` exited 0.

An independent Ubuntu Linux/amd64 run using Go 1.27.4 installed all four adapters together with the explicit [core preview v0.1.0-alpha.1](https://github.com/igorpooh1978/GoEF/releases/tag/v0.1.0-alpha.1) in another fresh module without replacements. The origin SHA and checksums matched. Module verification, vet, and shuffled tests passed, including a typed row read from file-backed SQLite. The 12-profile execution matrix remains the [real-engine evidence for this exact source](https://github.com/igorpooh1978/GoEF/issues/13#issuecomment-7197938653).
