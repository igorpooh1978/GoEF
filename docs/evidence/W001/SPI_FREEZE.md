# W001 architecture review and SPI freeze

[Issue #12](https://github.com/igorpooh1978/GoEF/issues/12). Reviewed baseline `8bb7030d6c2b4c1a151d669f48e6df2e681aed6a`. W001 approves the core module identity, dependency direction and provider execution SPI v1. The [provider package](../../../provider/transport.go) contains the frozen Go declarations. The [ADR](../../adr/0002-module-and-provider-boundaries.md) and [contract](../../architecture/PROVIDER_SPI.md) record responsibilities and compatibility.

| Review gate | Decision and proof |
| --- | --- |
| Module boundary | Core uses the exact import spelling `github.com/igorpooh1978/GoEF` and the standard library. The external fixture imports the exported SPI from a separate module. The prior four-module experiment exercises the Objects package graph, a wrong-case path and a forbidden internal import. |
| Versions and ownership | Check the static adapter descriptor before acquiring a connection. One lease owns the physical connection until release. Rows stay on that connection. Unknown or unsafe connections are discarded. Changes to required methods or semantics require a new SPI generation. |
| Input and stream behavior | The external fixture exercises rejection of malformed bindings before dispatch, retention of a private binary copy, a cancelled stream, and a known zero versus unavailable affected count. The Query context governs the whole stream. Cleanup uses a separate, bounded context when needed. |
| Extensions | Compiler IR, model codecs, transaction creation/savepoints, capability probing and migration operations remain separate extensions in their implementing waves. Their ownership and outcome rules are already reviewed. Future signatures require evidence from concrete adapters before freezing. |

The Windows checks use Go 1.27.4. The Linux/amd64 checks use the local Go 1.27.4 container, a read-only source mount, no container networking, and `GOTOOLCHAIN=local GOWORK=off GOPROXY=off`. Run `go mod tidy -diff`, `go build ./...`, `go vet ./...` and `go test -race -count=1 -shuffle=on ./...` separately in the repository root and the external provider SPI, W000 language and W001 LINQ fixture modules.

The former Linux race gap for the architecture fixtures is resolved. The authority to run a CLI lab against engines establishes neither GoEF adapter semantics nor generated binding freshness. [Generated binding and release resolution follow-up #24](https://github.com/igorpooh1978/GoEF/issues/24) retains these gates beyond the scoped architecture review. The [W002 lab](../W002/LAB_SMOKE.md) records engine checks separately.

The verified provider and external-fixture trees are `c335b07d4a32061204c22af28efdafb8d4ffb49d` and `1e8d080adfc4e4d6e70d116856b0e41a25c80867`. Compare them using `git rev-parse HEAD:provider HEAD:test/architecture/provider-spi`.
