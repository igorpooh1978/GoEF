import argparse
import os
from pathlib import Path
import secrets
import socket
import subprocess
import sys
import tempfile


def reserve_port():
    with socket.socket() as listener:
        listener.bind(("127.0.0.1", 0))
        return listener.getsockname()[1]


def run_test(bin_dir: Path, count: int, coverprofile: Path | None):
    root = Path(__file__).resolve().parents[3]
    suffix = ".exe" if os.name == "nt" else ""
    run_options = {"creationflags": subprocess.CREATE_NO_WINDOW} if os.name == "nt" else {}
    with tempfile.TemporaryDirectory(prefix="goef-native-tx-") as directory:
        fixture = Path(directory)
        data = fixture / "db"
        password = secrets.token_hex(30)
        marker = secrets.token_hex(20)
        port = reserve_port()
        password_file = fixture / "init-password"
        password_file.write_text(password + "\n", encoding="ascii")
        with (fixture / "server-control.log").open("ab") as log:
            def control(executable, *args, timeout=40):
                subprocess.run(
                    [str(bin_dir / (executable + suffix)), *args], cwd=fixture,
                    stdout=log, stderr=subprocess.STDOUT, check=True, timeout=timeout,
                    **run_options,
                )

            try:
                control(
                    "initdb", "-D", str(data), "--username=goef_native_tx",
                    "--auth=scram-sha-256", "--encoding=UTF8", "--locale=C",
                    "--pwfile=" + str(password_file),
                )
                password_file.unlink()
                control(
                    "pg_ctl", "-D", str(data), "-l", str(fixture / "server.log"),
                    "-w", "-t", "20", "-o",
                    f"-h 127.0.0.1 -p {port} -c max_connections=8 "
                    f"-c goef.native_tx_fixture={marker}", "start",
                )
                env = {key: value for key, value in os.environ.items() if not key.startswith("PG")}
                env.update({
                    "GOWORK": "off", "PGHOST": "127.0.0.1", "PGPORT": str(port),
                    "PGDATABASE": "postgres", "PGUSER": "goef_native_tx",
                    "PGPASSWORD": password, "PGSSLMODE": "disable",
                    "GOEF_NATIVE_TX_FIXTURE": marker,
                })
                command = ["go", "test", "-v", "-count", str(count)]
                if coverprofile is not None:
                    command.extend([
                        "-coverpkg=.,github.com/igorpooh1978/GoEF/transaction",
                        "-coverprofile", str(coverprofile),
                    ])
                return subprocess.run(
                    [*command, "."],
                    cwd=root / "providers" / "postgres", env=env, timeout=120,
                ).returncode
            finally:
                if (data / "postmaster.pid").exists():
                    control("pg_ctl", "-D", str(data), "-m", "fast", "-w", "-t", "20", "stop")


def main():
    parser = argparse.ArgumentParser(description="Run the native transaction scenarios on a private PostgreSQL 18.7 cluster.")
    parser.add_argument("--bin-dir", required=True, type=Path)
    parser.add_argument("--count", type=int, default=1)
    parser.add_argument("--coverprofile", type=Path)
    options = parser.parse_args()
    if options.count < 1:
        parser.error("--count must be positive")
    coverprofile = options.coverprofile.resolve() if options.coverprofile is not None else None
    try:
        return run_test(options.bin_dir.resolve(strict=True), options.count, coverprofile)
    except (OSError, subprocess.SubprocessError) as error:
        print(f"Native transaction lab failed ({type(error).__name__}).", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
