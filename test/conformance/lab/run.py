"""Run pinned GoEF engine CLI smoke cells (issue #19)."""

import argparse
import importlib
import json
from pathlib import Path
import sys
from tempfile import TemporaryDirectory

sys.dont_write_bytecode = True
from runtime import Blocked, Lab, LabError


PROVIDERS = ("postgres", "sqlite", "sqlserver", "mysql")


def run_provider(provider, sqlite_binary, allow_sqlserver):
    try:
        if provider == "sqlserver" and not allow_sqlserver:
            raise Blocked("SQL Server requires --accept-sqlserver-eula for this run.")
        with TemporaryDirectory(prefix="goef-lab-") as directory:
            lab = Lab(Path(directory), sqlite_binary, allow_sqlserver)
            facts = importlib.import_module(provider).smoke(lab)
        return {"provider": provider, "status": "PASS", "facts": facts}
    except Blocked as exc:
        return {"provider": provider, "status": "BLOCKED", "reason": str(exc)}
    except LabError as exc:
        return {"provider": provider, "status": "FAIL", "reason": str(exc)}
    except Exception:
        # Unexpected third-party exceptions may contain SQL text, credentials or paths.
        return {"provider": provider, "status": "FAIL", "reason": "Unexpected lab failure."}


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("providers", nargs="+", choices=(*PROVIDERS, "all"))
    parser.add_argument("--sqlite-binary", type=Path, help="path to the pinned SQLite CLI")
    parser.add_argument("--accept-sqlserver-eula", action="store_true",
                        help="accept the SQL Server Developer EULA for this run only")
    args = parser.parse_args(argv)
    binary = args.sqlite_binary.resolve() if args.sqlite_binary else None
    providers = PROVIDERS if "all" in args.providers else dict.fromkeys(args.providers)
    failed = False
    for provider in providers:
        result = run_provider(provider, binary, args.accept_sqlserver_eula)
        print(json.dumps(result, sort_keys=True, allow_nan=False), flush=True)
        failed |= result["status"] != "PASS"
    return int(failed)


if __name__ == "__main__":
    raise SystemExit(main())
