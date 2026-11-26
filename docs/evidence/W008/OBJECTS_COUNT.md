# LINQ Objects Count

[Issue #4](https://github.com/igorpooh1978/GoEF/issues/4) · [LINQ Objects guide](../../user/LINQ_OBJECTS.md) · [Object scenarios](../../../specs/features/linq/objects.feature) · [Runnable example](../../../linq/objects/count_example_test.go)

## Pinned behavior and GoEF contract

The .NET 10.0.0 `Enumerable.Count` returns a signed 32-bit count and checks overflow when it enumerates. In a composed filter and projection, its iterator counts matches and runs the selector. The pinned tests cover empty sequences and repeated counts. The stable go-linq v5.0.0 `Query.Count` enumerates using an unchecked Go `int`. Its `TestCount` covers finite and empty inputs. [Enumerable Count](https://github.com/dotnet/runtime/blob/60629d14374c56f1cb51819049ad1fa529307f8d/src/libraries/System.Linq/src/System/Linq/Count.cs#L11-L44) · [Enumerable filter and projection](https://github.com/dotnet/runtime/blob/60629d14374c56f1cb51819049ad1fa529307f8d/src/libraries/System.Linq/src/System/Linq/Where.SpeedOpt.cs#L563-L590) · [Enumerable Count tests](https://github.com/dotnet/runtime/blob/60629d14374c56f1cb51819049ad1fa529307f8d/src/libraries/System.Linq/tests/CountTests.cs#L12-L62) · [go-linq Count](https://github.com/ahmetb/go-linq/blob/6ebc43b7c971a8a4cb9e38742f6cb491d4032cbb/result.go#L63-L71) · [go-linq Count tests](https://github.com/ahmetb/go-linq/blob/6ebc43b7c971a8a4cb9e38742f6cb491d4032cbb/result_test.go#L77-L92)

GoEF accepts a non-nil, finite `iter.Seq[T]`, visits each yielded value and returns an architecture-sized `int`. A count includes duplicates and typed nil values, completes every invoked callback before returning, uses constant counter space and panics on overflow. The independent implementation follows GoEF's existing generic function style. [Reference licenses and provenance](../../../THIRD_PARTY_NOTICES.md)

## Executable verification

`TestWhereSelectCountConsumesAllProjections` directly checks `LINQ-OBJECTS-001-S06`. The Go test verifies how many times the iterator starts and cleans up, all values observed by each callback and the count across two enumerations. Additional Go tests cover empty and single-use sources, typed nil and duplicate values. [Count tests](../../../linq/objects/count_test.go)

On the Windows/amd64 workstation with Go 1.27.4 on 2026-11-26, baseline `9b5defe005efdc9adcbbb63daabbc261818bdf85` failed to compile the Count test because the function was absent. A temporary implementation returning zero made `go test -mod=readonly -count=1 -run '^TestWhereSelectCountConsumesAllProjections$' ./linq/objects` exit 1 with count 0 rather than 3. The implemented counter passed `go test -mod=readonly -count=1 -shuffle=on -cover -v ./linq/objects` with 13 tests, two runnable examples and 15 of 16 statements covered.

The same workstation also ran the suite with `GOARCH=386` and `CGO_ENABLED=0`. All 14 tests and both examples passed with 16 of 16 statements covered. The 32-bit public boundary test exercised the actual overflow after 2,147,483,648 values and observed the source cleanup. [Overflow test](../../../linq/objects/count_386_test.go)

## Local differential match

The same fixture ran in GoEF, stable go-linq v5.0.0 at `6ebc43b7c971a8a4cb9e38742f6cb491d4032cbb` and .NET runtime 10.0.0 on Windows/amd64. All three traces matched.

| Phase | Count | Starts | Source values | Predicate inputs | Selector inputs | Cleanups |
| --- | ---: | ---: | --- | --- | --- | ---: |
| After construction | | 0 | | | | 0 |
| First count | 3 | 1 | 1, 2, 4, 2, 3 | 1, 2, 4, 2, 3 | 2, 4, 2 | 1 |
| Second count | 3 | 2 | 1, 2, 4, 2, 3 | 1, 2, 4, 2, 3 | 2, 4, 2 | 2 |

The temporary Go fixture used `go run -mod=readonly .` with local replacements for both checkouts. The installed .NET SDK 10.0.402 built the equivalent fixture locally using `dotnet build fixture.csproj --source <fixture directory>` and ran it with the previously verified runtime through `dotnet exec --fx-version 10.0.0 fixture.dll`. Both fixture runs and the reference `go test -mod=readonly -count=1 -run '^TestCount$' -v ./...` exited 0. These results cover the non-predicate in-memory Count and the tested filter and projection pipeline. [Prior runtime evidence](OBJECTS_SELECT.md) · [Runtime release metadata](https://builds.dotnet.microsoft.com/dotnet/release-metadata/10.0/releases.json)

The integrated `7796de91e1ddb573a589078916d831eb05605312` retained the independently reviewed Objects source and S06 tests. Its local Windows/amd64 run covered 15 of 16 Objects statements, and the Windows/386 run covered all 16, including checked overflow and cleanup. The guide programs printed `even: 2` and `3`. [Integrated verification](../W003/PROVIDER_TYPE_BINDING.md#integrated-verification)
