"""Run the transactional migration scenarios on disposable PostgreSQL 18.7."""

import json
from pathlib import Path
import secrets
from tempfile import TemporaryDirectory

from driver_run import (
    BINARY, GOEF_MODULE, MODULE, PROFILES, _verified_profile_scenarios, postgres_container,
)
from runtime import Blocked, Lab, LabError, command, inherited, safe_reason


PROFILE = "postgres-18"
RESULT_PREFIX = "GOEF_POSTGRES_MIGRATION_RESULT="
SCENARIOS = tuple(f"MIG-POSTGRES-001-S{number:02}" for number in range(1, 7))
PACKAGE = f"{GOEF_MODULE}/providers/postgres/migration"
COVERAGE_FILE = "/tmp/goef-postgres-migrations.cover"
TEST_ARGS = (
    "-test.run=^TestPostgresMigrations$", "-test.v", "-test.count=1",
    "-test.timeout=45s", f"-test.coverprofile={COVERAGE_FILE}",
)


def build_binary(path):
    env = {
        **inherited(("HOME", "USERPROFILE", "LOCALAPPDATA", "XDG_CACHE_HOME")),
        "GOENV": "off", "GOTOOLCHAIN": "local", "GOWORK": "off",
        "GOPROXY": "https://proxy.golang.org", "GOSUMDB": "sum.golang.org",
        "GOOS": "linux", "GOARCH": "amd64", "CGO_ENABLED": "0",
    }
    command([
        "go", "test", "-c", "-mod=readonly", "-covermode=atomic", f"-coverpkg={PACKAGE}",
        "-o", str(path), ".",
    ], cwd=MODULE, env=env, timeout=600)


def coverage(profile):
    lines = profile.read_text(encoding="utf-8").splitlines()
    if not lines or lines[0] != "mode: atomic":
        raise LabError("Missing atomic coverage for PostgreSQL migration package.")
    blocks = set()
    covered = total = 0
    for line in lines[1:]:
        location, statements, count = line.split()
        if not location.startswith(PACKAGE + "/") or location in blocks:
            raise LabError("Unexpected PostgreSQL migration coverage block.")
        blocks.add(location)
        total += int(statements)
        if int(count):
            covered += int(statements)
    if not total or not covered:
        raise LabError("No instrumented PostgreSQL migration statements were executed.")
    return {"covered": covered, "total": total}


def main():
    profile = PROFILES[PROFILE]
    report = {"provider": "postgres", "profile": PROFILE}
    phase = "build"
    try:
        with TemporaryDirectory(prefix="goef-postgres-migrations-") as temporary:
            path = Path(temporary)
            binary = path / "driverlab"
            build_binary(binary)
            password = secrets.token_hex(32)
            token = secrets.token_hex(32)
            phase = "provisioning"
            lab = Lab(path, None, False)
            with postgres_container(lab, password, token, profile) as container:
                phase = "copy test binary"
                container.docker(["cp", str(binary), f"{container.id}:{BINARY}"], timeout=60)
                container.docker(["exec", "--user", "0", container.id, "chmod", "0555", BINARY])
                phase = "driver execution"
                env = {
                    "GOEF_DRIVERLAB_PROVIDER": "postgres",
                    "GOEF_DRIVERLAB_PROFILE": PROFILE,
                    "GOEF_DRIVERLAB_POSTGRES_MIGRATIONS": "1",
                    "GOEF_DRIVERLAB_PASSWORD": password,
                    "GOEF_DRIVERLAB_TOKEN": token,
                }
                output = container.exec([BINARY, *TEST_ARGS], env=env, timeout=60)
                phase = "collect coverage"
                profile_path = path / "migration.cover"
                container.docker(["cp", f"{container.id}:{COVERAGE_FILE}", str(profile_path)], timeout=60)
                measured = coverage(profile_path)
                phase = "cleanup"
            phase = "evidence"
            scenarios = _verified_profile_scenarios(output, RESULT_PREFIX, SCENARIOS, "postgres", PROFILE, profile.version)
    except Blocked as error:
        print(json.dumps(report | {"status": "BLOCKED", "phase": phase, "reason": safe_reason(error)}))
        return 1
    except Exception as error:
        print(json.dumps(report | {"status": "FAIL", "phase": phase, "reason": safe_reason(error)}))
        return 1
    print(json.dumps(report | {"status": "PASS", "scenarios": scenarios, "coverage": measured}, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
