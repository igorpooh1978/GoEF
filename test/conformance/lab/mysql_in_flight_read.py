"""Run the local MySQL in-flight SPI read characterization (GoEF #13)."""

import argparse
import json
from pathlib import Path
import secrets
from tempfile import TemporaryDirectory

from driver_run import BINARY, PROFILES, build_binary, mysql_container
from runtime import Blocked, Lab, LabError, safe_reason


MYSQL_PROFILES = ("mysql-8.0", "mysql-8.4")
RESULT_PREFIX = "GOEF_MYSQL_IN_FLIGHT_READ_RESULT="


def verified_result(output, profile):
    records = [
        line.removeprefix(RESULT_PREFIX)
        for line in output.splitlines()
        if line.startswith(RESULT_PREFIX)
    ]
    if len(records) != 1:
        raise LabError("The MySQL in-flight test must emit exactly one result.")
    try:
        observed = json.loads(records[0])
    except ValueError:
        raise LabError("The MySQL in-flight test emitted malformed evidence.") from None
    if not isinstance(observed, dict) or observed.get("cancel_phase") not in ("query", "first_next"):
        raise LabError("The MySQL in-flight test did not report an accepted cancellation phase.")
    expected = {
        "profile": profile,
        "server_version": PROFILES[profile].version,
        "status": "PASS",
        "wait_observed": "exact_innodb_threads",
        "release_control": "row_then_eof",
        "cancel_cause": "context.Canceled",
        "cancel_phase": observed["cancel_phase"],
        "dispatch_count": "1_each",
        "pool_after_cancel": "0_open_0_in_use",
        "recovery": "fresh_session_row_unchanged",
    }
    if observed != expected:
        raise LabError("The MySQL in-flight result did not match its expected evidence.")
    return expected


def run_profile(binary, profile):
    password, token = secrets.token_hex(32), secrets.token_hex(32)
    target = PROFILES[profile]
    with TemporaryDirectory(prefix="goef-mysql-in-flight-read-") as directory:
        lab = Lab(Path(directory), None, False)
        with mysql_container(lab, password, token, target) as container:
            container.docker(["cp", str(binary), f"{container.id}:{BINARY}"], timeout=60)
            container.docker(["exec", "--user", "0", container.id, "chmod", "0555", BINARY])
            output = container.exec(
                [BINARY, "-test.run", "^TestDriverMySQLInFlightRead$", "-test.count=1", "-test.timeout=60s", "-test.v"],
                env={
                    "GOEF_MYSQL_IN_FLIGHT_READ": "1",
                    "GOEF_DRIVERLAB_PROFILE": profile,
                    "GOEF_DRIVERLAB_PROVIDER": "mysql",
                    "GOEF_DRIVERLAB_PASSWORD": password,
                    "GOEF_DRIVERLAB_TOKEN": token,
                },
                timeout=85,
            )
    return {**verified_result(output, profile), "image": target.image}


def failure(error, profile=None):
    result = {
        "status": "BLOCKED" if isinstance(error, Blocked) else "FAIL",
        "reason": safe_reason(error),
    }
    if profile is not None:
        result["profile"] = profile
    return result


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--profile", choices=MYSQL_PROFILES)
    args = parser.parse_args(argv)
    profiles = (args.profile,) if args.profile else MYSQL_PROFILES
    with TemporaryDirectory(prefix="goef-mysql-in-flight-read-build-") as directory:
        try:
            binary = build_binary(Path(directory), "linux")
        except (LabError, OSError) as error:
            print(json.dumps(failure(error), sort_keys=True), flush=True)
            return 1
        failed = False
        for profile in profiles:
            try:
                result = run_profile(binary, profile)
            except (LabError, OSError) as error:
                result = failure(error, profile)
            print(json.dumps(result, sort_keys=True), flush=True)
            failed |= result["status"] != "PASS"
    return int(failed)


if __name__ == "__main__":
    raise SystemExit(main())
