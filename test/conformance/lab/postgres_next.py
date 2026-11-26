"""Run PostgreSQL native row iteration cancellation on pinned local profiles."""

import argparse
import json
from pathlib import Path
import secrets
from tempfile import TemporaryDirectory

from driver_run import BINARY, PROFILES, build_binary, postgres_container
from runtime import Blocked, Lab, LabError, safe_reason


POSTGRES_PROFILES = {name: profile for name, profile in PROFILES.items() if profile.provider == "postgres"}
RESULT_PREFIX = "GOEF_POSTGRES_NEXT_RESULT="
TEST_ARGS = ("-test.run=^TestPostgresNativeNext$", "-test.v", "-test.count=1", "-test.timeout=45s")
OUTCOMES = {
    "cancel-native-next": {
        "terminal": "context.Canceled",
        "backend": "exited-before-unlock",
        "recovery": "original-row",
    },
    "holder-release-control": {"second": "2", "terminal": "clean-eof"},
}


def verified_results(output, profile):
    shared = {
        "engine_version": profile.version,
        "server_version_num": profile.probe,
        "first": "1",
        "phase": "native-next",
        "lock_wait": "observed",
    }
    expected = {
        name: {"provider": "postgres", "scenario": name, "status": "PASS", "facts": shared | facts}
        for name, facts in OUTCOMES.items()
    }
    observed = set()
    for line in output.splitlines():
        if not line.startswith(RESULT_PREFIX):
            continue
        try:
            report = json.loads(line.removeprefix(RESULT_PREFIX))
        except ValueError:
            raise LabError("Invalid PostgreSQL Next evidence.") from None
        name = report.get("scenario") if isinstance(report, dict) else None
        if not isinstance(name, str) or name in observed or report != expected.get(name):
            raise LabError("Unexpected PostgreSQL Next evidence.")
        observed.add(name)
    if observed != expected.keys():
        raise LabError("Both PostgreSQL Next scenarios must report complete evidence.")
    return list(expected.values())


def run_profile(profile_name, binary):
    report = {"provider": "postgres", "profile": profile_name}
    phase = "provisioning"
    try:
        profile = POSTGRES_PROFILES[profile_name]
        password = secrets.token_hex(32)
        token = secrets.token_hex(32)
        with TemporaryDirectory(prefix="goef-postgres-next-cell-") as temporary:
            lab = Lab(Path(temporary), None, False)
            with postgres_container(lab, password, token, profile) as container:
                phase = "copy test binary"
                container.docker(["cp", str(binary), f"{container.id}:{BINARY}"], timeout=60)
                container.docker(["exec", "--user", "0", container.id, "chmod", "0555", BINARY])
                phase = "driver execution"
                env = {
                    "GOEF_DRIVERLAB_PROVIDER": "postgres",
                    "GOEF_DRIVERLAB_PROFILE": profile_name,
                    "GOEF_DRIVERLAB_POSTGRES_NEXT": "1",
                    "GOEF_DRIVERLAB_PASSWORD": password,
                    "GOEF_DRIVERLAB_TOKEN": token,
                }
                output = container.exec([BINARY, *TEST_ARGS], env=env, timeout=60)
                phase = "cleanup"
        phase = "evidence"
        scenarios = verified_results(output, profile)
        return report | {"status": "PASS", "scenarios": scenarios}
    except Blocked as error:
        return report | {"status": "BLOCKED", "phase": phase, "reason": safe_reason(error)}
    except Exception as error:
        return report | {"status": "FAIL", "phase": phase, "reason": safe_reason(error)}


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--profile", choices=tuple(POSTGRES_PROFILES), action="append", required=True)
    args = parser.parse_args(argv)
    with TemporaryDirectory(prefix="goef-postgres-next-build-") as temporary:
        try:
            binary = build_binary(Path(temporary), "linux")
        except Exception as error:
            print(json.dumps({"provider": "postgres", "status": "FAIL", "phase": "build", "reason": safe_reason(error)}))
            return 1
        failed = False
        for name in dict.fromkeys(args.profile):
            report = run_profile(name, binary)
            print(json.dumps(report, sort_keys=True), flush=True)
            failed = failed or report["status"] != "PASS"
    return int(failed)


if __name__ == "__main__":
    raise SystemExit(main())
