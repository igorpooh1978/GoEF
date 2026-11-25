# LINQ conformance fixtures

**Planned · LINQ-CONFORMANCE-001 · [issue #4](https://github.com/igorpooh1978/GoEF/issues/4).** The [feature files](../../../specs/features/linq/) specify observable outcomes. No GoEF runner, database execution or benchmark result exists in this bootstrap lane. W002 supplies the runner. W008, W012 and W013 attach executable cases to the exact overloads and supported provider versions. W019 measures regressions.

| Fixture | Inputs and observations |
| --- | --- |
| Filter, select, take | Use a counting iterator over 1, 2, 3, 4 and an infinite source with the same prefix. Construction reads zero items. Record items read before the first match, callback counts and early-stop propagation. |
| Source lifetime | Enumerate a stable slice twice, drain a closed channel twice and cancel a blocked channel receive. Check runes, nil maps/slices, iterator factories and independent readers of immutable data. Use synchronization for producer completion and run the race detector. |
| Equality and order | Mix duplicate keys, equal sort keys with different IDs, map iteration, slices, nullable keys, typed nils and explicit comparers. Check equivalence/hash consistency and ordering transitivity under fuzzing. |
| Sets, groups, joins | Outer keys A, A, B and inner keys A, A, C give four matching A pairs. A left join adds the unmatched B, and a full join also adds C. Assert multiplicity and presence flags. Run a separate nullable-key case against the chosen reference. |
| Cardinality and numeric edges | Empty, one zero, two zeroes, negative and from-end indices, empty nullable sums/averages, signed overflow, decimal rounding, NaN/infinities. Compare error category or absence independently of the returned value. |
| Typed frontend | Compile valid type-changing projections and reject wrong fields, aliases, operand types and generic interface declarations. Fuzz expression visitors, IR serialization and malformed AST shapes. Reject stale generated output. Rebind the same generated query with changed captured values. |
| Database semantics | Seed two tenants with duplicated and null join keys, tied timestamps, mixed-case accented text and decimals. Configure each provider's collation, time zone, null behavior and transaction. Use a unique final sort key for ordered comparisons. Verify rows and their multiplicities, parameter types, SQL shape, command count, identity resolution and transaction ownership. |
| Explicit boundary | Reject an unregistered predicate before any command. Materialize only filtered rows, then run a local transform. Inject a late row error, a decoding failure, cancellation and early stop. Observe terminal error, row closure, pool return and preservation of a caller-owned transaction. |

### Comparison protocol

1. Record fixture/overload IDs, seed, input data, source SHA, Go and .NET versions, exact EF/provider/driver and engine versions, configuration and integrated GoEF SHA. Reuse the same data and bounds across runs. Use pinned .NET Enumerable or go-linq for a local comparison and EF with the corresponding real engine for a relational comparison.
2. Execute independently and capture typed values, order only where guaranteed, null/missing/error states, callback/command counts and resources. Compare bags for unordered output. A SQL golden checks compilation. Only an actual engine run can check decoded results and dialect behavior.
3. Record unavailable or unexecuted baselines and cells as BLOCKED or NOT RUN with a reason. Keep each overload, negative case and provider/version denominator visible. Store test names, commands, exit codes, artifacts and independent review against the exact integrated SHA. Keep statement coverage of owned Go modules/providers separate from semantic coverage.

### Benchmark protocol

Compare native loops/iterators, GoEF Objects and go-linq v5 on equal input and identical result sinks. Include filter/project/first with matches at the start, end and nowhere, materialization of a full slice, stable sorting with ties, and joins at low and high key cardinality. Check outputs, prefix consumed and allocation ownership before timing. Record benchmark size, repetitions, allocations/op, bytes/op, elapsed time/op, throughput and peak retained memory when buffering dominates. Consume results so work cannot disappear.

For provider paths, use the corresponding native driver and equivalent parameterized SQL on the same database, connection settings and seed. Record rows scanned/returned where available, plan compilation versus cache hits, warm/cold timings, p50/p95 latency, repeat count, CPU architecture, memory, OS and toolchain. Measure server-filtered queries and explicitly materialized client work separately. Timing cannot offset incorrect results, excess row retrieval or a hidden callback fallback. [Upstream go-linq benchmark definitions](https://github.com/ahmetb/go-linq/blob/6ebc43b7c971a8a4cb9e38742f6cb491d4032cbb/benchmark_test.go) · [acceptance scenarios](../../../specs/features/linq/conformance.feature)
