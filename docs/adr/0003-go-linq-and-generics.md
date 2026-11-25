# ADR 0003 — Generic query facade and expression binding

**Status:** Compiler constraints verified on Windows/amd64 with Go 1.27.4. The API design is proposed for W001. [Evidence with reproducible fixtures](../evidence/W000/GO_FEASIBILITY.md) · [W001 issue #12](https://github.com/igorpooh1978/GoEF/issues/12)

## Decision under review

Expose a concrete database context with generic `Set[T]` and a concrete query with immutable operators and type-changing `Select[R]`. Express a projected member with its owner and result type. The compiler infers the projected type from the descriptor. The provider consumes a versioned ordinary plan through non-generic methods. A local Objects facade accepts callbacks over Go values. SQL queries accept typed symbolic inputs or generated entry points that bind explicit declaration parameters.

## What the compiler established

| Candidate | Result on Go 1.27.4 | Design consequence |
| --- | --- | --- |
| Concrete `Set[T]`, ordinary `Where`, generic `Select[R]` and interface dispatch | Compiles and runs. The test assigns an inferred named result type, invokes the ordinary provider method and verifies the original query is intact. | Generic fluency is possible on concrete types. The SQL compiler can use ordinary interfaces. |
| Interface declaring a method type parameter | Compiler rejects the declaration. | A generic context/query interface with those methods cannot be a provider extension point. |
| Generic concrete method implementing an otherwise compatible ordinary interface method | Compiler rejects the interface assignment. | Implement the exact ordinary method, or supply an explicit non-generic adapter. |
| Method on `Query[T]` returning `Query[[]T]` | The fixture triggers an instantiation cycle. A top-level generic function producing this shape compiles. | Test each recursively expanding API separately. Prefer a top-level generic operator or a distinct non-recursive result wrapper when necessary. |
| Field from another entity or incompatible projected result assignment | External-package callers receive compile errors. | Keep typed owners and results visible at the facade. Self-join alias identity still needs runtime validation. |

Go 1.27 added type parameters on concrete methods and restricts interface methods. These rules do not provide a runtime representation of a closure's expression. [Go 1.27 language changes](https://go.dev/doc/go1.27#language)

## Alternatives and remaining evidence

A functional API with top-level generic functions supports type-changing and nesting operators, at the cost of longer fluent composition. Select it for individual operations that fail the recursive receiver check. A string-based predicate would move entity/field errors into execution and require parsing and binding user text. Restrict textual SQL to the explicit raw API. Reflecting on function pointers cannot retrieve a portable typed expression body.

Use one logical query IR for runtime descriptors and a generated-entry-point frontend. W001 must exercise source-span errors, alias safety on self-joins, captures, repeated binding, code-generation freshness, DTO projections and stable client/server transitions. Publish exact package names and signatures only after these checks and cross-provider semantics are reviewed. [Query engine](../architecture/QUERY_ENGINE.md) · [Public API](../architecture/PUBLIC_API.md)
