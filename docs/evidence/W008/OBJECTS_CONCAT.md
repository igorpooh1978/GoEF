# Lazy sequence composition in LINQ Objects

[Issue #4](https://github.com/igorpooh1978/GoEF/issues/4) · [LINQ Objects guide](../../user/LINQ_OBJECTS.md) · [Object scenarios](../../../specs/features/linq/objects.feature) · [Runnable example](../../../linq/objects/concat_example_test.go)

## Contract and provenance

`Concat(first, second)` yields both typed sequences in argument order. `Append(source, value)` yields a final value only after the source completes. `Prepend(source, value)` offers the value before starting the source. Constructing these operators leaves every source idle. When a consumer stops, the current source receives the stop signal and later segments stay idle. Replayability and cleanup follow each source. Inputs must be non-nil `iter.Seq` functions. Empty sources, repeated values and typed nil elements retain their normal sequence semantics. The GoEF implementation uses constant state per operator and streams values without materialization. [Implementation](../../../linq/objects/concat.go)

.NET runtime v10.0.0 implements sequence composition as lazy enumerable iterators. Its tests include mixed empty inputs, repeated enumeration, run-once sources and consuming the prepended item without starting the source. Stable go-linq v5.0.0 composes typed iterator methods with equivalent ordering and early-stop behavior. The GoEF code, tests and examples are original work based on behavioral comparison. [Enumerable Concat](https://github.com/dotnet/runtime/blob/60629d14374c56f1cb51819049ad1fa529307f8d/src/libraries/System.Linq/src/System/Linq/Concat.cs#L11-L36) · [Concat iterator](https://github.com/dotnet/runtime/blob/60629d14374c56f1cb51819049ad1fa529307f8d/src/libraries/System.Linq/src/System/Linq/Concat.cs#L231-L264) · [Enumerable Append and Prepend](https://github.com/dotnet/runtime/blob/60629d14374c56f1cb51819049ad1fa529307f8d/src/libraries/System.Linq/src/System/Linq/AppendPrepend.cs#L11-L34) · [Concat tests](https://github.com/dotnet/runtime/blob/60629d14374c56f1cb51819049ad1fa529307f8d/src/libraries/System.Linq/tests/ConcatTests.cs#L14-L48) · [Append and Prepend tests](https://github.com/dotnet/runtime/blob/60629d14374c56f1cb51819049ad1fa529307f8d/src/libraries/System.Linq/tests/AppendPrependTests.cs#L53-L102) · [go-linq reference](https://github.com/ahmetb/go-linq/blob/6ebc43b7c971a8a4cb9e38742f6cb491d4032cbb/concat.go) · [License and notices review](../../../THIRD_PARTY_NOTICES.md)

## Workstation verification

At baseline `c9a438928cc25b0c8ee8fde7bf78e27d037d6e65`, the public tests failed to compile before the three APIs existed. A temporary implementation that buffered every source made `go test -mod=readonly -count=1 -run '^(TestConcat|TestAppendAndPrepend|TestSequenceComposition|TestCompositionRetains)' ./linq/objects` exit 1. The failures showed eager source starts, incorrect replay of a single-use input and an extra pull past the requested prefix. [Public tests](../../../linq/objects/concat_test.go)

On the Windows/amd64 workstation with Go 1.27.4 on 2026-11-26, `go test -mod=readonly -count=1 -shuffle=on -coverprofile=<profile> ./linq/objects` exited 0 and covered 55 of 56 Objects statements, including every statement added here. `go vet -mod=readonly ./linq/objects` and `go test -mod=readonly -count=1 -shuffle=on ./...` also exited 0.

## Local differential comparison

A temporary equivalent fixture matched GoEF, stable go-linq v5.0.0 at `6ebc43b7c971a8a4cb9e38742f6cb491d4032cbb` and .NET 10.0.0 on Windows. It concatenated sources yielding `1, 2` and `2, 3`, then added `0` before and `9` after them. `go run -mod=readonly .` used local source replacements. .NET SDK 10.0.402 built its counterpart with `dotnet build fixture.csproj --source <fixture directory>`, then the pinned runtime ran it using `dotnet exec --fx-version 10.0.0 fixture.dll`. All commands exited 0.

| Demand | Results | Source events |
| --- | --- | --- |
| Construct | | No starts or pulls |
| One value | 0 | Both sources idle |
| Three values | 0, 1, 2 | Start and close the first source. The second stays idle. |
| Four values | 0, 1, 2, 2 | Close the first source before opening the second. Close the second after one pull. |
| Replay to completion | 0, 1, 2, 2, 3, 9 | Start and close each source in order. Close the second before yielding 9. |

On Windows/amd64 with an Intel Core Ultra 9 185H, a temporary benchmark summed two sequences of 1,024 integers each with the prepended and appended values included. `go test -mod=readonly -run '^$' -bench '^BenchmarkSequenceComposition' -benchmem -count=3 -benchtime=100ms .` exited 0. Median observed times were 1,792 ns/op for two direct loops, 10,867 ns/op for GoEF and 10,650 ns/op for go-linq. The direct loops allocated zero bytes. Each iterator pipeline measured 106 B/op in six allocations. This is one local workload.
