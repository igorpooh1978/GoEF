# ADR 0002 — Module and provider boundaries

**Status:** Proposed for W001. W000 records the repository identity, dependency constraints and the compiler experiment. The first production module, public package paths, compatibility policy and dependency pins require review in [issue #12](https://github.com/igorpooh1978/GoEF/issues/12). [W000 issue #3](https://github.com/igorpooh1978/GoEF/issues/3)

## Context

GoEF must support an application importing only in-memory LINQ, an application using a single relational driver, and independent provider authors. At baseline `2a886c78e622f583fa2980ea0a61a7e328a40e28` the public repository had a README and no production Go module. `git remote get-url origin` and the GitHub repository API identify `igorpooh1978/GoEF` with the same case. The candidate root path is **`github.com/igorpooh1978/GoEF`**, with the lower-case Go package name `goef`. Case is significant in module identity. The Go proxy escapes the final component as `!go!e!f` for transport. Source imports would retain `GoEF`. [Repository](https://github.com/igorpooh1978/GoEF) · [Go module paths](https://go.dev/ref/mod#module-path) · [Module proxy case encoding](https://go.dev/ref/mod#goproxy-protocol)

## Decision under review

Start with one portable core module containing independently importable `linq/objects`, symbolic expressions, the queryable facade, model and migration APIs and a small public SPI. Keep Objects usable with the standard library and native iterators. Keep translation, tracking, materialization and physical planning internal. Expose a minimal versioned relational plan view when a provider needs to consume one. A contract must never require an outside provider to name a core internal type.

Give each concrete relational provider its own module, depending on core and its selected driver. Keep CLI, generator and design-time packages outside the core runtime dependency closure. Application constructors select concrete providers explicitly. A driver upgrade in one provider should leave the core and the other providers' module graphs unchanged. A major compatibility break follows Go module version rules and a documented provider compatibility matrix.

## Alternatives and cost

| Option | Tradeoff |
| --- | --- |
| One module including all drivers | Simplifies local versioning but ties independent consumers to a module requiring every driver. |
| Separate module for every public package | Isolates dependencies but creates coordination and version skew between tightly coupled query and model changes. A future independent Objects release may justify extracting that package. |
| Lowercase import alias or vanity domain | Needs an explicit owner policy, metadata/redirect control and one canonical identity before first publication. Mixing case introduces distinct import paths. |
| One driver-shaped SPI | Speeds integration of the first driver but exposes its connection, SQL and type assumptions to downstream providers. Use small consumer-owned interfaces reviewed against two independent cases. |

## Acceptance

Build a consumer in a separate module using only Objects, another using one concrete provider and an out-of-tree adapter implementing ordinary narrow contracts. Inspect `go list -deps` and `go list -m all` under isolated module resolution for the exact forbidden edges. Compile from a clean module cache with the chosen canonical case. Verify version mismatch diagnostics and document any coordinated release requirements for core/provider modules. W000's `example.invalid` fixture demonstrates language mechanics independently of the eventual module name. [Provider SPI](../architecture/PROVIDER_SPI.md) · [Language evidence](../evidence/W000/GO_FEASIBILITY.md)
