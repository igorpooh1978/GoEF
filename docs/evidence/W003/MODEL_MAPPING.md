
# Code First table and column mapping

[Issue #3](https://github.com/igorpooh1978/GoEF/issues/3) · [Model contract](../../architecture/MODEL_ENGINE.md) · [Code First guide](../../user/MODELS_CODE_FIRST.md) · [Gherkin](../../../specs/features/model/model-map-001.feature)

## Upstream reference and Go behavior

EF Core v10.0.15 at `b34c0e9d7e768aa808ded44f19e7f871bea6869f` distinguishes explicit configuration, data annotations and conventions. Its relational property builder configures column names independently of other facets. The property builder tests confirm that a data annotation can replace the maximum length previously set by a data annotation, while a convention cannot replace it. The relational model tests exercise explicit and conventional table mappings. [Configuration precedence](https://github.com/dotnet/efcore/blob/b34c0e9d7e768aa808ded44f19e7f871bea6869f/src/EFCore/Metadata/ConfigurationSourceExtensions.cs#L24-L46) · [Column builder](https://github.com/dotnet/efcore/blob/b34c0e9d7e768aa808ded44f19e7f871bea6869f/src/EFCore.Relational/Extensions/RelationalPropertyBuilderExtensions.cs#L24-L31) · [Maximum length tests](https://github.com/dotnet/efcore/blob/b34c0e9d7e768aa808ded44f19e7f871bea6869f/test/EFCore.Tests/Metadata/Internal/InternalPropertyBuilderTest.cs#L81-L105) · [Table mapping tests](https://github.com/dotnet/efcore/blob/b34c0e9d7e768aa808ded44f19e7f871bea6869f/test/EFCore.Relational.Tests/Metadata/RelationalModelTest.cs#L41-L58) · [Pinned MIT license](https://github.com/dotnet/efcore/blob/b34c0e9d7e768aa808ded44f19e7f871bea6869f/LICENSE.txt)

GoEF takes explicit registrations and resolves the table name, column names and maximum lengths before exposing metadata. A fluent option overrides the corresponding property tag while other facets remain intact. GoEF rejects unequal settings at the same precedence. It reports numeric coordinates for both declarations, even if a higher-priority setting overrides their facet. Equal settings at one precedence coalesce. This preserves the GoEF rule for detecting contradictions regardless of registration order.

The version 1 JSON mapping snapshot sorts entities by exact name and preserves field order and composite key order. It records names, lengths, direct pointer nullability and ordered keys. Mapping and snapshot copies are independent of the caller's key and option slices and of copies previously returned.

## Executable evidence

| Scenario | Direct Go test | In-scope result |
| --- | --- | --- |
| MODEL-MAP-001-S02 | `TestMapMODEL_MAP_001S02` | Fluent table and column names resolve, the tagged length survives and both registration orders produce identical versioned bytes. The published mapping stays stable after mutations of input slices and returned metadata. |
| MODEL-MAP-001-S03 | `TestMapMODEL_MAP_001S03` | Duplicate effective columns prevent publication. The portable error locates both fields without the raw name. |
| MODEL-MAP-001-S01 | Provider design-time generation | Remains specified for all four providers. It requires schema inspection, provider-bound types and generated Go files. |

The exact S02 and S03 tests first failed to compile against baseline `347a3ff31e0c3c0ff77634abafb0207e4d6a6d40` before the mapper API existed. A behavioral control then forced the original Go field name into the metadata and bypassed duplicate-column detection. On Windows/amd64 with Go 1.27.4, `go test -mod=readonly -count=1 -run '^TestMapMODEL_MAP_001S0[23]$' ./model` exited 1. S02 reported the wrong resolved column names and S03 observed an invalid published mapping. The restored implementation passed the same command with exit 0.

The workstation run of `go test -mod=readonly -shuffle=on -count=1 -cover ./model ./goeferr` exited 0. The packages reported 98.7% and 100% statement coverage respectively. The bounded `go test -mod=readonly -count=1 -run '^$' -fuzz '^FuzzMapColumnNames$' -fuzztime=3s -parallel=2 ./model` probe exited 0 on the same implementation and checked exact identifier preservation through JSON and typed rejection of invalid identifiers. `ExampleMap` compiles the guide example and verifies `customers customer_name 60`. [Model mapping tests](../../../model/mapping_test.go)

## Integrated verification

Revision `a425e2baed049da3c48c7d70625ed4b2ad930627` combines Code First mapping and `Where` → `Select`. The Windows/amd64 workstation used Go 1.27.4 on 2026-11-26. Independent reviewers checked the exact integrated code, the new scenario assertions, and the scope recorded in both registries. [Pipeline evidence](../W008/OBJECTS_SELECT.md)

- `go test -mod=readonly -shuffle=on -count=1 -coverprofile=<profile> ./goeferr ./model ./linq/objects` exited 0. The affected packages covered 254 of 257 statements. Coverage was 12 of 12 for `goeferr`, 232 of 235 for `model` and 10 of 10 for `linq/objects`.
- `go test -mod=readonly -count=1 -run '^$' -fuzz '^FuzzMapColumnNames$' -fuzztime=5s -parallel=2 ./model` exited 0 after 20,429 executions. The three complete guide programs ran with `go run -mod=readonly` and printed `1 1 1`, `customers customer_name 60` and `even: 2`.
- `go -C <fixture> test -mod=readonly -count=1 -shuffle=on ./...` exited 0 for `test/bootstrap/go-feasibility`, `test/architecture/linq-compile` and `test/architecture/provider-spi`.
- `pwsh -NoProfile -File scripts/ci/goef-w002.ps1 -Compatibility -AcceptSqlServerEula -AcceptLegacySqlServerEula` exited 0 in the detached worktree. It ran the bootstrap, build, vet and module checks and passed 84 distinct driver scenario results across 12 engine profiles. The repeated SQLite 3.54.1 profile brought the total to 91 passing driver rows. Four file tests covered SQLite 3.51.3 and 3.54.1 in both directions with DELETE and WAL. All engine checks passed on the first run. [Engine coverage](../W002/LEGACY_COMPATIBILITY.md)
- In a network-isolated Linux/amd64 container on the same workstation with Go 1.27.4, `go test -mod=readonly -race -count=1 -shuffle=on ./...` exited 0 for the root module. In the driver lab module, `go test -mod=vendor -race -count=1 -shuffle=on ./...` exited 0 using the pinned local vendor snapshot.
