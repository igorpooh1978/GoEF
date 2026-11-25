# W002 lab design evidence

[Design #16](https://github.com/igorpooh1978/GoEF/issues/16), part of [runtime infrastructure #13](https://github.com/igorpooh1978/GoEF/issues/13). Read-only feasibility probe on 2026-11-25 UTC against baseline `c789602e46c7063e6c0f1ddbfec68bc22c2dbb75`. The [lab design](../../development/CONFORMANCE_LAB.md) specifies the proposed runner, seed, controls, fault boundaries and acceptance.

## Available artifacts

The candidate versions come from the [provider matrix](../../parity/PROVIDER_MATRIX.md). Registry observations below use Linux/amd64. An index addresses a set of platforms. The platform manifest selects the layers for this runner. The Microsoft registry served a single OCI manifest. The three selected platform manifests resolved by digest. No image layer was downloaded or run.

| Official server image and observed tag | Published index SHA-256 | Selected Linux/amd64 manifest SHA-256 |
| --- | --- | --- |
| [docker.io/library/postgres:18.7](https://hub.docker.com/v2/repositories/library/postgres/tags/18.7) | `c699303802144847d238196173655dbbecf7a96aeca1565f2b02eb671a1f0b6b` | `32f5fdbf4d27ebe0d4552313126a593400f40b21bff207e43617b2305838bce3` |
| [docker.io/library/mysql:8.4.13.1](https://hub.docker.com/v2/repositories/library/mysql/tags/8.4.13.1) | `8c233bad33d520a7aae1c1e619c3a9775f4a1ba2d463d692fada04ec7f40be00` | `f0c7a4430d773ef8433aae82e08cb02ade4a6ad806a18b1610a48a1728fbdfd5` |
| [mcr.microsoft.com/mssql/server:2022-CU28-ubuntu-22.04](https://mcr.microsoft.com/v2/mssql/server/tags/list) | Single OCI manifest | `4525e3efac9f3e241cb003bb2ff2a82818b502f37e12049d788b1a9fa7e13b7f` |

For example, the proposed PostgreSQL Linux/amd64 input is `docker.io/library/postgres@sha256:32f5fdbf4d27ebe0d4552313126a593400f40b21bff207e43617b2305838bce3`. The runner must match the actual server version, platform and driver to the selected tuple after startup.

SQLite uses the engine embedded in the linked driver. The driver lists SQLite 3.54.1 for Linux/amd64. The expected upstream SQLite source ID is `2026-10-16 14:45:20 555e07ef33f69a0957d24d8e555b702464a131d6f1607f4c3c1e4551e8e6ca76`. Acceptance compares both the version and source ID from the test process and captures compile options. A standalone system SQLite cannot attest to the linked module. [Tagged driver platform table](https://pkg.go.dev/modernc.org/sqlite@v1.63.0#hdr-Supported_platforms_and_architectures) · [SQLite release identity](https://www.sqlite.org/releaselog/3_54_1.html)

| Candidate driver | Module content hash returned by the public Go checksum database |
| --- | --- |
| [github.com/jackc/pgx/v5 v5.13.0](https://sum.golang.org/lookup/github.com/jackc/pgx/v5@v5.13.0) | `h1:Hl08k64maU0nrRyVUEbwCnK6/gT0EhMrtAqBHz9/gA4=` |
| [modernc.org/sqlite v1.63.0](https://sum.golang.org/lookup/modernc.org/sqlite@v1.63.0) | `h1:ClxazqRFNfr7XPHTVLbyzTVz2PjWPz+BfJNu2e8JDMs=` |
| [github.com/microsoft/go-mssqldb v1.11.0](https://sum.golang.org/lookup/github.com/microsoft/go-mssqldb@v1.11.0) | `h1:YbDqolEjGH9hBfvKzONTf5/dbl9RKXmizMJE93lVxNs=` |
| [github.com/go-sql-driver/mysql v1.10.0](https://sum.golang.org/lookup/github.com/go-sql-driver/mysql@v1.10.0) | `h1:Q+1LV8DkHJvSYAdR83XzuhDaTykuDx0l6fkXxoWCWfw=` |

The probe read public checksum records over HTTPS. Dependency approval, Go toolchain verification of the authenticated module graph including transitive dependencies, and the runtime driver build remain pending.

## Reproducing the observations

| Read-only operation | Observed result |
| --- | --- |
| `git rev-parse HEAD`, `git remote get-url origin` and `gh issue view 16 --repo igorpooh1978/GoEF --json title,body,url,state` (also issue 13) | Exit 0. Baseline above, expected GitHub origin and open issues. |
| `go version`, `docker --version`, `docker compose version`, `docker context show`, `docker version --format 'server={{.Server.Version}} os={{.Server.Os}} arch={{.Server.Arch}}'` | Exit 0. Go 1.27.4 windows/amd64, Docker CLI and server 29.4.0, Compose v5.1.1, desktop Linux context, server linux/amd64. Host reports Windows 10.0.26300 X64. No database endpoint or existing container was accessed. |
| Docker Hub tag GET for `library/postgres:18.7` and `library/mysql:8.4.13.1`. Anonymous Registry V2 GET `/v2/library/{repository}/manifests/{reference}` at `registry-1.docker.io`, using only repository pull scope. Request OCI and Docker index/manifest media types. | Exit 0. For each tag, selected its Linux/amd64 descriptor. GET by the index digest and platform digest returned bytes with SHA-256 equal to the returned digest header and the expected reference. Docker Hub metadata agreed. |
| Microsoft registry GET `/v2/mssql/server/manifests/{reference}` at `mcr.microsoft.com` with `Accept: application/vnd.oci.image.manifest.v1+json` included in the allowed media types. Repeat using the reported OCI digest and read the referenced image config. | Exit 0. Tag and digest yielded the same hashed bytes. Config `sha256:548ecf4a83632020d95dfc744bac431f91e31458c2a785b61275941abae7c2bf` reported linux/amd64. A manifest requested only in Docker schema v2 had a different digest, which failed independent lookup. The table uses the verified OCI digest. |
| HTTPS GET `https://sum.golang.org/lookup/{module}@{version}` for all four rows | Exit 0. The returned content hashes are recorded above. No module installation or compilation was performed. |

## What remains to prove

Design validation on the lane worktree used `python -B test/bootstrap/verify.py` and `python -B -m unittest discover -s test/bootstrap -p test_verify.py`. Both exited 0. The validator found 64 specified Gherkin scenarios, and all seven bootstrap validator tests passed. Cross-checking the expanded scenario references in the design against tags in `specs/features` resolved 36 distinct existing scenario IDs. These checks validate the design's references and bootstrap gates.

| Cell | Design evidence | Runtime gate |
| --- | --- | --- |
| PostgreSQL | Exact image metadata and driver record available. | NOT RUN. Approve pgx mode and isolate a real database. Confirm 18.7 from the server. Exact documentation MCP provenance remains tracked under [#10](https://github.com/igorpooh1978/GoEF/issues/10). |
| SQLite | Tagged driver platform table, checksum and vendor source ID available. | NOT RUN. Build the selected driver, inspect the embedded engine and prove restart, contention and cleanup in both journal modes. |
| SQL Server | Exact CU tag and platform manifest available. | NOT RUN. Approve Developer edition terms and eligible Linux x64 runner, then verify product build, settings and actual resources. |
| Oracle MySQL | Exact update tag and driver record available. | NOT RUN. Verify the running update, InnoDB and session settings. Exact documentation MCP provenance remains tracked under [#11](https://github.com/igorpooh1978/GoEF/issues/11). |

Runtime work depends on approved [W001 contracts](https://github.com/igorpooh1978/GoEF/issues/12), runner and dependency decisions, and a selectable W002 runtime harness. No provider runtime scenarios, Gherkin RED/GREEN, GitHub Actions jobs or coverage measurements ran in this design slice. All four real-engine rows and the separate Objects suite remain pending.
