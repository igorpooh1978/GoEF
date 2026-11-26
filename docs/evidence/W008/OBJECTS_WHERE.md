# Initial LINQ Objects Where implementation

[Issue #4](https://github.com/igorpooh1978/GoEF/issues/4) · [LINQ Objects contract](../../architecture/LINQ_OBJECTS.md) · [Object scenarios](../../../specs/features/linq/objects.feature)

Local revision `3c6378ab18f06a313228f8a31ba493d3da4b80ed` defines a generic filter over `iter.Seq[T]` using the Go standard library. The pinned .NET Enumerable and go-linq `Where` implementations and tests supply behavioral references. The Go prototype was written independently. [Pinned sources](../../upstream/lock.json)

Building the filtered sequence consumes zero inputs. Stopping at the first even number in `1, 2, 3, 4` observes value `2` after exactly two pulls. The prototype forwards the stop signal to the source, preserves encounter order and duplicates, and starts a fresh enumeration for a replayable source. The eager control failed the zero-pull and repeated-enumeration checks before the lazy implementation passed.

On the Windows/amd64 workstation with Go 1.27.4, `go build ./...`, `go vet ./...` and five Go tests passed. `Where` covered six of six statements. On the same workstation, Ubuntu WSL with verified Go 1.27.4 Linux/amd64 passed `go test -p=2 -mod=readonly -race -count=1 -shuffle=on ./linq/objects` with `GOPROXY=off`. Independent review confirmed the exact early-stop and cleanup assertions.

The current `objects.Where` is available over non-nil `iter.Seq` sources and predicates. `TestWhereConsumesOnlyNecessaryPrefix` is a direct Go characterization of `LINQ-OBJECTS-001-S01`. Gherkin runner integration, the full Objects pipeline and broader operator parity retain their acceptance gates. [Runnable example](../../user/LINQ_OBJECTS.md) · [Where tests](../../../linq/objects/where_test.go)

Integrated revision `9594340c32ca6d6d7b5fc5bee29152c0209cdaed` contains the same `linq/objects` files. The focused test rerun on Windows/amd64 covered all six implementation statements and confirmed `iter` as the package's only direct import. The guide example compiled and printed `2`. [Integrated workstation verification](../W003/MODEL_KEY_VALIDATION.md#integrated-verification)
