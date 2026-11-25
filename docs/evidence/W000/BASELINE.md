# W000 baseline

- Mode: BOOTSTRAP_ONLY. Tracking: [issue #1](https://github.com/igorpooh1978/GoEF/issues/1).
- Repository: [igorpooh1978/GoEF](https://github.com/igorpooh1978/GoEF). Branch: main. Initial commit: 2a886c78e622f583fa2980ea0a61a7e328a40e28.
- git status --short --branch returned a clean main tracking origin/main. git ls-remote origin refs/heads/main returned the same commit. The only tracked file was README.md. No Go module, Go workspace, CI, repository agents, generators, tests, or CodeGraph index were present.
- go version returned go1.27.4 windows/amd64. go env GOOS GOARCH CGO_ENABLED GOTOOLCHAIN returned windows, amd64, 0, auto. See [Go's release notes](https://go.dev/doc/go1.27) and the [feasibility evidence](GO_FEASIBILITY.md).
- Python 3.14.4, Docker CLI 29.4.0, GitHub CLI and Codex CLI 0.162.1 were available. Docker client presence supplies no server or engine proof. No database was accessed. W000 test design uses synthetic data exclusively.
- The issues list started empty. Issue support and repository permissions were confirmed. The [checkpoint](../../roadmap/CHECKPOINT.md) lists the tracked work.
- The stand-alone master plan and the appendix text matched on readback after trimming outer whitespace. [The repository copy](../../roadmap/MASTER_PLAN.md) is the English plan.
- The repository baseline had no project license. See [issue #15](https://github.com/igorpooh1978/GoEF/issues/15).
