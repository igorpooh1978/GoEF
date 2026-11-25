# Relational provider SPI

Proposed consumer-owned contracts. The public facade may use generic concrete methods. A provider implements ordinary methods over versioned, publicly nameable contract types. Independent modules and third-party adapters must be able to implement the SPI without importing a core internal package. The compiler experiment exercises ordinary interface dispatch of a tiny plan value. [Evidence](../evidence/W000/GO_FEASIBILITY.md) · [Module ADR](../adr/0002-module-and-provider-boundaries.md)

| Contract | Inputs → outputs | Acceptance |
| --- | --- | --- |
| Identity and capabilities | Server, driver, configuration and SPI version → immutable feature/limit descriptors | Reject incompatible contract versions or unsupported requirements before executing a command. Capability evidence records engine and driver versions. |
| Model mapping | Logical scalar/facets + provider annotations → store type, read/write codec and generation semantics | Verify nullable, precision, offsets and collation behavior. Surface explicit limits or lossy conversions. |
| Query translation | Validated relational nodes and ordered result slots → SQL template + typed parameter layout | Quote each identifier component and bind every data slot through the driver. An unknown node or missing capability produces a typed failure. |
| Command and rows | Context + template + bindings + bound transaction → count/result stream | Distinguish unknown counts, cancellation and late row errors. Specify row and statement cleanup with early return. |
| Transactions | Connection + supported isolation → transaction handle, optional savepoints, terminal outcome | Report ownership, rollback failure and ambiguous commit. Only a complete replay-safe unit qualifies for retry after a confirmed rollback. |
| Schema and migrations | Database identity + snapshot + operation graph → inspection, validated DDL, scoped lock, history/journal actions | Prove actual schema/data/history after apply, rollback or partial failure. Capability metadata identifies operations requiring separate transaction groups. |
| Error translation | Native structured diagnostics + phase and outcome → public error kind, details and retry class | Preserve cancellation; redact query inputs. Unknown codes stay unknown. Exercise actual server failures. |
| Optional extensions | Declared bulk operation or provider-specific function → namespaced capability | Require a positive capability match and a stable error for unsupported combinations. |

Compose only contracts consumed by a particular operation. A migration planner can work with offline metadata while an executor requires a connection. Split query SQL and DDL nodes, preserve transaction binding and place all direct driver imports inside provider modules. Keep public plan types versioned, immutable or explicitly borrowed for a documented lifetime. A consuming application wires concrete dependencies at construction.

W001 should compile an external test adapter against exported SPI types and a second independent use case before stabilizing them. A synthetic adapter tests the contract boundary. PostgreSQL, SQLite, SQL Server and MySQL each require separate, independently identified real-engine proof for semantics, cancellation, cleanup, transaction outcomes, native error mapping and migrations. Provider/driver versions, module cut, ABI compatibility policy and export names require review. [W001 issue #12](https://github.com/igorpooh1978/GoEF/issues/12)
