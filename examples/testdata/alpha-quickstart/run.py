"""Run a GoEF database quickstart with a connection string."""

import argparse
from getpass import getpass
import os
import re
import subprocess
import sys


def localdb_connection_string():
    instance = "MSSQLLocalDB"
    subprocess.run(["sqllocaldb", "start", instance], check=True, stdout=subprocess.DEVNULL)
    info = subprocess.check_output(["sqllocaldb", "info", instance], text=True, errors="replace")
    match = re.search(r"\\\\\.\\pipe\\LOCALDB#[0-9A-F]+\\tsql\\query", info, re.IGNORECASE)
    if not match:
        raise RuntimeError("LocalDB did not report a named pipe.")
    return f"server={match[0]};protocol=np;database=master;authenticator=winsspi"


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--localdb", action="store_true", help="connect to Windows SQL Server Express LocalDB")
    args = parser.parse_args()
    if args.localdb and os.name != "nt":
        parser.error("--localdb requires Windows.")

    try:
        if args.localdb:
            dsn = localdb_connection_string()
        else:
            dsn = os.environ.get("GOEF_DSN") or getpass("Database connection string: ")
        return subprocess.run(
            ["go", "run", "-mod=readonly", "."],
            env=dict(os.environ, GOEF_DSN=dsn),
            check=False,
        ).returncode
    except (OSError, subprocess.CalledProcessError, RuntimeError) as error:
        print(error, file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
