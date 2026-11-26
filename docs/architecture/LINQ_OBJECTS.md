# LINQ to Objects

`objects.Where` and `objects.Select` compose lazily over `iter.Seq`. A caller can filter orders, project matches to IDs and stop after one result. A consumer's stop signal travels back through both operators. A terminal `objects.Count` enumerates the whole sequence once, runs the upstream callbacks for the demanded items and counts every projected value. Its Go `int` result is checked for overflow. The remaining obligations below describe planned behavior. [Examples](../user/LINQ_OBJECTS.md) · [Issue #4](https://github.com/igorpooh1978/GoEF/issues/4)

| Input | Proposed observation |
| --- | --- |
| Slice | Enumerations traverse the current elements of the captured slice. No snapshot or synchronization is implied. Specify mutation timing before accepting concurrent readers. |
| Map | Yield typed key/value pairs. Enumeration order is unspecified. Order explicitly before comparing two outputs by position. |
| String | Yield Unicode code points in iteration order. Indexed operators count elements. Byte-offset APIs need distinct names. |
| Native iterator | Forward early stop. Replayability follows the source. A factory can supply a fresh iterator when replay is required. |
| Channel | Receive consumes values once. The producer owns closure. Cancellation and early exit require an agreed producer shutdown mechanism. |
| Empty/nil | Specify nil slice/map as empty and test both. A nil channel can block, so a channel adapter needs a cancellation policy. The current `Where` and `Select` take non-nil iterator and callback functions. |

### Operator obligations

| Operations | Required distinction and test |
| --- | --- |
| Where, Select, SelectMany, Zip | Preserve source encounter order. An indexed callback counts input positions before filtering. Flatten in outer-then-inner order. Zip stops at the shorter input and closes any pulled iterators. Test side effects, including how many values were read from each input. |
| Any, All, First, Single | Any over empty is false and All is true. First may stop after one match. Single must distinguish zero, one and at least two matches. Distinguish missing from a present Go zero value. Define explicit default value overloads. |
| Skip, Take, Chunk, Last, Range | Cover negative counts, bounds from the end, short final chunks and steps in both directions. Taking a finite prefix must terminate on an infinite source. Several suffix and from-end forms need buffering or complete traversal. |
| OrderBy, ThenBy, Reverse, Shuffle | Stable sort preserves encounter order on ties. ThenBy refines that order. Reverse needs the tail before it yields the head. For Shuffle compare permutations with multiplicity and use a reproducible random fixture. Unbounded inputs cannot yield a fully sorted result. |
| Distinct, Union, Intersect, Except, Concat | Specify first occurrence, multiplicity and encounter order separately. Concat keeps duplicates. Set operations require equivalent hash/equality contracts. A projected comparable key or explicit equality function can accommodate slices and map-containing structs. Cost can be quadratic when only pairwise equality is available. |
| GroupBy, Join, GroupJoin, Left/Right/FullJoin | Exercise two equal inner keys, unmatched rows on both sides and a nullable key. Test group encounter order and member order. Hash joins retain an index of one side and grouping may retain all items. Presence flags distinguish an unmatched value from its zero value. |
| Sum, Average, Min/Max, Aggregate | Record seed, selector, result type, overflow policy, nullable inputs, empty input, NaN and infinities. Check fixed-width integers at both limits. Pin rounding for decimal data. |
| ToSlice, ToMap, ToLookup, ToHashSet | Materialization costs at least the retained output size. Distinguish dictionary duplicate-key errors, explicit replace policies and lookup buckets. Test typed nil members independently of a missing element. |

Go functions with comparable-key constraints or ordered selectors keep type checking at the call site. Go 1.27 allows type parameters on concrete methods. Interface methods cannot declare them or be implemented by generic methods. A top-level generic function is a candidate for recursively expanding outputs such as a query of chunks. Public receiver signatures, constraints and comparator error contracts await exact compiler fixtures. The package should compile in isolation using the standard library. No ORM, driver or SQL imports belong in its dependency graph. [Go 1.27 language changes](https://go.dev/doc/go1.27#language)

See the [operator registry](../parity/linq-operators.json), [source research](../upstream/LINQ_RESEARCH.md) and [object specifications](../../specs/features/linq/objects.feature).
