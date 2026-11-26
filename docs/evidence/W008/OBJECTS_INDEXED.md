# Indexed LINQ Objects filtering and projection

[Issue #4](https://github.com/igorpooh1978/GoEF/issues/4) · [LINQ Objects guide](../../user/LINQ_OBJECTS.md) · [Object scenarios](../../../specs/features/linq/objects.feature) · [Executable example](../../../linq/objects/indexed_test.go)

## Contract and provenance

`WhereIndexed` passes the source position to its predicate before deciding whether to yield the value. `SelectIndexed` numbers the elements yielded by its immediate source. Both start at zero on each enumeration, preserve encounter order and forward early termination to the source. Replayability follows the source. Each operator checks that the next index fits Go `int` before calling the callback or yielding a result. A callback can observe `math.MaxInt` as its last valid index. The APIs require non-nil iterator and callback functions. [Go implementation](../../../linq/objects/indexed.go)

The pinned .NET v10.0.0 indexed overloads receive the value followed by a checked 32-bit position. The pinned tests exercise actual predicate positions, overflow and deferred enumeration. Stable go-linq v5.0.0 places the index first and uses unchecked Go `int` increments. GoEF exposes value-first callbacks and checks the architecture-sized Go index. All implementation, fixtures and examples are original GoEF work based on behavioral comparison. [Enumerable Where](https://github.com/dotnet/runtime/blob/60629d14374c56f1cb51819049ad1fa529307f8d/src/libraries/System.Linq/src/System/Linq/Where.cs#L53-L89) · [Enumerable Select](https://github.com/dotnet/runtime/blob/60629d14374c56f1cb51819049ad1fa529307f8d/src/libraries/System.Linq/src/System/Linq/Select.cs#L73-L106) · [Where tests](https://github.com/dotnet/runtime/blob/60629d14374c56f1cb51819049ad1fa529307f8d/src/libraries/System.Linq/tests/WhereTests.cs#L984-L1026) · [Select tests](https://github.com/dotnet/runtime/blob/60629d14374c56f1cb51819049ad1fa529307f8d/src/libraries/System.Linq/tests/SelectTests.cs#L100-L126) · [go-linq Where](https://github.com/ahmetb/go-linq/blob/6ebc43b7c971a8a4cb9e38742f6cb491d4032cbb/where.go#L17-L37) · [go-linq Select](https://github.com/ahmetb/go-linq/blob/6ebc43b7c971a8a4cb9e38742f6cb491d4032cbb/select.go#L29-L51) · [License and notices review](../../../THIRD_PARTY_NOTICES.md)

## Workstation verification

The Windows workstation used Go 1.27.4 on 2026-11-26. At baseline `85799b679601610a1fb72971b74c38a40d9f7224`, tests for the absent API failed to compile. A temporary implementation that passed index zero into both callbacks and skipped overflow checking compiled. In that revision `go test -mod=readonly -count=1 -run '^(TestIndexed|TestAdvanceIndex)' ./linq/objects` exited 1 with assertion failures for first, replayed and single-use positions and the missing overflow panic. The checked implementation passed the full package suite. [Public contract tests](../../../linq/objects/indexed_test.go) · [Index-boundary test](../../../linq/objects/indexed_internal_test.go)

- `go vet -mod=readonly ./linq/objects` exited 0. `go test -mod=readonly -count=1 -shuffle=on -coverprofile=<amd64 profile> -v ./linq/objects` exited 0 with 30 of 31 Objects statements covered. It exercised every indexed-operator statement.
- On the same workstation with `GOARCH=386` and `CGO_ENABLED=0`, `go test -mod=readonly -count=1 -timeout=100s -shuffle=on -coverprofile=<386 profile> -v ./linq/objects` exited 0 with all 31 statements covered. One public test per operator observed positions from zero through `math.MaxInt`, confirmed the overflow panic on the next input and verified that the source cleanup ran. [Public overflow fixture](../../../linq/objects/indexed_386_test.go)

## Local differential comparison

A temporary equivalent fixture ran in GoEF, stable go-linq v5.0.0 at `6ebc43b7c971a8a4cb9e38742f6cb491d4032cbb` and .NET runtime 10.0.0. Go ran the fixture with `go run -mod=readonly .` and local replacements for the checked-out source trees. .NET SDK 10.0.402 built the equivalent fixture offline with `dotnet build fixture.csproj --source <fixture directory>`. The pinned runtime executed it with `dotnet exec --fx-version 10.0.0 fixture.dll`. Both launches exited 0 and produced matching traces.

| Phase | Starts | Cleanups | Source values | Predicate inputs as index:value | Selector inputs and results as index:value |
| --- | --- | --- | --- | --- | --- |
| Construction | 0 | 0 | | | |
| Stop at the first result | 1 | 1 | 1, 2 | 0:1, 1:2 | 0:2 |
| Replay to completion | 2 | 2 | 1, 2, 4, 2, 3 | 0:1, 1:2, 2:4, 3:2, 4:3 | 0:2, 1:4, 2:2 |

On Windows/amd64 with an Intel Core Ultra 9 185H, a temporary benchmark over 1,024 integers compared a direct loop, the GoEF indexed pipeline and the pinned go-linq pipeline. The three paths used the same indexed filter, indexed projection and checksum accumulation. `go test -mod=readonly -run '^$' -bench '^BenchmarkIndexedPipeline' -benchmem -count=3 -benchtime=100ms .` exited 0. Median observed times were 1,222 ns/op for the direct loop, 6,927 ns/op for GoEF and 5,583 ns/op for go-linq. The direct loop and GoEF had zero measured allocations, and go-linq used 152 B/op in six allocations. This is one local workload.
