# Adoption follow-up integrated evidence

Runtime and build candidate commit [`493db45c51d9a2acd1998fe413e1d9ae241aff2a`](https://github.com/igorpooh1978/GoEF/commit/493db45c51d9a2acd1998fe413e1d9ae241aff2a). The starting point was [`388e85cb27d3bebe9a4674b922e235658f647570`](https://github.com/igorpooh1978/GoEF/commit/388e85cb27d3bebe9a4674b922e235658f647570). [Migrations #35](https://github.com/igorpooh1978/GoEF/issues/35) · [Typed query #36](https://github.com/igorpooh1978/GoEF/issues/36) · [Transactions #37](https://github.com/igorpooh1978/GoEF/issues/37) · [Versions #38](https://github.com/igorpooh1978/GoEF/issues/38) · [Pool reuse #39](https://github.com/igorpooh1978/GoEF/issues/39)

## Integrated execution

On 2026-11-28, Go 1.27.4 on Windows/amd64 and a Linux/amd64 local Docker daemon ran the complete compatibility command. It exited 0. The 12 distinct profiles covered PostgreSQL 16.16, 17.12, 18.7 and 19.0, SQLite 3.51.3 and 3.54.1, MySQL 8.0.46 and 8.4.13.1, and SQL Server 2017, 2019 and 2022 Developer at the documented compatibility levels. All 36 distinct session reuse scenario cells passed. The command also ran the repository validator, Python unit suites, Go build, vet and module checks, shuffled Go tests, and the selected provider suites. [Pinned engines](../../parity/ENGINE_COMPATIBILITY.md) · [Four-provider session tests](FOUR_PROVIDER_SESSION_REUSE.md)

```sh
python -B scripts/ci/goef_w002.py --compatibility --accept-sqlserver-eula --accept-legacy-sqlserver-eula --model-types --owned-tx --savepoint-tx --save-changes --tx-options --decimal-binding --commit-outcome --production-adapters
```

The native PostgreSQL 18.7 Windows runner passed three repetitions of all five transaction scenarios, including the explicit cancellation and deadline variants of the observed advisory-lock wait. Independent acceptance reproduced the incorrectly classified deadline on the earlier integrated build. The final test verified that the deadline stays distinguishable from cancellation and the unusable session is discarded. Independent Go 1.27.4 Linux/amd64 race-instrumented runs also passed the native PostgreSQL tests and the driver lab's production adapter, direct and wrapped pooling, Queryable and migration checks on the pinned PostgreSQL 18.7 image. The core's shuffled Linux race test and the Windows Go vulnerability checks exited 0. [Native transactions](../W005/POSTGRES_NATIVE_TRANSACTIONS.md) · [Migrations](../W006/POSTGRES_MIGRATIONS.md) · [Typed projection](../W008/POSTGRES_EQUALITY_READ.md)

| Owned module or package | Covered statements | Coverage source |
| --- | ---: | --- |
| Core module | 1831/1900 (96.37%) | Windows unit suite |
| Core transaction package | 533/553 (96.38%) | Core unit and native PostgreSQL live suite |
| PostgreSQL adapter and compiler | 85/85 (100%) | Adapter unit, native and driver-lab suite union |
| PostgreSQL migration package | 134/140 (95.71%) | Live migration suite |
| PostgreSQL module total | 219/225 (97.33%) | Both owned PostgreSQL packages |
| SQLite module | 21/22 (95.45%) | Windows unit suite |
| MySQL module | 23/24 (95.83%) | Windows unit suite |
| SQL Server module | 30/30 (100%) | Windows unit suite |

The read-only reviewer accepted the tested source after the unsupported lease cleanup, private cancellation cause and native lifetime deadline findings were fixed. The PostgreSQL microbenchmark had zero physical reopens for reviewed reads, versus approximately one per ordinary query. The four-provider acceptance uses physical opens and result correctness rather than a timing threshold. The application's PgBouncer path and production latency remain unmeasured. Automatic import of history without digests and wider LINQ translation remain outside the implemented slices. [PostgreSQL benchmark](POSTGRES_SESSION_REUSE.md)

## Build candidate

Go 1.27.4 built Windows/amd64 and Linux/amd64 generator binaries from the exact clean source and an isolated `v0.2.0-alpha.1` tag. Both binaries reported their full source revision and `dirty=false`, with matching `go version -m` output. A separate local release rehearsal resolved all five candidate Go modules without replacement directives and built an independent consumer. The public prerelease continues to use the existing `v0.1.0-alpha.1` release tags. [Build and preview version procedure](../../development/BUILD_VERSIONS.md)

| Build | SHA-256 |
| --- | --- |
| Windows/amd64 | `9ee9f1c91b73e37d1c49c6b51a1c5677d3881d09141ffe3a930d4b3c29da4ac4` |
| Linux/amd64 | `b025b33ece554eea7e49eb337452917c36f6fa26974e8e97d58f62c10d73d7ee` |

## Public source build

The tested source is available by commit. `python -B test/architecture/public-module/verify.py 493db45c51d9a2acd1998fe413e1d9ae241aff2a --generated-bindings --generator-version` exited 0 in a fresh external module with Go 1.27.4. The public Go proxy and checksum database resolved `v0.1.0-alpha.1.0.20261128074836-493db45c51d9` at the expected origin commit. The archive checksum was `h1:VU4/6zZNXYvy5Vef1azVH00pLFHs1nh/xgqj/xf1R7g=` and the manifest checksum was `h1:ABeTL0NnLBe42NYSr8liTicW7AUWHsszrL7EzWFpFF0=`. Generated bindings compiled and the installed generator reported the selected source version. The four provider modules pin that source revision in their manifests so consuming applications resolve the same core without local replacements.

Each pinned provider module passed `go mod verify`, `go mod tidy -diff`, build, vet and shuffled tests in the checkout. Separate temporary copies dropped their local core replacements and ran `go mod tidy`, `go mod verify`, build and tests with the public module. All four selected the exact core source version and exited 0.
