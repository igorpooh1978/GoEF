"""Build and run the archived EF Core and pgx PostgreSQL comparison offline."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
from tempfile import TemporaryDirectory


ROOT = Path(__file__).resolve().parents[2]
BASELINE = "822c0ce3fe1cb4cde1fabdb2acf8b127a578c38c"
SDK = "10.0.402"
RUNTIME = "10.0.13"
GO_VERSION = "go1.27.4"
DRIVERLAB = "test/conformance/driverlab"
BUNDLE_INPUTS = {
    "Pilot.csproj": "1a0e8398ec3aeb3e549850fccfe4ae295dc2bf4f04e005e3d025ded7faa006c0",
    "Program.cs": "b8f144c99288b89949a64d3510d5129929df73a62bb300683e0f2396b7249cb2",
    "main.go": "2a82a90c98a4d35d20e328951e0bb151d347d9b166efe3df88ed100c5fb16977",
    "NuGet.Config": "58d54f9db6944980984a44f4a2b923387559b9d523883187897024cc3b47501a",
    "packages.lock.json": "146762d06d9f9de8058f13f3ba83c6c80e624dd746608ac37e7c78eb091c7f3e",
    "run.py": "becdbb223fb80c1be6e18df31913a451ad0ae59a67dba61f4adf2aa59191ed03",
    "verify.py": "b22b802a4cb400c45224e6c6e271bf5daeadab089b4fb8c7a30179f6fd99b70f",
    "seed.sql": "f8b7f80ec93ad9d3acf70aff3c674abe73d1a67366d3a31ef2d6131a66de2d5e",
}
RUNTIME_PACKS = (
    "microsoft.netcore.app.runtime.linux-x64",
    "microsoft.aspnetcore.app.runtime.linux-x64",
    "microsoft.netcore.app.host.linux-x64",
)


class PilotError(Exception):
    """The pilot could not be reproduced with the available artifacts."""


def command(label, argv, scratch, env=None):
    try:
        result = subprocess.run(
            argv, cwd=scratch, env=env, capture_output=True, check=False,
        )
    except OSError:
        raise PilotError(f"{label} could not be started.") from None
    if result.returncode:
        raise PilotError(f"{label} exited with code {result.returncode}.")
    return result.stdout


def stage_bundle(bundle, scratch):
    for name, expected_hash in BUNDLE_INPUTS.items():
        try:
            data = (bundle / name).read_bytes()
        except OSError:
            raise PilotError(f"Missing or unreadable bundle input: {name}.") from None
        if hashlib.sha256(data).hexdigest() != expected_hash:
            raise PilotError(f"Bundle checksum mismatch: {name}.")
        (scratch / name).write_bytes(data)


def stage_nuget_feed(scratch):
    cache = Path(os.environ.get("NUGET_PACKAGES") or Path.home() / ".nuget" / "packages")
    feed = scratch / "feed"
    feed.mkdir()
    locked = json.loads((scratch / "packages.lock.json").read_text(encoding="utf-8"))
    packages = {(name, RUNTIME) for name in RUNTIME_PACKS}
    packages.update(
        (name.lower(), details["resolved"])
        for framework in locked["dependencies"].values()
        for name, details in framework.items()
        if details.get("resolved")
    )
    for package, version in sorted(packages):
        name = f"{package}.{version}.nupkg"
        try:
            shutil.copyfile(cache / package / version / name, feed / name)
        except OSError:
            raise PilotError(f"Offline package missing or unreadable: {name}.") from None


def build_dotnet(scratch):
    env = dict(
        os.environ, DOTNET_CLI_HOME=str(scratch / "dotnethome"),
        DOTNET_SKIP_FIRST_TIME_EXPERIENCE="1", DOTNET_NOLOGO="1",
        DOTNET_CLI_TELEMETRY_OPTOUT="1", DOTNET_CLI_WORKLOAD_UPDATE_NOTIFY_DISABLE="true",
        NUGET_PACKAGES=str(scratch / "packages"),
    )
    if command(".NET SDK check", ["dotnet", "--version"], scratch, env).strip() != SDK.encode():
        raise PilotError(f"The pilot requires .NET SDK {SDK}.")
    stage_nuget_feed(scratch)
    command(".NET offline restore", [
        "dotnet", "restore", "Pilot.csproj", "--locked-mode", "--configfile", "NuGet.Config",
        "--source", "./feed", "--runtime", "linux-x64",
    ], scratch, env)
    command(".NET publish", [
        "dotnet", "publish", "Pilot.csproj", "--no-restore", "-c", "Release", "-o", "publish",
    ], scratch, env)


def build_go(repository, scratch):
    env = dict(
        os.environ, GOTOOLCHAIN="local", GOWORK="off", GOPROXY="off", GOSUMDB="off",
        GOOS="linux", GOARCH="amd64", CGO_ENABLED="0", GOFLAGS="",
    )
    actual = command("Go toolchain check", ["go", "version"], scratch, env).decode("utf-8").strip()
    if not re.fullmatch(rf"go version {re.escape(GO_VERSION)} \S+", actual):
        raise PilotError(f"The pilot requires Go {GO_VERSION.removeprefix('go')}.")
    git_show = ["git", "-C", str(repository), "show"]
    go_sum = command("Pinned Go checksums lookup", [*git_show, f"{BASELINE}:{DRIVERLAB}/go.sum"], scratch)
    go_mod = command(
        "Pinned Go module lookup", [*git_show, f"{BASELINE}:{DRIVERLAB}/go.mod"], scratch,
    ).decode("utf-8")
    replacement = "replace github.com/igorpooh1978/GoEF => "
    if go_mod.count(replacement + "../../..") != 1:
        raise PilotError("The pinned driver module has an unexpected GoEF replacement.")
    go_mod = go_mod.replace(
        replacement + "../../..", replacement + json.dumps(repository.as_posix(), ensure_ascii=False),
    )
    (scratch / "go.sum").write_bytes(go_sum)
    (scratch / "go.mod").write_text(go_mod, encoding="utf-8", newline="\n")
    command("Offline Go build", ["go", "build", "-mod=readonly", "-o", "pgx-control", "main.go"], scratch, env)


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--bundle", type=Path, required=True, help="directory containing the extracted evidence archive")
    parser.add_argument("--repository", type=Path, default=ROOT, help="GoEF checkout with the recorded lab revision")
    args = parser.parse_args(argv)
    try:
        with TemporaryDirectory(prefix="goef-ef-postgres-pilot-") as directory:
            scratch = Path(directory)
            stage_bundle(args.bundle, scratch)
            repository = args.repository.resolve()
            build_dotnet(scratch)
            build_go(repository, scratch)
            evidence = command(
                "PostgreSQL comparison", [sys.executable, "-B", "run.py", str(repository)], scratch,
            )
            print(evidence.decode("utf-8").strip())
    except PilotError as error:
        print(f"FAIL {error}", file=sys.stderr)
        return 1
    except (OSError, ValueError):
        print("FAIL Could not prepare or read the pilot environment.", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
