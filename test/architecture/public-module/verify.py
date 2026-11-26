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


def verify(commit, version):
    with TemporaryDirectory(prefix="goef-public-consumer-") as temporary:
        root = Path(temporary)
        consumer = root / "consumer"
        consumer.mkdir()
        env = dict(
            os.environ,
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
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("commit", help="expected full Git commit SHA")
    parser.add_argument("--version", help="exact public module version to resolve")
    args = parser.parse_args()
    if not re.fullmatch(r"[0-9a-f]{40}", args.commit):
        parser.error("commit must be a full lowercase Git SHA")
    if args.version is not None and not re.fullmatch(r"v\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.+-]+)?", args.version):
        parser.error("version must be an exact Go module version")

    try:
        print(json.dumps(verify(args.commit, args.version), indent=2))
    except (OSError, ValueError, subprocess.SubprocessError) as error:
        print(error, file=sys.stderr)
        if isinstance(error, subprocess.CalledProcessError) and error.stderr:
            print(error.stderr.rstrip(), file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
