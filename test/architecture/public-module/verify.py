"""Check a published GoEF revision from a fresh external module."""

import argparse
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
from tempfile import TemporaryDirectory


MODULE = "github.com/igorpooh1978/GoEF"
GO_VERSION = "go1.27.4"
EXPECTED_OUTPUT = "8 true\n002_customer_notes\n"


def verify(commit, version, generated_bindings=False, *, generator_version=False):
    with TemporaryDirectory(prefix="goef-public-consumer-") as temporary:
        root = Path(temporary)
        consumer = root / "consumer"
        consumer.mkdir()
        env = dict(
            os.environ,
            GOBIN=str(root / "bin"),
            GOENV="off",
            GOFLAGS="",
            GOINSECURE="",
            GOMODCACHE=str(root / "modcache"),
            GONOPROXY="none",
            GONOSUMDB="none",
            GOPRIVATE="",
            GOPROXY="https://proxy.golang.org",
            GOSUMDB="sum.golang.org",
            GOTOOLCHAIN="local",
            GOWORK="off",
        )
        for name in ("GO111MODULE", "GOOS", "GOARCH"):
            env.pop(name, None)

        def go(*args):
            result = subprocess.run(
                ["go", *args], cwd=consumer, env=env, check=True,
                capture_output=True, encoding="utf-8", timeout=120,
            )
            return result.stdout

        toolchain = go("version").strip()
        if not toolchain.startswith(f"go version {GO_VERSION} "):
            raise ValueError(f"expected {GO_VERSION}, got {toolchain}")

        go("mod", "init", "example.com/goef-public-consumer")
        download = json.loads(go("mod", "download", "-json", f"{MODULE}@{version or commit}"))
        origin = download.get("Origin") or {}
        if (
            download.get("Path") != MODULE
            or origin.get("VCS") != "git"
            or origin.get("URL") != f"https://{MODULE}"
            or origin.get("Hash") != commit
        ):
            raise ValueError(f"module download provenance does not match commit {commit}")

        resolved = download["Version"]
        if version is not None and resolved != version:
            raise ValueError(f"expected module version {version}, got {resolved}")
        if not all(download.get(field, "").startswith("h1:") for field in ("Sum", "GoModSum")):
            raise ValueError("module download checksums are missing")

        shutil.copyfile(Path(__file__).with_name("consumer.go.txt"), consumer / "main.go")
        go("mod", "edit", f"-require={MODULE}@{resolved}")
        go("mod", "tidy")
        if generated_bindings:
            verify_generated_bindings(go, consumer)
        if generator_version:
            verify_generator_version(go, consumer, env, resolved)
        go("mod", "verify")
        selected = json.loads(go("list", "-mod=readonly", "-m", "-json", MODULE))
        if selected.get("Path") != MODULE or selected.get("Version") != resolved or "Replace" in selected:
            raise ValueError("consumer selected a different GoEF dependency")
        go("build", "-mod=readonly", "./...")
        go("vet", "-mod=readonly", "./...")
        if go("run", "-mod=readonly", ".") != EXPECTED_OUTPUT:
            raise ValueError("public consumer returned unexpected results")

    return {
        "module": MODULE,
        "version": resolved,
        "commit": commit,
        "sum": download["Sum"],
        "goModSum": download["GoModSum"],
        "go": toolchain,
        "consumer": "PASS",
        **({"generatedBindings": "PASS"} if generated_bindings else {}),
        **({"generatorVersion": resolved} if generator_version else {}),
    }


def verify_generator_version(go, consumer, env, version):
    go("install", f"{MODULE}/cmd/goef-linqgen@{version}")
    executable = Path(env["GOBIN"]) / ("goef-linqgen.exe" if os.name == "nt" else "goef-linqgen")
    result = subprocess.run(
        [executable, "-version"], cwd=consumer, env=env, check=True,
        capture_output=True, encoding="utf-8", timeout=15,
    )
    if result.stdout != f"goef-linqgen {version}\n" or result.stderr:
        raise ValueError("installed generator did not report the expected module version")


def verify_generated_bindings(go, consumer):
    fixture_dir = Path(__file__).parent
    source = consumer / "query.go"
    output = consumer / "query_gen.go"
    shutil.copyfile(fixture_dir / "query.go.txt", source)
    command = ("run", "-mod=readonly", f"{MODULE}/cmd/goef-linqgen",
               "-source", source.name, "-output", output.name)
    go(*command)
    expected = output.read_bytes()
    go(*command)
    if output.read_bytes() != expected:
        raise ValueError("public generator output changed on a second run")
    go(*command, "-check")

    original_source = source.read_bytes()
    source.write_bytes(original_source.replace(b"row.City == city", b"row.Name == city", 1))
    verify_stale_check(go, command, output, expected)
    source.write_bytes(original_source)
    edited_output = expected + b"\n// stale binding\n"
    output.write_bytes(edited_output)
    verify_stale_check(go, command, output, edited_output)
    go(*command)
    if output.read_bytes() != expected:
        raise ValueError("public generator failed to restore the original output")
    go(*command, "-check")

    shutil.copyfile(fixture_dir / "generated_test.go.txt", consumer / "generated_test.go")
    go("mod", "tidy")
    go("test", "-mod=readonly", "-count=1", "./...")


def verify_stale_check(go, command, output, unchanged):
    try:
        go(*command, "-check")
    except subprocess.CalledProcessError as error:
        if error.returncode != 1 or "stale" not in (error.stderr or "").lower():
            raise
    else:
        raise ValueError("public generator accepted a stale binding")
    if output.read_bytes() != unchanged:
        raise ValueError("public generator check rewrote stale output")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("commit", help="expected full Git commit SHA")
    parser.add_argument("--version", help="exact public module version to resolve")
    parser.add_argument("--generated-bindings", action="store_true",
                        help="generate and exercise typed LINQ bindings from the downloaded revision")
    parser.add_argument("--generator-version", action="store_true",
                        help="install the versioned generator and check its build version")
    args = parser.parse_args()
    if not re.fullmatch(r"[0-9a-f]{40}", args.commit):
        parser.error("commit must be a full lowercase Git SHA")
    if args.version is not None and not re.fullmatch(r"v\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.+-]+)?", args.version):
        parser.error("version must be an exact Go module version")

    try:
        report = verify(
            args.commit, args.version, args.generated_bindings,
            generator_version=args.generator_version,
        )
        print(json.dumps(report, indent=2))
    except (OSError, ValueError, subprocess.SubprocessError) as error:
        print(error, file=sys.stderr)
        if isinstance(error, subprocess.CalledProcessError) and error.stderr:
            print(error.stderr.rstrip(), file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
