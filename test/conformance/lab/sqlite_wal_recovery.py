"""Verify committed SQLite WAL recovery with both pinned local runtimes (GoEF #31)."""

import argparse
import hashlib
import json
from pathlib import Path
import secrets
import stat
from tempfile import TemporaryDirectory

from runtime import Blocked, LabError, command, safe_reason
from sqlite_compat import CURRENT, GO_VERSION, MINIMUM_PROFILE, PROFILES, build


DIRECTIONS = (
    ("LAB-SQLITE-WAL-001-S01", MINIMUM_PROFILE, CURRENT),
    ("LAB-SQLITE-WAL-001-S02", CURRENT, MINIMUM_PROFILE),
)
TEST = "^TestSQLiteWALRecoveryStep$"
RESULT_PREFIX = "GOEF_SQLITE_WAL_RESULT="
COMMITTED_EXIT = 67
MAIN_FILE = "compatibility.db"
WAL_FILE = MAIN_FILE + "-wal"


def run_stage(directory, binary, profile, stage, token):
    marker = f"{RESULT_PREFIX}{profile.engine_version}/{stage}"
    stops_after_commit = stage == "commit-exit"
    output = command(
        [str(binary), "-test.v", "-test.run", TEST, "-test.timeout", "30s"],
        cwd=directory, timeout=35,
        env={
            "GOEF_SQLITE_COMPAT_TOKEN": token,
            "GOEF_SQLITE_WAL_RECOVERY_STAGE": stage,
        },
        expected_failure=(COMMITTED_EXIT, marker) if stops_after_commit else None,
    )
    if not stops_after_commit:
        markers = [line for line in output.splitlines() if line.startswith(RESULT_PREFIX)]
        if markers != [marker]:
            raise LabError("The SQLite WAL recovery stage did not report its pinned runtime and outcome.")


def main_digest(path):
    if not stat.S_ISREG(path.lstat().st_mode):
        raise LabError("The SQLite WAL fixture is not a regular database file.")
    with path.open("rb") as file:
        return hashlib.file_digest(file, "sha256").digest()


def wal_size(path):
    try:
        info = path.lstat()
    except FileNotFoundError:
        return 0
    if not stat.S_ISREG(info.st_mode):
        raise LabError("The SQLite WAL sidecar is not a regular file.")
    return info.st_size


def run_direction(directory, binaries, scenario, origin, peer, report):
    directory.mkdir(mode=0o700)
    token = secrets.token_hex(32)
    with (directory / ".goef-sqlite-exchange").open("x", encoding="ascii") as marker:
        marker.write(token)
    main_path = directory / MAIN_FILE
    wal_path = directory / WAL_FILE

    report["phase"] = f"{scenario} seed"
    run_stage(directory, binaries[origin], PROFILES[origin], "seed", token)
    before = main_digest(main_path)

    report["phase"] = f"{scenario} commit-exit"
    run_stage(directory, binaries[origin], PROFILES[origin], "commit-exit", token)
    if wal_size(wal_path) <= 32 or main_digest(main_path) != before:
        raise LabError("The committed change must still be in WAL while the main file is unchanged.")

    report["phase"] = f"{scenario} checkpoint"
    run_stage(directory, binaries[peer], PROFILES[peer], "checkpoint", token)
    if wal_size(wal_path) != 0 or main_digest(main_path) == before:
        raise LabError("The checkpoint must move the committed change into the main file and empty the WAL.")

    report["phase"] = f"{scenario} reopen"
    run_stage(directory, binaries[origin], PROFILES[origin], "reopen", token)
    return {
        "scenario": scenario,
        "writer": origin,
        "checkpointing_peer": peer,
        "journal_mode": "wal",
        "committed_writer_exit": COMMITTED_EXIT,
        "status": "PASS",
    }


def main(argv=None):
    argparse.ArgumentParser(description=__doc__).parse_args(argv)
    report = {"status": "FAIL", "go_version": GO_VERSION, "profiles": {}, "checks": []}
    report["phase"] = "allocate temporary directory"
    try:
        with TemporaryDirectory(prefix="goef-sqlite-wal-") as directory:
            root = Path(directory)
            binaries = {}
            for name, profile in PROFILES.items():
                report["phase"] = f"build {name}"
                binaries[name], platform = build(root, name, profile)
                report["profiles"][name] = {
                    "engine_version": profile.engine_version,
                    "source_id": profile.source_id,
                    "driver_version": profile.driver_version,
                    "libc_version": profile.libc_version,
                    "platform": platform,
                }
            for scenario, origin, peer in DIRECTIONS:
                report["checks"].append(run_direction(root / scenario, binaries, scenario, origin, peer, report))
            report["phase"] = "cleanup"
    except Blocked as error:
        report.update(status="BLOCKED", reason=safe_reason(error))
    except (LabError, OSError, UnicodeError) as error:
        report["reason"] = safe_reason(error)
    else:
        report["status"] = "PASS"
        del report["phase"]
    print(json.dumps(report, sort_keys=True, allow_nan=False), flush=True)
    return {"PASS": 0, "FAIL": 1, "BLOCKED": 2}[report["status"]]


if __name__ == "__main__":
    raise SystemExit(main())
