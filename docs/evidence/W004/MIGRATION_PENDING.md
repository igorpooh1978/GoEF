# Validate a migration lineage in memory

[Issue #34](https://github.com/igorpooh1978/GoEF/issues/34) · [Implementation](../../../migration/pending.go) · [Go guide](../../user/MIGRATIONS_IN_GO.md) · [Scenario MIG-DRIFT-001-S03](../../../specs/features/migrations/mig-drift-001.feature)

## Upstream and contract

The pinned EF Core v10.0.15 migration service targets an installed migration or the latest available migration. Its internal planner partitions assembly entries by case-insensitive membership in the applied-ID set and orders each planned list for the target. Each history row carries a migration ID and EF Core product version. The infrastructure tests assert the ordered IDs after migrating to the latest version or to an explicit intermediate target. The pin is MIT-licensed. This Go implementation uses the upstream behavior as a reference. [Migration service](https://github.com/dotnet/efcore/blob/b34c0e9d7e768aa808ded44f19e7f871bea6869f/src/EFCore.Relational/Migrations/IMigrator.cs#L25-L37) · [PopulateMigrations](https://github.com/dotnet/efcore/blob/b34c0e9d7e768aa808ded44f19e7f871bea6869f/src/EFCore.Relational/Migrations/Internal/Migrator.cs#L465-L524) · [HistoryRow](https://github.com/dotnet/efcore/blob/b34c0e9d7e768aa808ded44f19e7f871bea6869f/src/EFCore.Relational/Migrations/HistoryRow.cs#L12-L38) · [Infrastructure tests](https://github.com/dotnet/efcore/blob/b34c0e9d7e768aa808ded44f19e7f871bea6869f/test/EFCore.Relational.Specification.Tests/Migrations/MigrationsInfrastructureTestBase.cs#L65-L138) · [License](https://github.com/dotnet/efcore/blob/b34c0e9d7e768aa808ded44f19e7f871bea6869f/LICENSE.txt)

GoEF requires a single ordered lineage. It verifies all registered definitions, including the unapplied suffix. Applied history must contain the exact initial entries by byte-for-byte ID and predecessor and matching nonzero SHA-256 digest. The caller supplies the canonical artifact digest. The planner returns an independently owned suffix in the supplied order. Conflicts produce `ErrMigrationConflict` with only portable classification details. This provider-neutral function compares caller-supplied values. Live inspection, migration journals and execution belong to the migration engine.

## Verification

Baseline `dc13ff64370f7e7348f0f7958dbf996819177a90`. Windows/amd64 used Go 1.27.4. The race suite used the locally cached Go 1.27.4 Linux/amd64 image (`sha256:5f1b98fa02fe396ebce2a0227a0e08309e11792915b9dd2e1f71053d7e7e6666`) with no network and a read-only checkout mount. All data in the test and example is synthetic.

| Check | Command | Result |
| --- | --- | --- |
| RED | `go test -mod=readonly -count=1 -run '^TestPendingMIG_DRIFT_001S03$' ./migration` | Exit 1 with the test on the baseline, which has no migration implementation. Exit 1 again with the API signature and conflict detail present and a temporary no-op planner. Test assertions rejected missing pending entries and missing conflicts. |
| GREEN and example | `go test -mod=readonly -count=1 -run '^(TestPendingMIG_DRIFT_001S03\|ExamplePending)$' ./migration` | Exit 0. The example prints the pending entry and handles the altered digest by stable sentinel. |
| Build and vet | `go build -mod=readonly ./...` and `go vet -mod=readonly ./...` | Both exited 0. |
| Shuffled root suite | `go test -mod=readonly -shuffle=on -count=1 -coverprofile=<profile> ./...` | Exit 0. Root statement coverage 960/1000 (96.0%). The existing provider package measured 8/21 (38.1%). |
| Planner and error coverage | `go test -mod=readonly -count=1 -shuffle=on -coverpkg=./migration,./goeferr -coverprofile=<profile> ./migration ./goeferr` | Exit 0. Merged coverage across the two test binaries covered 14/14 migration statements and 24/24 error statements. |
| Race | `CGO_ENABLED=1 go test -mod=readonly -race -shuffle=on -count=1 ./...` | Exit 0 in the isolated Linux container. |

The checked Go blobs are `55e456923e3db6187604c778048e0248cdf5fe26` for the planner, `4de672baaaec442400fdb674eda84e30ace55523` for its tests, `a1fd87fadc7b4e7fad222b4a819ca3a7a6889a20` for the runnable example and `187fddadbe7c32ca0e3eca6f9ba77137b9f87c97` for the portable error detail. [Contract test](../../../migration/pending_test.go) · [Runnable example](../../../migration/example_test.go) · [Error registry](../../errors/README.md)

The executable cases cover empty and complete history, registry order, independent output, duplicate and malformed IDs, a broken predecessor or missing digest anywhere in the registry, and unknown, skipped, reordered or changed applied entries. They verify case sensitivity and the absence of normalization or redaction leaks. Real-engine tests apply to durable history and execution in [issue #13](https://github.com/igorpooh1978/GoEF/issues/13).

## Integrated acceptance

The independent review confirmed the planner and error contract on `36d7e36e7ba9938c20db56d019b9e6c72db8a0a2`. Windows Go 1.27.4 build, vet, shuffled tests, the runnable example and the bootstrap checks passed. Linux/amd64 Go 1.27.4 with cgo passed race detection for all six root packages. The planner covered 14/14 statements. Merged planner and portable error coverage was 38/38. The aggregate root coverage was 980/1007 (97.3%), including 21/21 transport statements after the separate provider contract tests.

The corrected upstream references were included at `32a79ef3a39a6ea6c33f32389264d1b9e472f8e2`. Its Windows root test and coverage command exited 0, preserved the planner's 14/14 statements, and covered 1007/1034 root statements (97.4%). The Linux root race command also exited 0 on that commit. The reviewed planner, test and example blobs are identical at both revisions.
