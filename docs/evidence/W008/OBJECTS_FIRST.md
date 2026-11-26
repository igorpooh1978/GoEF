# First-value selection in LINQ Objects

[Issue #32](https://github.com/igorpooh1978/GoEF/issues/32) · [Objects guide](../../user/LINQ_OBJECTS.md) · [Scenarios](../../../specs/features/linq/objects.feature) · [Implementation](../../../linq/objects/first.go) · [Runnable examples](../../../linq/objects/first_example_test.go)

## Contract and provenance

`First(source)` returns its first yielded value and true, or the zero value of the element type and false after an empty traversal. `FirstOrDefault(source, fallback)` returns the supplied fallback only after the source completes without yielding a value. Both preserve yielded zero values and typed nils. They enumerate once, return after upstream cleanup, and use constant space. Replay and concurrency follow the source and callbacks. For predicate forms, compose a lazy `Where`. An unbounded input terminates as soon as it supplies a match. A search without a match needs exhaustion. Sources and supplied predicates must be non-nil.

.NET runtime v10.0.0 defines `Enumerable.First` to throw if the sequence has no elements or no match, and `FirstOrDefault` to yield the supplied or implicit default. In Go, check the presence flag from `First` and pass the desired default explicitly. The .NET tests cover repeated calls, missing values, predicate order, a run-once source and explicit defaults. Stable go-linq v5.0.0 returns a value and presence flag from `First` and `FirstWith`. A caller can apply its fallback to that result. The GoEF implementation and tests are original. They use the standard library, and the upstream pins serve as behavioral references under [MIT and Apache-2.0 provenance](../../../THIRD_PARTY_NOTICES.md). [Enumerable implementation](https://github.com/dotnet/runtime/blob/60629d14374c56f1cb51819049ad1fa529307f8d/src/libraries/System.Linq/src/System/Linq/First.cs) · [First tests](https://github.com/dotnet/runtime/blob/60629d14374c56f1cb51819049ad1fa529307f8d/src/libraries/System.Linq/tests/FirstTests.cs) · [FirstOrDefault tests](https://github.com/dotnet/runtime/blob/60629d14374c56f1cb51819049ad1fa529307f8d/src/libraries/System.Linq/tests/FirstOrDefaultTests.cs) · [Stable go-linq implementation](https://github.com/ahmetb/go-linq/blob/6ebc43b7c971a8a4cb9e38742f6cb491d4032cbb/result.go#L87-L114) · [Stable go-linq tests](https://github.com/ahmetb/go-linq/blob/6ebc43b7c971a8a4cb9e38742f6cb491d4032cbb/result_test.go#L106-L127)

## Executable verification

At baseline `dc13ff64370f7e7348f0f7958dbf996819177a90`, `go test -mod=readonly -count=1 -run '^TestFirst' ./linq/objects` exited 1 because both API functions were missing. With a temporary exhaustive implementation, `go test -mod=readonly -count=1 -run '^TestFirst(Stops|Preserves)' ./linq/objects` exited 1. Its extra read consumed the value after the first match, and its first call exhausted the single-use source. [Contract tests](../../../linq/objects/first_test.go)

On 2026-11-26, Go 1.27.4 on Windows/amd64 and Windows/386 ran `go test -mod=readonly -count=1 -shuffle=on -coverprofile=<profile> ./linq/objects` with exit 0. Coverage was 82/83 Objects statements on amd64 and 83/83 on 386. Both architectures covered all 7 new statements. `go vet -mod=readonly ./linq/objects` and `go test -mod=readonly -count=1 -shuffle=on ./...` exited 0 on Windows/amd64. With GCC 13.3.0 and Go 1.27.4 on Ubuntu WSL/linux-amd64, `CGO_ENABLED=1 go test -mod=readonly -race -count=1 -shuffle=on ./linq/objects` exited 0. The Windows environment lacked a C compiler for the Windows race build. The race result is from WSL.

The root package set also passed `go build -mod=readonly ./...` and `go vet -mod=readonly ./...` on Windows/amd64 and `CGO_ENABLED=1 go test -mod=readonly -race -count=1 -shuffle=on ./...` on Ubuntu WSL. `govulncheck ./linq/objects` exited 0 with no findings. `python test/bootstrap/verify.py` validated 50 features, 627 operator records and 79 scenarios with exit 0. `python -B -m unittest discover -s test/bootstrap -p test_verify.py` ran seven validator tests with exit 0.

In the pinned go-linq checkout, `go test -mod=readonly -count=1 -run '^TestFirst(With)?$' -v ./...` exited 0.

## Local differential comparison

On Windows/amd64, GoEF's two-stage filter and terminal, stable go-linq's `FirstWith` with a caller-applied fallback, and .NET 10.0.0's explicit-default predicate overload agreed on results and reads. Each case started and closed its source exactly once. The temporary Go fixture ran `go run -mod=readonly .` with local replacements for the GoEF and pinned go-linq sources. .NET SDK 10.0.402 built the same workload with `dotnet build fixture.csproj --source <fixture directory> --nologo`. The pinned runtime executed it with `dotnet exec --fx-version 10.0.0 fixture.dll`. All three commands exited 0.

| Input | Result of first even value with fallback -1 | Values read |
| --- | ---: | --- |
| 3, 5, 0, 2 | 0 | 3, 5, 0 |
| 3, 5 | -1 | 3, 5 |
| Empty | -1 | None |

A temporary benchmark found the first match at position 511 in a slice of 1,024 integers. On Windows/amd64 with an Intel Core Ultra 9 185H, `go test -mod=readonly -run '^$' -bench '^BenchmarkFirst$' -benchmem -count=3 -benchtime=100ms .` exited 0. Median times were 216.2 ns/op for a direct loop, 284.3 ns/op for GoEF `Where` plus `First`, and 1,256 ns/op for stable go-linq `FirstWith`. The direct loop and GoEF each reported zero allocations. go-linq reported 64 B/op in two allocations. These measurements describe this local workload.
