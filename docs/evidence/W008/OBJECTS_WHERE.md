# Local LINQ Objects Where prototype

[Issue #4](https://github.com/igorpooh1978/GoEF/issues/4) · [LINQ Objects contract](../../architecture/LINQ_OBJECTS.md) · [Object scenarios](../../../specs/features/linq/objects.feature)

Local revision `3c6378ab18f06a313228f8a31ba493d3da4b80ed` defines a generic filter over `iter.Seq[T]` using the Go standard library. The pinned .NET Enumerable and go-linq `Where` implementations and tests supply behavioral references. The Go prototype was written independently. [Pinned sources](../../upstream/lock.json)

Building the filtered sequence consumes zero inputs. Stopping at the first even number in `1, 2, 3, 4` observes value `2` after exactly two pulls. The prototype forwards the stop signal to the source, preserves encounter order and duplicates, and starts a fresh enumeration for a replayable source. The eager control failed the zero-pull and repeated-enumeration checks before the lazy implementation passed.

On the Windows/amd64 workstation with Go 1.27.4, `go build ./...`, `go vet ./...` and five Go tests passed. `Where` covered six of six statements. On the same workstation, Ubuntu WSL with verified Go 1.27.4 Linux/amd64 passed `go test -p=2 -mod=readonly -race -count=1 -shuffle=on ./linq/objects` with `GOPROXY=off`. Independent review confirmed the exact early-stop and cleanup assertions. The production Objects contract, broader operator parity and the [project license decision](https://github.com/igorpooh1978/GoEF/issues/15) retain their own acceptance requirements.
