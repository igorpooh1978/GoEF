# GoEF architecture

W000 design for [issue #3](https://github.com/igorpooh1978/GoEF/issues/3). The only implemented example in this contract is a standalone language experiment. W001 owns the first production module and contract fixtures. [Experiment](../evidence/W000/GO_FEASIBILITY.md) · [W001 issue #12](https://github.com/igorpooh1978/GoEF/issues/12)

## How an operation crosses the system

An application selects a provider and opens a factory with explicit pool ownership. The factory builds a logical model from declarations, validates and freezes it, then resolves its physical mapping against the selected provider and capabilities. A short-lived context uses that frozen model and owns one identity map.

| Operation | Consumers of the frozen model | Result |
| --- | --- | --- |
| Read | Typed DSL or generated declaration → logical query → relational plan → provider compiler and binder → executor | The materializer decodes rows against the projection and optionally resolves tracked entity identity. |
| Save | Change detector → original/current snapshot differences → ordered modifications → transaction and provider executor | Confirmed outcomes determine when temporary keys become durable and changes can be accepted. |
| Migrate | Versioned logical snapshot + inspected physical schema → operation graph → reviewed provider DDL plan | The migration library coordinates locks, execution, history and recovery. CLI delegates to that library. |
| Scaffold | Physical schema inspection → logical model + mapping → deterministic generator | User-owned model extensions survive regeneration. Design-time tools stay outside the runtime import graph. |
| Process a local collection | Go slice or native iterator → LINQ to Objects | Ordinary callbacks run as values are enumerated. Database operations require an explicit materialization or stream boundary. |

Provider modules depend on the core SPI and their chosen database drivers. Consumers own narrow contracts. The application composes concrete providers. SQL dialect nodes enter at relational translation, and DDL operations have a distinct representation from query expressions. Keep driver-specific types inside adapters. Future nonrelational storage needs its own contract.

## Contracts to review together

- [Public API](PUBLIC_API.md) and [module boundaries](../adr/0002-module-and-provider-boundaries.md) define construction, ownership and dependency direction.
- [Model](MODEL_ENGINE.md), [query](QUERY_ENGINE.md) and [tracking](CHANGE_TRACKER.md) describe where validation, parameterization and identity are enforced.
- [Provider SPI](PROVIDER_SPI.md) and [security and lifecycle](SECURITY_AND_LIFECYCLE.md) specify what a concrete provider must prove at the I/O boundary.
- [Generics ADR](../adr/0003-go-linq-and-generics.md) distinguishes compiler-verified method shapes from proposed APIs. The model, SQL, tracking, migration and resource contracts need independent conformance evidence.
