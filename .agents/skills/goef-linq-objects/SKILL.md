---
name: goef-linq-objects
description: Work on typed in-memory Go LINQ over slices, maps, iter.Seq or one-shot sources. Cover Enumerable semantics, laziness, short-circuiting and benchmarks.
---

# GoEF Linq Objects

1. Read [LINQ Objects](../../../docs/architecture/LINQ_OBJECTS.md) and [LINQ research](../../../docs/upstream/LINQ_RESEARCH.md). Compare pinned .NET Enumerable and go-linq cases. Keep the Objects package independent of database drivers.
2. Specify source ownership, replayability, laziness, short-circuiting and buffering for each operator. State nil/empty, element count, duplicates, equality, tie ordering, default values and error behavior. Address typed nil and non-comparable values with explicit constraints or comparers.
3. Test instrumented pulls, cleanup on early termination, finite/infinite behavior, one-shot reuse and mutation limits. Check stable ordering where promised. Preserve explicit errors for error-aware inputs.
4. Record scenario/test evidence and operator rows. For performance, compare matched native Go and go-linq workloads, including allocations, and retain exact environment/toolchain details.

For `Where` followed by first-match selection, prove zero pulls during construction and termination after the first matching element.
