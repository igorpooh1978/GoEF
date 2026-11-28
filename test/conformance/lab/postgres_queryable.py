"""Run the typed equality projection against an isolated PostgreSQL 18.7 container."""

import json
from pathlib import Path
import secrets
from tempfile import TemporaryDirectory

from driver_run import BINARY, PROFILES, build_binary, postgres_container
from runtime import Blocked, Lab, LabError, safe_reason


PROFILE = "postgres-18"
TEST = "TestPostgresQueryableEquality"
SCENARIOS = ("LINQ-PG-001-S01", "LINQ-PG-001-S02", "LINQ-PG-001-S03")


def main():
    phase = "build"
    profile = PROFILES[PROFILE]
    try:
        with TemporaryDirectory(prefix="goef-queryable-postgres-") as temporary:
            directory = Path(temporary)
            binary = build_binary(directory, "linux")
            phase = "provisioning"
            password = secrets.token_hex(32)
            token = secrets.token_hex(32)
            lab = Lab(directory, None, False)
            with postgres_container(lab, password, token, profile) as container:
                phase = "copy test binary"
                container.docker(["cp", str(binary), f"{container.id}:{BINARY}"], timeout=60)
                container.docker(["exec", "--user", "0", container.id, "chmod", "0555", BINARY])
                phase = "driver execution"
                output = container.exec(
                    [BINARY, f"-test.run=^{TEST}$", "-test.v", "-test.count=1", "-test.timeout=45s"],
                    env={
                        "GOEF_DRIVERLAB_PROVIDER": "postgres",
                        "GOEF_DRIVERLAB_PROFILE": PROFILE,
                        "GOEF_DRIVERLAB_POSTGRES_QUERYABLE": "1",
                        "GOEF_DRIVERLAB_PASSWORD": password,
                        "GOEF_DRIVERLAB_TOKEN": token,
                    },
                    timeout=60,
                )
                phase = "cleanup"
        phase = "evidence"
        for scenario in SCENARIOS:
            marker = f"--- PASS: {TEST}/{scenario} ("
            if sum(line.strip().startswith(marker) for line in output.splitlines()) != 1:
                raise LabError("PostgreSQL Queryable run did not complete every scenario.")
    except LabError as error:
        print(json.dumps({"phase": phase, "status": "BLOCKED" if isinstance(error, Blocked) else "FAIL",
                          "reason": safe_reason(error)}))
        return 1
    print(json.dumps({"provider": "postgres", "engine_version": profile.version, "status": "PASS",
                      "scenarios": SCENARIOS}))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
