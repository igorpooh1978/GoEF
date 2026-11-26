# Public Go module resolution at a published commit

[Issue #24](https://github.com/igorpooh1978/GoEF/issues/24) · [W001 SPI review](SPI_FREEZE.md)

On 2026-11-26, `git ls-remote origin refs/heads/main 'refs/tags/*'` reported [`7a08d90b215d11904109b109c1381ba19c881f35`](https://github.com/igorpooh1978/GoEF/commit/7a08d90b215d11904109b109c1381ba19c881f35) on public `main` with zero tags. The GitHub releases endpoint reported an empty list.

A fresh temporary consumer module outside the GoEF checkout used Go 1.27.4 on Windows/amd64, a new module cache and workspace mode off. The public Go proxy and checksum database were enabled. The selected dependency had no replacement.

| Module download field | Observed value |
| --- | --- |
| Path | `github.com/igorpooh1978/GoEF` |
| Version | `v0.0.0-20261126150900-7a08d90b215d` |
| Origin | Git at `https://github.com/igorpooh1978/GoEF`, hash `7a08d90b215d11904109b109c1381ba19c881f35` |
| Module archive checksum | `h1:RhREJVJfD6Z1vsluN/dDGkiR1zjGXgFRCAPQPBMGqSg=` |
| Module file checksum | `h1:ABeTL0NnLBe42NYSr8liTicW7AUWHsszrL7EzWFpFF0=` |

## Consumer check

The original module initialization and SHA resolution commands each exited 0. Run this Python snippet from an empty consumer directory to repeat them.

```python
import os
import subprocess
from tempfile import TemporaryDirectory

module = "github.com/igorpooh1978/GoEF@7a08d90b215d11904109b109c1381ba19c881f35"
env = dict(
    os.environ,
    GOWORK="off",
    GOTOOLCHAIN="local",
    GOPROXY="https://proxy.golang.org",
    GOSUMDB="sum.golang.org",
    GOPRIVATE="",
    GONOPROXY="none",
    GONOSUMDB="none",
)
with TemporaryDirectory(prefix="goef-consumer-modcache-") as cache:
    env["GOMODCACHE"] = cache
    for arguments in (
        ["mod", "init", "example.com/goef-issue24-consumer"],
        ["mod", "download", "-json", module],
        ["get", module],
    ):
        subprocess.run(["go", *arguments], env=env, check=True)
```

The consumer imported `github.com/igorpooh1978/GoEF/linq/objects` and called `objects.First(objects.Where(slices.Values(values), isEven))` from an ordinary Go source file. `TestFirstEven` checked that `[3, 8, 10]` returns `(8, true)` and `[3, 7]` returns `(0, false)`.

In the original run, with those files present, `go mod tidy`, `go mod verify`, `go build -mod=readonly ./...`, `go vet -mod=readonly ./...` and `go test -mod=readonly -count=1 -v ./...` all exited 0. `go list -m -json github.com/igorpooh1978/GoEF` confirmed the expected version, Go 1.27.4 and an unset replacement.

## Repeatable public consumer check

Run [the public module verifier](../../../test/architecture/public-module/verify.py) from the repository root with Python 3.11 or newer and Go 1.27.4. It uses a fresh consumer directory and module cache, the public Go proxy and checksum database, and normal module mode with workspaces disabled. It checks the downloaded module path, origin SHA, both checksums, and the selected dependency with its replacement field unset. It runs tidy, module verification, a read-only build and vet, then executes a [small consumer program](../../../test/architecture/public-module/consumer.go.txt) that filters a local sequence and lists a pending migration.

```sh
python -B test/architecture/public-module/verify.py fdd22eef3d8ddc07aadef1a152cb20392b090c3f
python -B test/architecture/public-module/verify.py fdd22eef3d8ddc07aadef1a152cb20392b090c3f --version v0.0.0-20261126152102-fdd22eef3d8d
```

Both commands exited 0 on Windows/amd64 with Python 3.14.4 and Go 1.27.4. The public download resolved to `v0.0.0-20261126152102-fdd22eef3d8d` at `fdd22eef3d8ddc07aadef1a152cb20392b090c3f`, matching `git ls-remote origin refs/heads/main`. The archive checksum was `h1:Ig7RtMcwpg2VIWiwzn1xps4XYUANHeu67fP4O+lOmzg=` and the module file checksum was `h1:ABeTL0NnLBe42NYSr8liTicW7AUWHsszrL7EzWFpFF0=`. The consumer produced `8 true` and `002_customer_notes`. [Published source](https://github.com/igorpooh1978/GoEF/commit/fdd22eef3d8ddc07aadef1a152cb20392b090c3f)

The control used the same version with the earlier expected SHA `7a08d90b215d11904109b109c1381ba19c881f35`. It exited 1 on the provenance mismatch before compiling the consumer. A repeat with the caller's proxy and checksum database disabled, an invalid workspace override, and vendor mode selected also exited 0. The verifier supplied its own module environment. An approved release can use the same verifier with its full commit SHA and exact public Go module version.

## Acceptance at the original snapshot

- The MIT license is present at the verified commit and [issue #15](https://github.com/igorpooh1978/GoEF/issues/15) is closed.
- Binding reproducibility and stale-output rejection require the planned generator and the generated-binding fixtures. The checked commit contains compiler fixtures. The generation workflow and changed-capture and stale-output checks are specified in the design. [Generator wave](../../roadmap/MASTER_PLAN.md#9-roadmap-dependency-driven-waves) · [LINQ generation contract](../../architecture/LINQ_EXPRESSIONS.md)
- Release module resolution requires an approved public release revision and a consumer check pinned to that revision. The observed result establishes public module resolution and execution for the checked commit. [Release gates](../../development/RELEASE_GATES.md)

## Tagged public release

The maintainer approved commit `9fcd036a3927b40e26381202c7f1b5aa4fb3d858` for the first public preview. GitHub published [GoEF v0.1.0-alpha.1](https://github.com/igorpooh1978/GoEF/releases/tag/v0.1.0-alpha.1) on 2026-11-26. The local annotated tag, the remote Git tag, and the GitHub tag API agreed on tag object `7fcd49f96a3aadb8b609b8c04d2acf16f27f5454` and the approved commit.

```sh
python -B test/architecture/public-module/verify.py 9fcd036a3927b40e26381202c7f1b5aa4fb3d858 --version v0.1.0-alpha.1 --generated-bindings
```

The command exited 0 on Windows/amd64 with Go 1.27.4. An independent Linux/amd64 run using Python 3 and the same Go version also exited 0. Each run created a fresh external Go module and module cache. The public Go proxy and checksum database resolved the exact tagged version without a replacement. The downloaded origin SHA matched the approved commit.

| Check | Verified value |
| --- | --- |
| Module archive checksum | `h1:q9DrLJGAVL6o49FNoHAxrjdsL9cbsJxMIWEJKWaMPUA=` |
| Module file checksum | `h1:ABeTL0NnLBe42NYSr8liTicW7AUWHsszrL7EzWFpFF0=` |
| External consumer | `PASS` |
| Generated bindings | `PASS` |

The consumer passed module verification, read-only build, vet, tests, and execution. Two generations produced identical files. Freshness checks rejected both an edited source declaration and an edited generated file without rewriting the generated output. This evidence covers the root Go module's public preview. [Issue #24](https://github.com/igorpooh1978/GoEF/issues/24)
