# LINQ Objects quantifiers

[Issue #4](https://github.com/igorpooh1978/GoEF/issues/4) · [LINQ Objects guide](../../user/LINQ_OBJECTS.md) · [Object scenarios](../../../specs/features/linq/objects.feature) · [Runnable example](../../../linq/objects/quantifiers_example_test.go)

## Pinned behavior and GoEF contract

The .NET 10.0.0 `Enumerable.Any` checks for a first value, `Enumerable.Any(source, predicate)` stops at the first match, and `Enumerable.All` stops at the first failure. The predicate overloads evaluate elements in encounter order. On empty input both forms of `Any` return `false`, and `All` returns `true`. Stable go-linq v5.0.0 exposes the same decisions as `Any`, `AnyWith` and `All`. [Enumerable implementation](https://github.com/dotnet/runtime/blob/60629d14374c56f1cb51819049ad1fa529307f8d/src/libraries/System.Linq/src/System/Linq/AnyAll.cs#L11-L108) · [Any tests](https://github.com/dotnet/runtime/blob/60629d14374c56f1cb51819049ad1fa529307f8d/src/libraries/System.Linq/tests/AnyTests.cs) · [All tests](https://github.com/dotnet/runtime/blob/60629d14374c56f1cb51819049ad1fa529307f8d/src/libraries/System.Linq/tests/AllTests.cs) · [go-linq quantifiers](https://github.com/ahmetb/go-linq/blob/6ebc43b7c971a8a4cb9e38742f6cb491d4032cbb/result.go#L7-L41) · [go-linq tests](https://github.com/ahmetb/go-linq/blob/6ebc43b7c971a8a4cb9e38742f6cb491d4032cbb/result_test.go#L8-L62)

GoEF's `Any`, `AnyBy` and `All` are terminal generic functions over a non-nil `iter.Seq[T]`. `AnyBy` names the predicate overload to express that overload as a distinct Go function. Predicates must be non-nil. Any yielded value counts as present, including a Go zero value or a typed nil. Every call starts its source once and forwards early termination through the iterator. The source owns replayability and cleanup. Each operator uses constant auxiliary space and can decide an infinite source as soon as it sees a decisive element. A case that requires exhaustion can run indefinitely on an infinite source.

The .NET implementation uses the MIT license and go-linq's pinned snapshot uses Apache-2.0. GoEF's generic range loops and fixtures were written independently from those behavioral references and require only the standard library. [Reference provenance](../../../THIRD_PARTY_NOTICES.md)

## Executable verification

`TestQuantifiersShortCircuitThroughPipeline` verifies pull, filter, projection and predicate counts, both terminal outcomes, source cleanup before return, and repeat enumeration with a replayable source. Other tests cover empty inputs, zero and typed nil values, absence of matches, universal success, infinite prefixes, and consumption of a single-use source. [Go contract tests](../../../linq/objects/quantifiers_test.go)

On 2026-11-26 with Go 1.27.4 on Windows, the fixtures failed to compile against baseline `85799b679601610a1fb72971b74c38a40d9f7224` because the APIs were absent. With temporary implementations that used `Count`, `go test -mod=readonly -count=1 -run '^TestQuantifiersShortCircuitThroughPipeline$' ./linq/objects` exited 1. All three terminals read all six values. The restored short-circuit implementation passed the full Objects suite and runnable examples on Windows/amd64 and Windows/386 (`GOARCH=386`, `CGO_ENABLED=0`) with `go test -mod=readonly -count=1 -shuffle=on -v -coverprofile=<profile> ./linq/objects`. Package coverage was 26/27 statements on amd64 and 27/27 on 386. Quantifiers covered all 11 statements on both architectures. `go vet -mod=readonly ./linq/objects` and `go test -mod=readonly -count=1 -run '^Test(All|Any|AnyWith)$' -v ./...` in the pinned go-linq checkout exited 0.

## Local differential match

GoEF, stable go-linq at `6ebc43b7c971a8a4cb9e38742f6cb491d4032cbb`, and .NET runtime 10.0.0 produced identical results and traces on Windows/amd64. Each terminal used a fresh sequence of 1 through 6, kept the even values, multiplied them by 10, and used the predicate `value >= 40` where applicable. The source started and cleaned up once for each terminal.

| Terminal | Result | Source pulls and filter inputs | Projection inputs | Terminal predicate inputs |
| --- | --- | --- | --- | --- |
| `Any` | `true` | 1, 2 | 2 | |
| `AnyBy` | `true` | 1, 2, 3, 4 | 2, 4 | 20, 40 |
| `All` | `false` | 1, 2 | 2 | 20 |

The temporary fixture ran `go run -mod=readonly .` with local replacements for the GoEF and go-linq checkouts. .NET SDK 10.0.402 built the equivalent fixture with `dotnet build fixture.csproj --source <fixture directory>`. The pinned runtime ran it with `dotnet exec --fx-version 10.0.0 fixture.dll`. All commands exited 0. The final comparison checked the Boolean result, ordered trace and source lifecycle counts for each operation.
