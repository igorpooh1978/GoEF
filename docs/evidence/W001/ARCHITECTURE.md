# W001 architecture contract experiment

The W001 design lane for [issue #12](https://github.com/igorpooh1978/GoEF/issues/12) starts at `c789602e46c7063e6c0f1ddbfec68bc22c2dbb75`. The Git remote uses `igorpooh1978/GoEF`. This experiment builds four temporary modules with local replacements, a fresh module cache and the proxy disabled. It copies no upstream code and requires only Python and Go 1.27.4. The existing [W000 fixture](../W000/GO_FEASIBILITY.md) separately covers five compiler rejection cases.

## Reproduce the isolated checks

Run this PowerShell command from the repository root. It extracts the Python block below and executes it using the local interpreter. The Go source files are synthetic type shapes. The empty query methods establish assignability only. The adapter also checks that an ordinary interface call preserves a plan field across module boundaries.

```powershell
python -c "from pathlib import Path; s=Path('docs/evidence/W001/ARCHITECTURE.md').read_text(encoding='utf-8'); exec(s.split(chr(96)*3+'python'+chr(10),1)[1].split(chr(10)+chr(96)*3,1)[0])"
```

```python
from pathlib import Path
import os
import subprocess
import tempfile

if not __debug__:
    raise SystemExit("Run with Python assertions enabled")

root = "github.com/igorpooh1978/GoEF"
adapter = "example.invalid/adapter"
consumer = "example.invalid/consumer"

with tempfile.TemporaryDirectory() as tmp:
    base = Path(tmp)
    env = dict(os.environ, GOTOOLCHAIN="local", GOWORK="off", GOPROXY="off",
               GOFLAGS="", GOMODCACHE=str(base / "modcache"))
    def write(path, content):
        target = base / path
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(content.replace("@CORE@", root), encoding="utf-8")

    def module(directory, path, dependencies=()):
        lines = [f"module {path}", "go 1.27.4"]
        for name, location in dependencies:
            lines.extend([f"require {name} v0.0.0", f"replace {name} => ../{location}"])
        write(f"{directory}/go.mod", "\n".join(lines) + "\n")

    def go(directory, *args, reject=None):
        cmd = ["go", *args]
        result = subprocess.run(cmd, cwd=base / directory, env=env, text=True,
                                capture_output=True)
        if reject is None:
            assert result.returncode == 0, (cmd, result.stderr)
        else:
            assert result.returncode != 0 and reject in result.stderr, (cmd, result.stderr)
        return result.stdout.splitlines()

    module("core", root)
    module("adapter", adapter, [(root, "core")])
    module("objects", consumer + "/objects", [(root, "core")])
    module("app", consumer, [(root, "core"), (adapter, "adapter")])

    write("core/linq/objects/source.go", """package objects
func FromSlice[T any](v []T) []T { return v }
""")
    write("core/provider/compiler.go", """package provider
type RelationalPlan struct { SlotCount int }
type Command struct { SlotCount int }
type Compiler interface { Compile(RelationalPlan) (Command, error) }
""")
    write("core/internal/queryir/plan.go", "package queryir\n")
    write("core/linq/expressions/types.go", """package expressions
type Predicate[T any] struct{}
type Projection[T, R any] struct{}
""")
    write("core/linq/queryable/query.go", """package queryable
import (
    "context"
    "@CORE@/linq/expressions"
)
type Set[T any] struct{}
type EntityQuery[T any] struct{}
type Query[R any] struct{}
func (Set[T]) Where(expressions.Predicate[T]) EntityQuery[T] { return EntityQuery[T]{} }
func (EntityQuery[T]) ToList(context.Context) ([]*T, error) { return nil, nil }
func (EntityQuery[T]) Select[R any](expressions.Projection[T,R]) Query[R] { return Query[R]{} }
func (Query[R]) ToList(context.Context) ([]R, error) { return nil, nil }
""")
    write("core/context.go", """package goef
import "@CORE@/linq/queryable"
type Context struct{}
func (*Context) Set[T any]() queryable.Set[T] { return queryable.Set[T]{} }
""")
    write("adapter/adapter.go", """package adapter
import "@CORE@/provider"
type Compiler struct{}
func (Compiler) Compile(p provider.RelationalPlan) (provider.Command, error) {
    return provider.Command{SlotCount: p.SlotCount}, nil
}
var _ provider.Compiler = Compiler{}
""")
    write("objects/consumer.go", """package consumer
import "@CORE@/linq/objects"
var _ []int = objects.FromSlice([]int{1})
""")
    write("app/shape_test.go", """package consumer_test
import (
    "context"
    "testing"
    goef "@CORE@"
    "@CORE@/linq/expressions"
    "@CORE@/provider"
    "example.invalid/adapter"
)
type order struct{}
type orderID int64
func TestExternal(t *testing.T) {
    q := new(goef.Context).Set[order]().Where(expressions.Predicate[order]{})
    rows, _ := q.ToList(context.Background())
    ids, _ := q.Select(expressions.Projection[order, orderID]{}).ToList(context.Background())
    var _ []*order = rows
    var _ []orderID = ids
    var c provider.Compiler = adapter.Compiler{}
    got, err := c.Compile(provider.RelationalPlan{SlotCount: 3})
    if err != nil || got.SlotCount != 3 { t.Fatal(got, err) }
}
""")
    version, = go("app", "version")
    assert version.split()[2] == "go1.27.4", version
    print(version)
    go("app", "test", "-count=1", "./...")
    go("objects", "build", "./...")
    deps = go("objects", "list", "-deps", "-f", "{{if not .Standard}}{{.ImportPath}}{{end}}", ".")
    assert set(deps) == {root + "/linq/objects", consumer + "/objects"}, deps
    mods = go("objects", "list", "-m", "-f", "{{.Path}}", "all")
    assert set(mods) == {root, consumer + "/objects"}, mods
    write("adapter/invalid.go", 'package adapter\nimport _ "@CORE@/internal/queryir"\n')
    go("adapter", "build", "./...", reject="use of internal package")
    write("objects/consumer.go", f'package consumer\nimport _ "{root.lower()}/linq/objects"\n')
    go("objects", "build", "./...", reject="no required module provides package " + root.lower() + "/linq/objects")
    print("PASS: external adapter, pointer/value signatures, Objects closure, internal and case controls")
```

## Evidence and remaining stages

On November 25, 2026, the command above exited 0 on Windows/amd64 with Python 3.14.4 and Go 1.27.4. Both negative builds returned a nonzero exit and matched the diagnostic for their intended cause. Repository validation via `python -B test/bootstrap/verify.py` exited 0. Its seven regression tests via `python -B -m unittest discover -s test/bootstrap -p test_verify.py` also exited 0. These checks validate the design experiment and repository references. They do not exercise a production provider.

| Stage | Local result or evidence required |
| --- | --- |
| Repository path, separate consumer and adapter | In the temporary graph, the correctly cased local replacement resolves. An external package implements the ordinary compiler interface and produces the expected slot count. A separate negative build rejects an adapter import of core's private query IR. |
| Objects-only import | The fixture consumer's only nonstandard package dependency is its Objects source package, besides the consumer itself. Its module graph has two entries, the consumer and core. A lower-case module import in the same isolated setup fails. Real package imports must repeat these assertions once implemented. |
| Entity and projection terminals | The external caller assigns the inferred entity list to `[]*order` and the projected scalar list to `[]orderID`. No database is opened. Actual row materialization, identity resolution, DTO projections and streaming need executable tests. |
| Before approving public modules in W001 | Build the actual core, a one-provider consumer and an out-of-tree adapter in isolation. Inspect both direct/transitive package imports and selected module versions. Verify path resolution without local replacement on a suitable case-sensitive runner, version negotiation, mutable-binding isolation, invalid owners/aliases, stale generated entry points, cancellation, overlap/close and a race run. Record scope-specific evidence from [the LINQ fixture issue](https://github.com/igorpooh1978/GoEF/issues/17). |
| Before accepting runtime behavior | The [W002 conformance issue](https://github.com/igorpooh1978/GoEF/issues/13) owns the engine environment. Run separate PostgreSQL, SQLite, SQL Server and MySQL suites for pointer identity, cancellation/early stream release, external transaction ownership, loss of commit acknowledgement, destructive approval and interrupted migration recovery at their implementing waves. Keep Objects separate. |
| Before release | Require the integrated revision, approved project license and module identity, pinned drivers/engines, versioned adapter compatibility and independent acceptance. The W000 PostgreSQL/MySQL document gaps remain tracked separately at the [checkpoint](../../roadmap/CHECKPOINT.md). |

[Proposed import graph](../../architecture/MODULE_GRAPH.md) · [Proposed application API](../../architecture/PUBLIC_API.md)
