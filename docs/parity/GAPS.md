# Open parity gaps

The [W000 registry](features.json) contains 50 seed records across 18 registry families. 31 records have proposed Gherkin scenarios and 19 are discovered. Zero GoEF behaviors have executable or independent acceptance evidence. By origin there are 40 EF Core records, 3 .NET LINQ reference records and 7 GoEF extensions. The [EF Core research index](../upstream/RESEARCH_INDEX.md) counts 21 survey families, 18 with at least one seed and 3 awaiting a first record. The family survey and seed registry count different units.

| Registry family | Seed records | Specified | Discovered |
| --- | ---: | ---: | ---: |
| Context | 2 | 0 | 2 |
| Diagnostics | 1 | 0 | 1 |
| Documentation | 2 | 2 | 0 |
| Errors | 4 | 4 | 0 |
| LINQ boundary | 1 | 1 | 0 |
| LINQ conformance | 1 | 1 | 0 |
| LINQ expressions | 1 | 1 | 0 |
| LINQ objects | 1 | 1 | 0 |
| LINQ queryable | 1 | 1 | 0 |
| Materialization | 1 | 0 | 1 |
| Migrations | 7 | 7 | 0 |
| Model | 8 | 5 | 3 |
| Nonrelational stores | 2 | 0 | 2 |
| Provider-specific | 2 | 0 | 2 |
| Query | 6 | 2 | 4 |
| Tooling | 2 | 0 | 2 |
| Tracking | 2 | 0 | 2 |
| Transactions | 6 | 6 | 0 |

| Provider | Cells | Applicable and NOT_RUN | NOT_APPLICABLE | Verified |
| --- | ---: | ---: | ---: | ---: |
| PostgreSQL | 50 | 41 | 9 | 0 |
| SQLite | 50 | 42 | 8 | 0 |
| SQL Server | 50 | 43 | 7 | 0 |
| MySQL | 50 | 42 | 8 | 0 |
| Total | 200 | 168 | 32 | 0 |

The 32 inapplicable cells cover provider-independent LINQ object and expression construction, documentation freshness, nonrelational store behavior, and explicitly engine-specific SQLite, MySQL and SQL Server operations. The reason is recorded in each cell. An applicable cell still needs actual engine and Go driver versions and version-specific evidence. These counts are not a percentage of EF Core coverage. The [LINQ operator matrix](linq-operators.json) tracks the finer operator denominator.

| Gap | Current state | Next proof | Tracking |
| --- | --- | --- | --- |
| Complete EF Core capability inventory | PARTIAL | Enumerate public contracts, provider overrides and positive/negative test methods within each surveyed family. Add one record per observable case, including CLI, analyzer and integration cases. Split out dedicated issues before execution. | [#7](https://github.com/igorpooh1978/GoEF/issues/7), [#3](https://github.com/igorpooh1978/GoEF/issues/3). W001-W020+. |
| EF Core 10 and MySQL differential oracle | BLOCKED | Select and pin an EF Core 10-compatible Pomelo release once available, or record a separately approved reference strategy. Keep the GoEF MySQL provider and engine proofs in the release scope. | [#7](https://github.com/igorpooh1978/GoEF/issues/7), [#9](https://github.com/igorpooh1978/GoEF/issues/9). W002 onward. |
| Four-provider versioned proof and native errors | NOT_RUN | Pin the engine/driver matrix, run executable Gherkin mappings on the real engines and record cancellation, write conflicts and error evidence per version. | [#2](https://github.com/igorpooh1978/GoEF/issues/2), [#9](https://github.com/igorpooh1978/GoEF/issues/9). W002/W005/W009/W011. |
| Migration interruption and transaction faults | NOT_RUN | Run lock contention, engine-specific DDL and data-preservation cases, then inject process/network failures and verify history and transaction outcome on the real provider matrix. | [#2](https://github.com/igorpooh1978/GoEF/issues/2), [#9](https://github.com/igorpooh1978/GoEF/issues/9). W006/W009. |
| Complete LINQ operator behavior | PARTIAL | Expand the operator matrix and prove objects, expression rejection and database-backed execution independently. Run matching upstream differential fixtures and separate stable and research semantics. | [#4](https://github.com/igorpooh1978/GoEF/issues/4), [#9](https://github.com/igorpooh1978/GoEF/issues/9). W002/W008/W012/W013/W019. |
| GoEF licensing and reuse | OPEN | Record the project license decision. Review exact file provenance and attribution before any source reuse or dependency approval. | [#15](https://github.com/igorpooh1978/GoEF/issues/15) for the project license. [#7](https://github.com/igorpooh1978/GoEF/issues/7) for upstream provenance. Before distribution. |
