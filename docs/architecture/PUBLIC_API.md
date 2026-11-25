# Public API and ownership

Proposed W001 contract. The following call shape illustrates a database projection. The application entity, its field descriptors and the database entry points still require implementation. [Go compiler experiment](../evidence/W000/GO_FEASIBILITY.md) · [W001 review](https://github.com/igorpooh1978/GoEF/issues/12)

```go
ids, err := db.Set[Order]().
    Where(Orders.TotalMinor.Gt(minimum)).
    Select(Orders.ID).
    ToList(ctx)
```

Here `db` is a short-lived context. `Order` denotes an entity type and `Orders.ID` carries both that owner and its projected Go type. Composing a query performs no I/O. Each execution binds its own arguments. Mutable values captured as execution inputs require a defensive copy or declared immutable codec semantics. An entity-tracking read must expose stable entity pointers. Scalar/DTO projections return typed values. W001 must settle how terminals distinguish tracked pointers from DTO values and compile a public example before publishing exact signatures.

| Surface | Proposed observable contract |
| --- | --- |
| Factory and context | A factory may be shared. Each context has one owner and rejects overlapping operations, including active row streams. Closing a context releases its own resources. A caller-supplied pool remains owned by the caller. |
| Model builder | Register entity, field, converter, key and relationship declarations. Freeze validates them before SQL generation. Typed errors identify the declaration path. |
| Query | A concrete generic facade provides `Set[T]`, immutable `Where` and type-changing `Select[R]`. A generation frontend uses explicit entry points. Both describe one logical query. |
| Terminal operations | `ToList(ctx)` returns an empty collection for no matches. A fetch expecting one row distinguishes missing and multiple results. The proposed optional form returns a presence flag so zero values remain valid data. |
| Streaming | `Stream(ctx)` yields `iter.Seq2[T,error]`. A successful item carries a nil error. A terminal error carries the zero value and is emitted once. A consumer that stops early causes row closure before enumeration returns. |
| Change tracking | Attach and mutation APIs accept addressable entity pointers. Save returns affected tracked entries and a typed error. Explicit acceptance requires a known successful transaction outcome. |
| Programmatic migrations | Applications can inspect, plan, script or apply a registered migration set using the library. CLI and design-time tools are optional callers. |
| Errors | Public kinds support `errors.Is`, structured details support `errors.As`, and wrapping preserves caller cancellation. Provider fields are sanitized. Check retry class and commit outcome before replay. |

The EF-parity profile tracks keyed entity queries by default. No-tracking reads and per-query identity resolution have explicit modes. Loading a relationship through network I/O requires an explicit call or opt-in loading policy. Expose a change to tracking defaults through a named compatibility profile and an ADR.

W001 must exercise source compatibility from an external package, cardinality errors, stream cancellation and early termination, context close, alternatives for pointer projections, provider construction and invalid field ownership. The upstream reference for save cancellation is `SaveChangesAsync_with_canceled_token`. [Pinned DbContext tests](https://github.com/dotnet/efcore/blob/b34c0e9d7e768aa808ded44f19e7f871bea6869f/test/EFCore.Tests/DbContextTest.cs)
