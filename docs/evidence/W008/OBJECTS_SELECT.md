# Initial LINQ Objects Select implementation

[Issue #4](https://github.com/igorpooh1978/GoEF/issues/4) · [LINQ Objects contract](../../architecture/LINQ_OBJECTS.md) · [Object scenarios](../../../specs/features/linq/objects.feature) · [Runnable example](../../../linq/objects/select_example_test.go)

## Pinned upstream behavior

The .NET runtime v10.0.0 Enumerable `Select` defers the selector until an input is available. Its iterator disposes the underlying enumerator when the consumer stops. `Select_SourceIsAnArray_ExecutionIsDeferred` checks deferred evaluation, `SelectProperty` covers type-changing projection with duplicates, and `SameResultsRepeatCallsStringQuery` covers replay. go-linq v5.0.0 selects each item through a lazy typed callback and forwards the consumer's stop signal. Its `TestSelect_TypeChanging` projects runes to strings. [Enumerable source](https://github.com/dotnet/runtime/blob/60629d14374c56f1cb51819049ad1fa529307f8d/src/libraries/System.Linq/src/System/Linq/Select.cs#L115-L155) · [Enumerable tests](https://github.com/dotnet/runtime/blob/60629d14374c56f1cb51819049ad1fa529307f8d/src/libraries/System.Linq/tests/SelectTests.cs#L37-L73) · [Deferred evaluation test](https://github.com/dotnet/runtime/blob/60629d14374c56f1cb51819049ad1fa529307f8d/src/libraries/System.Linq/tests/SelectTests.cs#L198-L205) · [go-linq source](https://github.com/ahmetb/go-linq/blob/6ebc43b7c971a8a4cb9e38742f6cb491d4032cbb/select.go#L16-L25) · [go-linq test](https://github.com/ahmetb/go-linq/blob/6ebc43b7c971a8a4cb9e38742f6cb491d4032cbb/select_test.go#L15-L22)

The independently written Go implementation follows the existing `Where` API by using a top-level generic function over `iter.Seq`, with a non-nil source and selector precondition. The .NET API validates null arguments at call time. GoEF leaves source ownership and replayability to the source iterator. [Upstream licenses and pin provenance](../../../THIRD_PARTY_NOTICES.md)

## Scenario and local verification

`TestWhereSelectStopsAtFirstProjectionAndReplays` directly exercises every assertion in `LINQ-OBJECTS-001-S05`. Construction performs zero pulls or callback calls. Stopping after `"2!"` consumes `1, 2`, calls the selector once and observes the source cleanup. Replay yields `"2!", "4!"` and cleans up again. Standalone checks cover encounter order, duplicates, a typed nil value, empty input and a single-use source. [Executable tests](../../../linq/objects/select_test.go)

The Windows/amd64 run used Go 1.27.4 on 2026-11-26, based on `347a3ff31e0c3c0ff77634abafb0207e4d6a6d40`. Before implementation, `go test -mod=readonly -count=1 ./linq/objects` exited 1 because `objects.Select` was undefined. A temporary eager implementation caused `go test -mod=readonly -count=1 -run '^TestWhereSelectStopsAtFirstProjectionAndReplays$' ./linq/objects` to exit 1. The source had already pulled all four inputs during construction. With the lazy implementation, `go test -mod=readonly -count=1 -shuffle=on -cover -v ./linq/objects` exited 0 with nine tests, one example and 100% statement coverage (4 of 4 statements in `Select` and 6 of 6 in `Where`). `go build -mod=readonly ./linq/objects` and `go vet -mod=readonly ./linq/objects` also exited 0. `ExampleSelect` and the guide source produced `even: 2`. A compiler probe assigned the string projection to `iter.Seq[int]` and exited 1 with a type mismatch.

## Local comparison with the references

The same fixture ran in GoEF, go-linq v5.0.0 at `6ebc43b7c971a8a4cb9e38742f6cb491d4032cbb`, and .NET runtime 10.0.0 on Windows/amd64. All three agreed on these observations.

| Phase | Output | Source pulls | Predicate calls | Selector calls | Total cleanups |
| --- | --- | ---: | ---: | ---: | ---: |
| After construction | | 0 | 0 | 0 | 0 |
| On the first yield | `2!` | 2 | 2 | 1 | 0 |
| After stopping | | 2 | 2 | 1 | 1 |
| Replay to completion | `2!,4!` | 4 | 4 | 2 | 2 |

The temporary Go fixture used `go run -mod=readonly .` and local module replacements for the two source trees. The upstream checkout also passed `go test -mod=readonly -count=1 -run '^TestSelect' -v ./...`. The installed .NET SDK 10.0.402 compiled an equivalent C# fixture with `dotnet build fixture.csproj --source <fixture directory>`. The SHA-512-verified Microsoft runtime was extracted into a temporary directory and ran the fixture with `dotnet exec --fx-version 10.0.0 fixture.dll`. All commands exited 0. The fixtures added no GoEF dependencies. [Microsoft runtime release metadata](https://builds.dotnet.microsoft.com/dotnet/release-metadata/10.0/releases.json)

This evidence covers the non-indexed local `Select` and its composition with `Where`. Indexed projection, the terminal count API and broader LINQ Objects parity retain their acceptance gates. S05 is executed by a Go test and remains unbound to a Gherkin runner.
