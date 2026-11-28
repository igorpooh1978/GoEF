# Generate a typed LINQ binding

Go 1.27.4 and the GoEF module are sufficient to generate a binding for one equality filter and one scalar projection. Try the [customer example](../../examples/linq/generated/query.go) from a GoEF checkout. [Issue #24](https://github.com/igorpooh1978/GoEF/issues/24)

```sh
go generate ./examples/linq/generated
go test -run '^Example$' -count=1 -v ./examples/linq/generated
```

The example calls one generated function with `"London"` and `"Berlin"`. The two invocations have equal shapes and distinct arguments. Reading the projected field of a supplied Berlin customer produces `Ada`.

```go
//goef:query FindCustomer
func findCustomer(row Customer, city string) (bool, string) {
	return row.City == city, row.Name
}
```

Add a `//goef:query EntryPoint` doc comment to a function in a regular Go source file. Declare its named row struct and any named scalar types in that file. Use a direct comparison `row.Field == argument` and return one direct field as the second result. Use the same Go type on both operands and for the projection field and result. Supported scalars are bool, string and the signed Go integer types, including their named forms. Use regular source and output filenames without platform suffixes. The declaration file is self-contained, with no imports or build constraints.

From the containing package, generate and then check the output. The GoEF module must be present in the consumer's module graph.

```sh
go run github.com/igorpooh1978/GoEF/cmd/goef-linqgen -source query.go -output query_gen.go
go run github.com/igorpooh1978/GoEF/cmd/goef-linqgen -source query.go -output query_gen.go -check
```

Commit the output and run the second command in CI. The generator normalizes line endings and fingerprints the source bytes. The same declaration file produces identical output regardless of checkout path. `-check` exits 1 if the output is missing or differs from a fresh generation, including manual edits. A rejected expression reports the source file, line and column and leaves the output intact. Regeneration only replaces a regular file with the generator's header.

To identify an executable built from the current source, run the version command by itself.

```sh
goef-linqgen -version
```

The command writes one line to standard output. A generator installed from a module tag prints `goef-linqgen` followed by its module version. Builds from Git add the revision and dirty state when Go recorded them. The version is `(devel)` when the module has no embedded version. See [Build and preview versions](../development/BUILD_VERSIONS.md) for source builds and release verification. [Issue #38](https://github.com/igorpooh1978/GoEF/issues/38)

Call the generated `FindCustomer(city)` each time you want to bind a new value. Its `Shape()` gives the Go field, argument and projection names. Its `Value()` retains the argument passed during that call. Generated bindings can be reused across goroutines, and each call owns its scalar value. `Project(row)` reads the selected field from an existing Go row supplied by the caller.

This generated frontend produces typed data for a single source row and a single non-null comparison. Query IR integration, provider translation and terminal database execution are planned in the [query architecture](../architecture/QUERY_ENGINE.md). For executable local LINQ pipelines, see [LINQ Objects](LINQ_OBJECTS.md).
