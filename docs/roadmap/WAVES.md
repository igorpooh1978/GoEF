# Wave gates

The [master plan](MASTER_PLAN.md) is the canonical scope and exit-gate register from W000 through W020+. The [checkpoint](CHECKPOINT.md) is the current execution state. A wave can be split into issues while preserving the parent scope.

| Sequence and prerequisites | Ownership | Handoff evidence |
| --- | --- | --- |
| W000 source, contracts, issues, skills and specs | [Bootstrap owner](https://github.com/igorpooh1978/GoEF/issues/1) | Source pins, reviewed local docs and recorded infrastructure blockers |
| W001 after W000 architectural handoff | [Architecture owner](https://github.com/igorpooh1978/GoEF/issues/12) | Compile-proven module, context and SPI acceptance fixtures |
| W002 after W001 contracts | [Test owner](https://github.com/igorpooh1978/GoEF/issues/13) | Executable traceability, negative cases, pinned real-engine environments and coverage |
| W003–W008 after relevant architecture and test gates | Model, migration, provider and query owners to be assigned per issue | Model/operations contracts before provider execution, provider foundation before migration execution or SQL queries, independent Objects proof |
| W009–W013 after provider/query foundations | Transaction, relationships, tracking and LINQ owners to be assigned per issue | Safe retry/outcomes, relationship freshness, change tracking and portable query/compiler proof |
| W014–W020+ after dependent core behaviors | Provider, tooling, hardening and release owners to be assigned per issue | Advanced mapping and query, provider breadth, data-safe release, measured and independently verified parity |

Move acceptance only with evidence and exact integrated SHA. Reopen the issue if a claim regresses or loses source coverage.
