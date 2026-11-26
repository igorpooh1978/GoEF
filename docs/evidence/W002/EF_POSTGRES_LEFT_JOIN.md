# EF Core and pgx left join on PostgreSQL 18.7

On 2026-11-26 UTC, EF Core and native pgx returned the same six ordered pairs for tenant 1 and the same two for tenant 2. The comparison exercised duplicate right keys, a missing match, NULL, case-sensitive text and rebinding the tenant parameter. It supplies the PostgreSQL reference workload for [W002 #13](https://github.com/igorpooh1978/GoEF/issues/13). The [application scenario LINQ-CONFORMANCE-001-S02](../../../specs/features/linq/conformance.feature) and operator parity still require GoEF Queryable execution and evidence for the other providers.

## Sources and environment

| Component | Identity used |
| --- | --- |
| Isolated local lab | Git revision `822c0ce3fe1cb4cde1fabdb2acf8b127a578c38c`. The pilot extracted the tracked lab directory at that revision and used its [postgres_container provisioner](https://github.com/igorpooh1978/GoEF/blob/822c0ce3fe1cb4cde1fabdb2acf8b127a578c38c/test/conformance/lab/driver_run.py) and [resource owner](https://github.com/igorpooh1978/GoEF/blob/822c0ce3fe1cb4cde1fabdb2acf8b127a578c38c/test/conformance/lab/runtime.py). |
| PostgreSQL | Cached Linux/amd64 image `docker.io/library/postgres@sha256:32f5fdbf4d27ebe0d4552313126a593400f40b21bff207e43617b2305838bce3`. Both clients checked build number 180007, C database collation, UTC, fixture database, runtime identity and protected run marker. |
| EF client | SDK 10.0.402, self-contained Linux x64 runtime 10.0.13, EF Core and Relational 10.0.15, Npgsql EF provider 10.0.4 at `fe1109ecf52646ed6fc718a9a233353ef2e7baf9`, Npgsql 10.0.4 at `a445c040bf90b22e8921c4a079549c6ce154c9e3`. The EF source pin is `b34c0e9d7e768aa808ded44f19e7f871bea6869f`. [Source lock](../../upstream/lock.json) · [Upstream GroupJoin_DefaultIfEmpty tests](https://github.com/dotnet/efcore/blob/b34c0e9d7e768aa808ded44f19e7f871bea6869f/test/EFCore.Specification.Tests/Query/NorthwindJoinQueryTestBase.cs) |
| Native client | Go 1.27.4, Linux/amd64, pgx/v5 v5.13.0. The binary's module information returned `h1:Hl08k64maU0nrRyVUEbwCnK6/gT0EhMrtAqBHz9/gA4=`. Build inputs used the [driver lab module](https://github.com/igorpooh1978/GoEF/blob/822c0ce3fe1cb4cde1fabdb2acf8b127a578c38c/test/conformance/driverlab/go.mod) and its checksums. |

## Fixture and assertions

1. Seed two tables in one owned database. Left tuples `(id, tenant, key)` were `(11,1,A), (12,1,A), (13,1,NULL), (14,1,a), (21,2,A)`. Right tuples `(id, key)` were `(101,A), (102,A), (103,NULL)`. Both keys used text with C collation. The clients accessed the same committed seed. An independent read ordered each table by ID before and after both clients ran. The SHA-256 of the two compact JSON arrays was `032c4a8d777ca2959e37a341359656df23f9024d5ef3b9846a95c3ada29a38eb` both times.
2. Filter the left input by an integer tenant parameter. Exclude NULL keys from the right input and left-join on key equality. Order matched rows first, then by left ID and right ID, and project both IDs with a nullable right ID. In EF use no tracking, a group join and `DefaultIfEmpty()`. In pgx execute an independently written parameterized SQL query. The EF interceptor captured a server-side left join, right-key filter and ordering in the dispatched command.
3. Count the measured SELECT on each client with an EF `DbCommandInterceptor` and a pgx `QueryTracer`. Fixture setup, identity checks and seed verification run outside the measurement. The EF parameter was `NpgsqlDbType.Integer`. The pgx argument was an `int32` mapped to `int4`. Each run had one parameter with the requested value.

| Bound tenant | Ordered (left ID, right ID) from each client | EF commands | pgx commands |
| --- | --- | ---: | ---: |
| 1 | (11,101), (11,102), (12,101), (12,102), (13,NULL), (14,NULL) | 1 | 1 |
| 2 | (21,101), (21,102) | 1 | 1 |

The verifier compared both arrays to each other and to the declared expected rows. Removing (14,NULL) from a copy of the first EF result produced exit 1 with `ordered pair mismatch`. The complete comparison exited 0. The provisioner verified removal of the owned container and its volumes. The container had no published ports, external network or host data mount. Client credentials and marker values stayed out of the evidence.

## Reproduction

The local evidence bundle `postgres-ef-pilot.zip` contains the scratch clients, verifier, seed, locked package list, build script and redacted reports. It was handed off outside Git. Its SHA-256 is `031598582d62a0b661e4cb548b3181af986d300cc2acd3faf9c933b27cbfe9af`. The EF source, pgx source and package-lock SHA-256 values are respectively `b8f144c99288b89949a64d3510d5129929df73a62bb300683e0f2396b7249cb2`, `2a82a90c98a4d35d20e328951e0bb151d347d9b166efe3df88ed100c5fb16977` and `146762d06d9f9de8058f13f3ba83c6c80e624dd746608ac37e7c78eb091c7f3e`.

To rebuild and run the comparison with the recorded SDKs, cached packages, and pinned image, point the [Python pilot runner](../../../scripts/ci/ef_postgres_pilot.py) at the extracted bundle. It checks the inputs against the archived checksums and builds in a temporary directory.

```sh
python -B scripts/ci/ef_postgres_pilot.py --bundle "path/to/extracted-bundle"
```

The Python replay exited 0 on PostgreSQL 18.7 on 2026-11-26. It reproduced both ordered result sets, preserved the seed rows, confirmed the negative comparator exited 1, and removed its container. Missing and modified archive inputs were rejected before either build.

To check the archived results directly, run the verifier commands from the extracted bundle directory.

```sh
python -B verify.py ef-1.json pgx-1.json 1
python -B verify.py ef-2.json pgx-2.json 2
python -B verify.py negative-edited.json pgx-1.json 1
```

The build uses `dotnet restore --locked-mode` against a scratch-only package source, `dotnet publish --no-restore` and `go build -mod=readonly` with the proxy off. The runner copies both clients into the disposable container. A fresh extraction of the bundle reproduced the ordered results, negative control and cleanup with exit 0. The last command above is the deliberate failing control. The repository's existing driver-lab command exercises its native Gherkin cases separately.
