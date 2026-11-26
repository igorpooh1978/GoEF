# LINQ Objects Skip and Take

[Issue #4](https://github.com/igorpooh1978/GoEF/issues/4) · [LINQ Objects guide](../../user/LINQ_OBJECTS.md) · [Object scenarios](../../../specs/features/linq/objects.feature) · [Runnable example](../../../linq/objects/paging_example_test.go)

## Pinned behavior and GoEF contract

The .NET 10.0.0 integer overloads of `Enumerable.Skip` and `Enumerable.Take` consume a prefix or yield a prefix in encounter order. A nonpositive skip count passes through the input. A nonpositive take count yields an empty sequence without starting the input. Stable go-linq v5.0.0 documents and implements the same integer-count operations. The upstream sources and tests served as behavioral references. GoEF's range loops and fixtures were written independently and use the standard library. [Enumerable Skip](https://github.com/dotnet/runtime/blob/60629d14374c56f1cb51819049ad1fa529307f8d/src/libraries/System.Linq/src/System/Linq/Skip.cs#L10-L35) · [Enumerable Take](https://github.com/dotnet/runtime/blob/60629d14374c56f1cb51819049ad1fa529307f8d/src/libraries/System.Linq/src/System/Linq/Take.cs#L10-L25) · [Enumerable Skip tests](https://github.com/dotnet/runtime/blob/60629d14374c56f1cb51819049ad1fa529307f8d/src/libraries/System.Linq/tests/SkipTests.cs) · [Enumerable Take tests](https://github.com/dotnet/runtime/blob/60629d14374c56f1cb51819049ad1fa529307f8d/src/libraries/System.Linq/tests/TakeTests.cs) · [go-linq Skip](https://github.com/ahmetb/go-linq/blob/6ebc43b7c971a8a4cb9e38742f6cb491d4032cbb/skip.go#L3-L18) · [go-linq Take](https://github.com/ahmetb/go-linq/blob/6ebc43b7c971a8a4cb9e38742f6cb491d4032cbb/take.go#L3-L23) · [Reference provenance and licenses](../../../THIRD_PARTY_NOTICES.md)

GoEF accepts a non-nil `iter.Seq[T]` and a Go `int` count. Construction performs no pulls. Each positive operator counts down per enumeration, holds constant auxiliary state and preserves the source's order, values and replay behavior. `Skip` consumes up to the requested number of items before yielding. `Take` completes immediately after the last requested item even if the consumer would accept more. A shorter source yields only the eligible values it contains. A downstream stop reaches the original source, allowing its cleanup to finish before iteration returns. A finite `Take` yields a bounded prefix of an infinite source.

## Executable verification

`TestSkipTakeStopsAndReplays` checks deferred composition of `Take(Skip(source, 2), 3)`, exact read counts, repeated enumeration, early consumer stop and source cleanup. `TestTakeNonPositiveDoesNotStartSource`, `TestSkipTakeCountBoundaries` and `TestSkipTakeRetainsSingleUseSource` cover empty and short sources, minimum and maximum `int` counts, typed nil and duplicate values, zero reads for nonpositive take, and remaining values in a single-use source. [Go contract tests](../../../linq/objects/paging_test.go)

On 2026-11-26 with Go 1.27.4 on Windows, the new tests failed to compile against `c9a438928cc25b0c8ee8fde7bf78e27d037d6e65` because the APIs were absent. A temporary `Take` that checked its limit only after requesting an item caused `TestTakeNonPositiveDoesNotStartSource` and `TestSkipTakeStopsAndReplays` to fail with exit 1. The final implementation passed `go test -mod=readonly -count=1 -shuffle=on -coverprofile=<profile> ./linq/objects` and `go vet -mod=readonly ./linq/objects` on Windows/amd64. The same suite passed on Windows/386 with `GOARCH=386` and `CGO_ENABLED=0`. It covered 61/62 package statements on amd64 and 62/62 on 386, with all 20 Skip and Take statements covered on both. In the pinned go-linq checkout, `go test -mod=readonly -count=1 -run '^Test(Skip|Take)$' -v ./...` exited 0.

## Local differential match

GoEF, stable go-linq at `6ebc43b7c971a8a4cb9e38742f6cb491d4032cbb`, and .NET runtime 10.0.0 agreed on every value and source lifecycle counter below. Each case used a fresh iterator over 0 through 7 on Windows/amd64.

| Sequence operation | Output | Source starts | Source pulls | Source cleanups |
| --- | --- | --- | --- | --- |
| Skip 2, take 3 | 2, 3, 4 | 1 | 5 | 1 |
| Skip 2, take 3 and stop after the first value | 2 | 1 | 3 | 1 |
| Skip 2, take 0 | Empty | 0 | 0 | 0 |
| Skip -1, take 3 | 0, 1, 2 | 1 | 3 | 1 |
| Skip 7, take 4 | 7 | 1 | 8 | 1 |

The temporary fixture used `go run -mod=readonly .` with local replacements for both Go checkouts. .NET SDK 10.0.402 built the corresponding fixture with `dotnet build fixture.csproj --source <fixture directory>`. It ran with the pinned runtime using `dotnet exec --fx-version 10.0.0 fixture.dll`. The commands and trace comparisons exited 0.
