# Candidate module and import graph

W001 adopts `github.com/igorpooh1978/GoEF` as the module path and `goef` as its root package. Reuse the exact case of the Git remote in every import. The offline experiment exercises this spelling with a local replacement. Publication, proxy resolution, release tags and a supported provider version matrix still require separate evidence. [Issue #12](https://github.com/igorpooh1978/GoEF/issues/12) · [ADR 0002](../adr/0002-module-and-provider-boundaries.md) · [Experiment](../evidence/W001/ARCHITECTURE.md)

## Who imports whom

Paths in this table are relative to the adopted module root. The provider execution contract, the initial `model` and `goeferr` packages, and `linq/objects.Where` are available in the current source. The table describes their planned expansion and the remaining dependency graph. Each row lists direct project dependencies relevant to the design. Each provider directory is a module boundary, including when several providers live in the same checkout.

| Importer | Proposed imports | Reason |
| --- | --- | --- |
| Application | Root `goef`, one selected provider, and the public packages it uses | The application passes the concrete adapter and its ownership policy to the factory. An Objects-only consumer imports just `linq/objects`. |
| `goef` | `model`, `provider`, `linq/queryable`, `goeferr`, internal query execution and tracking | The root composes a frozen model, context operation slot, transaction binding and tracker. |
| `linq/objects` | Standard library | An application can enumerate Go values without linking the ORM. |
| `linq/expressions` | `model` for logical descriptor identities | Typed symbolic inputs carry owner and result types. They require neither an executing context nor a SQL dialect. |
| `linq/queryable` | `linq/expressions`, internal query IR and execution binding | Set roots and projections refer to the context's execution binding. The binding is defined below the root facade, so queryable has no back edge to `goef`. |
| `model`, `goeferr` | Standard library, with `model` also using `goeferr` | Frozen logical metadata and portable errors are available to all consumers. Provider-resolved physical mappings are separate. |
| `provider` | `model` and `goeferr` where needed | The core owns the public, versioned SPI vocabulary. Driver packages can name every argument and result. |
| Internal query execution, materialization and tracking | Shared model/query IR, `provider`, `goeferr` | Both typed expressions and generated declarations lower into one logical IR. Lowering exposes a versioned relational view to providers while the normalizer and tracker stay private. |
| `migration` | `model`, `provider`, `goeferr` and internal schema/migration engine | Programmatic callers and the CLI use the same plan, approval and recovery rules. Internal engines receive neutral operation data below the public facade to avoid a reverse import. |
| `providers/postgres`, `providers/sqlite`, `providers/sqlserver`, `providers/mysql` | Each requires core and its own selected driver | An adapter implements ordinary SPI methods. Its peers, the root composition facade, CLI and generator are outside its dependency graph. |
| Design-time tools and optional CLI | Public model, migration and expression contracts, plus selected adapters where online execution is required | Generated application entry points reference runtime contracts only. GoEF runtime packages never import a tool. |

## Crossing the provider boundary

The factory checks the adapter's SPI major before acquiring a lease. Plan negotiation must also reject unknown required nodes. New optional capabilities use separate interfaces. A change to a required interface or its semantics requires a new SPI generation. The [frozen execution SPI](PROVIDER_SPI.md) contains Descriptor, Acquire, Query, Exec and the lease/row lifecycle. Compiler, transaction and schema methods in this table describe extensions that await their implementing waves.

| Candidate interface method | Caller and ownership | Failure to exercise |
| --- | --- | --- |
| `Backend.Descriptor() Descriptor` | The adapter reports its SPI major and stable adapter ID without I/O. The later capability extension identifies driver and server configuration. | Reject an incompatible major or missing required capability before issuing a user command. |
| `Compiler.Compile(RelationalPlan) (Command, error)` | Core supplies validated identifiers, parameter slots and output layout. The adapter returns a reusable template. Execution values belong to a single call. The adapter owns any copies it retains. | Unknown required operation produces a registered translation/capability error with zero commands. |
| `Connector.Acquire(context.Context) (Lease, error)` | Runtime or migrator acquires a connection lease. Reads, writes and transaction control for one bound unit stay on its physical connection. | Failed acquisition transfers no lease. A suspect connection is discarded before pool reuse. |
| `Executor.Query(context.Context, Command, Bindings) (Rows, error)` | The bound executor creates rows. The materializer consumes and closes them on exhaustion, error, cancellation or early stop. An active stream occupies the context operation slot. | A late read error remains observable. A close failure preserves the primary error and the connection disposition. |
| `Executor.Exec(context.Context, Command, Bindings) (WriteResult, error)` | A save dispatches ordered writes through the bound executor. A write result distinguishes a known affected count from an unavailable count. | An unknown count cannot establish an optimistic concurrency conflict. An adapter needs an independently verified way to determine the write effect. |
| `Transaction.Commit(context.Context) (Outcome, error)` | Only the owner completes the transaction. Outcome records the known durable result separately from an error returned during cleanup. | A lost commit acknowledgement takes precedence over a transport error. Suspend tracker acceptance and replay until reconciled. |
| `SchemaInspector.Inspect(context.Context, Scope) (SchemaSnapshot, error)` | A migration caller requests an inspection on the selected database/lease. DDL operations have their own plan types. | Recheck the target, history and drift under the migration lock before dispatch. An uncertain step requires recovery evidence. |

Adapters translate vendor diagnostics at their boundary. Public errors expose a stable kind and safe detail including phase and disposition. They preserve causal cancellation for `errors.Is`. Row values, driver error objects, SQL text and connection strings stay private. The unit-of-work owner decides replay using actual transaction disposition. A provider's “transient” flag cannot authorize replay after an unknown commit. [Error contract](ERROR_CONTRACT.md) · [Transaction ownership](TRANSACTION_ENGINE.md)

## Decisions and their cost

One core module lets the model, plan vocabulary and public facades evolve at the same revision. A separate Objects module would allow an independent release cadence and add a second version to coordinate. A module containing all four drivers would simplify tagging and enlarge the dependency graph of a one-provider consumer. The proposal gives each driver its own module and checks package closure and module requirements separately. Exposing the entire internal query AST would speed the first adapter and turn normalization details into external compatibility obligations. The SPI exposes the relational input that a compiler actually consumes.

The [offline experiment and acceptance stages](../evidence/W001/ARCHITECTURE.md) specify which parts have compiler evidence. The core module and execution SPI now have an external compile fixture. Exact driver pins, concrete adapters and compatibility publication require their own evidence.
