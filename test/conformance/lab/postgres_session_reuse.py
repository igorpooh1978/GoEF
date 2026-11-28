"""Run the PostgreSQL session reuse scenarios in the isolated driver lab (#39)."""

import argparse
from pathlib import Path
import secrets
from tempfile import TemporaryDirectory

from driver_run import PROFILES, build_binary, postgres_container
from runtime import Lab, LabError


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--benchmark", action="store_true", help="also compare the three read paths")
    args = parser.parse_args()

    profile = "postgres-18"
    password, token = secrets.token_hex(32), secrets.token_hex(32)
    environment = {
        "GOEF_DRIVERLAB_PROVIDER": "postgres",
        "GOEF_DRIVERLAB_PROFILE": profile,
        "GOEF_DRIVERLAB_PASSWORD": password,
        "GOEF_DRIVERLAB_TOKEN": token,
    }
    with TemporaryDirectory(prefix="goef-postgres-reuse-") as directory:
        root = Path(directory)
        binary = build_binary(root, "linux")
        with postgres_container(Lab(root, None, False), password, token, PROFILES[profile]) as container:
            target = "/tmp/goef-reuse.test"
            container.docker(["cp", str(binary), f"{container.id}:{target}"], timeout=60)
            container.docker(["exec", "--user", "0", container.id, "chmod", "0555", target])
            output = container.exec(
                [target, "-test.run=^TestPostgresSessionReuse$", "-test.v", "-test.timeout=1m"],
                env=environment, timeout=75,
            )
            if "--- PASS: TestPostgresSessionReuse " not in output:
                raise LabError("The PostgreSQL session reuse test did not report a pass.")
            print(output, end="")
            if args.benchmark:
                print(container.exec(
                    [target, "-test.run=^$", "-test.bench=^BenchmarkPostgresSessionReuse$",
                     "-test.benchtime=150x", "-test.benchmem", "-test.count=3", "-test.timeout=2m"],
                    env=environment, timeout=135,
                ), end="")


if __name__ == "__main__":
    main()
