# Public API and ownership

Proposed W001 contract. The following call shape illustrates a database projection. The application entity, its field descriptors and the database entry points still require implementation. [Go compiler experiment](../evidence/W000/GO_FEASIBILITY.md) · [W001 review](https://github.com/igorpooh1978/GoEF/issues/12)

```go
ids, err := db.Set[Order]().
    Where(Orders.TotalMinor.Gt(minimum)).
    Select(Orders.ID).
    ToList(ctx)
```

Here `db` is a short-lived context. `Order` denotes an entity type and `Orders.ID` carries both that owner and its projected Go type. Composing a query performs no I/O. Each execution binds its own arguments. Mutable values captured as execution inputs require a defensive copy or declared immutable codec semantics. An entity-tracking read must expose stable entity pointers. Scalar/DTO projections return typed values. The candidate below gives each a distinct terminal result type.

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

## Candidate signatures for external callers

The signatures below are design notation. Unqualified types belong to the named receiver's package. The [architecture experiment](../evidence/W001/ARCHITECTURE.md) and [W000 language fixture](../evidence/W000/GO_FEASIBILITY.md) identify which API call shapes have compiler evidence. The contracts below still require production packages, external callers and behavior tests.

| Operation | Candidate signature | What the caller owns or observes |
| --- | --- | --- |
| Build the factory | `goef.NewFactory(ctx context.Context, m *model.Model, backend provider.Backend, opts FactoryOptions) (*Factory, error)` | Borrow the supplied adapter by default. An explicit transfer takes effect only on successful construction. The factory validates model and capability compatibility. A failed construction releases resources acquired during that attempt. |
| Start and finish a unit of work | `(*Factory).NewContext() (*Context, error)`, `(*Context).Close() error`, `(*Factory).Close(ctx context.Context) error` | Each context starts with a fresh tracker. Closing releases owned reads and leases and detaches from external transactions. Repeated close is safe. Starting an operation after close returns `ErrContextClosed`. An overlapping operation returns `ErrContextBusy`. Factory shutdown uses the caller's time budget. |
| Register and freeze a model | `(*model.Builder).Register[T any](decl Entity[T]) error`, `(*model.Builder).Freeze() (*Model, error)` | The published model is immutable. Invalid logical declarations return `ErrInvalidModel` before SQL execution. The factory resolves provider-specific physical facets. |
| Get a root and filter it | `(*goef.Context).Set[T any]() queryable.Set[T]`, `(Set[T]).Where(expressions.Predicate[T]) EntityQuery[T]` | `T` denotes the registered entity value type. The query retains the context binding. Composition is pure. A terminal validates model membership and alias identity before dispatch. |
| Read entities | `(EntityQuery[T]).ToList(context.Context) ([]*T, error)` | A set exposes the same entity read operations on its unfiltered root. Repeated tracked reads of the same key return the same address. No-tracking keeps the same result type and skips the context identity map. |
| Project and read values | `(EntityQuery[T]).Select[R any](expressions.Projection[T, R]) Query[R]`, `(Query[R]).ToList(context.Context) ([]R, error)` | A field descriptor supplies both types. A scalar or DTO is materialized as its declared result type. Support for projections containing tracked entities remains a separate materializer case. |
| Check cardinality or stream | `(Query[R]).Single(context.Context) (R, error)`, `(Query[R]).SingleOrNone(context.Context) (R, bool, error)`, `(Query[R]).Stream(context.Context) iter.Seq2[R, error]` | Both single-row forms return `ErrMultipleResults` on duplicates. The required form returns `ErrNotFound` for no rows. The optional form returns zero, false and nil. A materializing terminal returns no partial list on failure. A stream can yield rows before a terminal error. |
| Stage and save edits | `(Set[T]).Add(*T) error`, `(Set[T]).Attach(*T) error`, `(Set[T]).Remove(*T) error`, `(*goef.Context).SaveChanges(ctx context.Context, opts SaveOptions) (SaveResult, error)`, `(*goef.Context).AcceptAllChanges() error` | Stage caller-owned pointers in the context. Count successfully flushed entries in `SaveResult`. A flush in a caller-controlled transaction is provisional. Acceptance requires confirmation of commit. Rollback restores provisional values and original tokens while retaining the intended edits. |
| Prepare and apply migrations | `migration.NewMigrator(backend provider.MigrationBackend, set *migration.Set) (*Migrator, error)`, `(*Migrator).Inspect(ctx context.Context) (Inspection, error)`, `(*Migrator).PlanTo(ctx context.Context, target Target) (Plan, error)`, `(*Migrator).GenerateScript(plan Plan) (Script, error)`, `(*Migrator).Apply(ctx context.Context, plan Plan, opts ApplyOptions) error` | The application supplies a separately configured migration backend and versioned definitions. Inspection and planning issue reads. Scripting a reviewed plan is offline. Apply checks identity, history, schema and plan-bound approval under its lock, then reports any need for recovery. |

The frozen `provider.Backend` supplies the descriptor and connection contracts. Mapping and compilation use separately versioned extensions when implemented. The proposed `provider.MigrationBackend` would supply inspection, DDL, lock and history operations for the programmatic migration engine. Applications can configure these independently. The [module graph](MODULE_GRAPH.md) describes the ordinary methods and their ownership at the adapter boundary.

A single `Query[T]` returning `[]T` for every entity would copy tracked values when `T` is an entity struct. Using `*T` as the expression owner would instead change ordinary field descriptor declarations and still need an identity policy. The candidate splits entity and value terminals while preserving the entity owner type for predicates. Prove cross-entity rejection and same-entity alias binding separately. Keeping filtered reads in `EntityQuery[T]` also leaves `Add`, `Attach` and `Remove` on an unfiltered set. [Tracking behavior](CHANGE_TRACKER.md) · [Expression ownership](LINQ_EXPRESSIONS.md)

Stream creation defers opening rows until enumeration. A consumer break ends yielding immediately. Subsequent cleanup failures use the redacted diagnostic path because the consumer has stopped accepting results. Causal cancellation remains discoverable with `errors.Is`. A lost commit acknowledgement produces `ErrCommitOutcomeUnknown` even if the connection also reports a timeout. The caller must reconcile before replay or tracker acceptance. [Security and lifecycle](SECURITY_AND_LIFECYCLE.md) · [Transaction outcomes](TRANSACTION_ENGINE.md)

The upstream reference for save cancellation is `SaveChangesAsync_with_canceled_token`. The [acceptance stages](../evidence/W001/ARCHITECTURE.md) distinguish compiler, synthetic lifecycle and real-engine obligations. [Pinned DbContext tests](https://github.com/dotnet/efcore/blob/b34c0e9d7e768aa808ded44f19e7f871bea6869f/test/EFCore.Tests/DbContextTest.cs)
