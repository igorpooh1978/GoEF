# LINQ generator build version checks

[Issue #38](https://github.com/igorpooh1978/GoEF/issues/38) · [Build and preview versions](../../development/BUILD_VERSIONS.md) · [Scenarios](../../../specs/features/documentation/build_version.feature)

## Baseline

At `388e85cb27d3bebe9a4674b922e235658f647570`, building the generator on Windows with Go 1.27.4 exited 0. `go version -m` showed the pseudo-version and revision. The binary's `-version` exited 1 with a flag diagnostic.

## Results

Windows amd64 used Go 1.27.4, Python 3.14.4 and Git 2.53.0.windows.2. Ubuntu Linux amd64 under WSL used Go 1.27.4 with CGO enabled.

| Check | Exit and result |
| --- | --- |
| `go build ./...` and `go vet ./...` on Windows | Both 0. |
| `go test -shuffle=on -count=1 -coverpkg=<owned> -coverprofile=<profile> ./...` on Windows | 0. Root production coverage 1,620/1,664 statements (97.4%), generator 208/216 (96.3%) and version formatter 19/19. The owned package set is `go list ./...` excluding examples. |
| `go test -race -shuffle=on -count=1 -coverpkg=<owned> -coverprofile=<profile> ./...` on Linux | 0. Root production coverage 97.4%. |
| `go generate ./examples/linq/generated` and `git diff --exit-code -- examples/linq/generated` on Windows | Both 0. The generated example stayed byte-identical. |
| `go test -run '^$' -fuzz '^FuzzGenerate$' -fuzztime 5s -parallel 2 ./cmd/goef-linqgen` on Windows | 0. 68,295 executions. |
| `govulncheck ./...` on Windows | 0. No vulnerabilities reported. |
| `python -B -m unittest discover -s test/bootstrap -p test_verify.py` on Windows | 0. Seven tests passed. |
| `python -B test/architecture/public-module/verify.py 9fcd036a3927b40e26381202c7f1b5aa4fb3d858 --version v0.1.0-alpha.1 --generated-bindings` on Windows | 0. The verifier still resolves and checks the existing public preview. |

## Versioned build fixtures

A disposable Git clone of the baseline contained the changed generator sources at fixture revision `1be0bbd7853fd59402d9a6c902e27258baf26a8b`. A local `v0.2.0-alpha.1` tag existed only in that clone. All commands below exited 0.

- Two standalone binaries built with `go build -buildvcs=true -o <temporary-binary> ./cmd/goef-linqgen` reported the expected pseudo-version before tagging and exact module version after tagging. Both reported the full fixture revision and `dirty=false`.
- Adding an untracked file and building again reported `v0.2.0-alpha.1+dirty`, the same revision and `dirty=true`. Building the same checkout with `-buildvcs=false` reported `goef-linqgen (devel)`.
- A fresh external module used direct Git with a subprocess-local URL rewrite to the disposable clone. It used an isolated module cache and executable directory. `go mod download -json github.com/igorpooh1978/GoEF@v0.2.0-alpha.1` resolved the tag to the expected fixture revision with both `h1:` checksums. The public verifier's generator-version check ran `go install github.com/igorpooh1978/GoEF/cmd/goef-linqgen@v0.2.0-alpha.1` against that clone. The installed binary printed `goef-linqgen v0.2.0-alpha.1` as its complete version line.

The [generator tests](../../../cmd/goef-linqgen/version_test.go) cover command output, argument rejection, tagged metadata, dirty metadata, missing metadata and replacements. The fixture proves local Git tag resolution. Public candidate resolution, the provider tags and independent acceptance require the integrated release candidate.
