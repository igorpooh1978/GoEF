"""Run the pinned W002 Go driver characterization on the local workstation."""

import argparse
from concurrent.futures import ThreadPoolExecutor
from contextlib import contextmanager
from dataclasses import dataclass
import json
import os
from pathlib import Path
import secrets
import sys
from tempfile import TemporaryDirectory

sys.dont_write_bytecode = True
import mysql
import postgres
import sqlserver
from runtime import Blocked, CommandError, Lab, LabError, command, inherited, safe_reason


PROVIDERS = ("postgres", "sqlite", "sqlserver", "mysql")
SCENARIOS = {f"LAB-DRIVER-001-S{i:02d}" for i in range(1, 4)}
MODULE = Path(__file__).resolve().parents[1] / "driverlab"
BINARY = "/tmp/goef-driverlab"
RESULT_PREFIX = "GOEF_DRIVERLAB_RESULT="
@dataclass(frozen=True)
class Profile:
    provider: str
    fact: str
    version: str
    image: str | None = None
    probe: str = ""
    client_flags: tuple[str, ...] = ()


PROFILES = {
    "postgres-18": Profile("postgres", "engine_version", "18.7", postgres.IMAGE, "180007"),
    "postgres-16": Profile(
        "postgres", "engine_version", "16.16",
        "docker.io/library/postgres@sha256:e8a7960781a4aeb21928aa4565124f0e469bd27e0bcc4321d8b9102145312f84",
        "160016",
    ),
    "sqlite-3.54": Profile("sqlite", "engine_version", "3.54.1"),
    "sqlserver-2022": Profile("sqlserver", "version", "16.0.4325.4", sqlserver.IMAGE),
    "mysql-8.4": Profile(
        "mysql", "server_version", "8.4.13.1", mysql.IMAGE, "VERSION_WITH_UPDATE()",
        ("--no-login-paths",),
    ),
    "mysql-8.0": Profile(
        "mysql", "server_version", "8.0.46",
        "docker.io/library/mysql@sha256:62fb722c78b24245ddff1796a0fcee4a49cc5b87e0aaaf20c92d1da9e0a2497b",
        "VERSION()",
    ),
}

DEFAULT_PROFILES = {
    "postgres": "postgres-18", "sqlite": "sqlite-3.54",
    "sqlserver": "sqlserver-2022", "mysql": "mysql-8.4",
}

NATIVE_CODES = {
    "postgres": ("23505", "23503"),
    "sqlite": ("2067", "787"),
    "sqlserver": ("2627", "547"),
    "mysql": ("1062/23000", "1452/23000"),
}

TEST_ARGS = ("-test.run=^TestDriverGherkin$", "-test.v", "-test.count=1", "-test.timeout=3m")
FACT_KEYS = {
    "autocommit", "busy_timeout_ms", "character_set_connection", "collation", "collation_connection",
    "compatibility_level", "compile_options_count", "compile_options_sha256", "database", "driver", "edition",
    "encoding", "engine_version", "foreign_keys", "isolation", "journal_mode", "native_code",
    "native_constraint_code", "product_update_level", "query_mode", "read_committed_snapshot",
    "server_version", "server_version_num", "snapshot_isolation", "source_id", "sql_mode", "storage_engine",
    "time_zone", "transaction_isolation", "user", "variant", "version", "xact_abort",
    "search_path", "login", "default_schema", "transport", "encryption",
}

PG_SETUP = r"""
\getenv fixture_password GOEF_PROVISION_PASSWORD
\getenv target_token GOEF_DRIVERLAB_TOKEN
CREATE ROLE goef_driver LOGIN PASSWORD :'fixture_password';
CREATE TABLE goef_driver_marker (token varchar(64) NOT NULL);
INSERT INTO goef_driver_marker VALUES (:'target_token');
GRANT USAGE, CREATE ON SCHEMA public TO goef_driver;
GRANT SELECT ON goef_driver_marker TO goef_driver;
"""


@contextmanager
def postgres_container(lab, password, token, profile):
    provision_password = secrets.token_hex(32)
    environment = {
        "POSTGRES_DB": postgres.DATABASE,
        "POSTGRES_PASSWORD": provision_password,
        "POSTGRES_INITDB_ARGS": "--auth-host=scram-sha-256 --auth-local=trust --encoding=UTF8 --locale=C",
        "TZ": "UTC",
    }
    with lab.container(profile.image, environment) as container:
        container.wait(["pg_isready", "--quiet", "--timeout=3", *postgres.TARGET])
        client = {"PGPASSWORD": provision_password, "PGCONNECT_TIMEOUT": "5"}
        identity = container.exec(
            postgres.PSQL,
            stdin="SELECT current_setting('server_version_num'), current_database();",
            env=client,
        )
        if identity.splitlines() != [f"{profile.probe}|{postgres.DATABASE}"]:
            raise LabError("PostgreSQL provisioning target does not match the pin.")
        container.exec(
            postgres.PSQL, stdin=PG_SETUP,
            env={**client, "GOEF_PROVISION_PASSWORD": password, "GOEF_DRIVERLAB_TOKEN": token},
        )
        yield container


@contextmanager
def mysql_container(lab, password, token, profile):
    provision_password = secrets.token_hex(32)
    environment = {
        "MYSQL_DATABASE": mysql.DATABASE,
        "MYSQL_USER": "goef_driver",
        "MYSQL_ROOT_PASSWORD": provision_password,
        "MYSQL_PASSWORD": password,
    }
    client = [
        "mysql", "--no-defaults", *profile.client_flags, "--protocol=SOCKET",
        "--user=root", "--batch", "--skip-column-names",
    ]
    with lab.container(profile.image, environment) as container:
        settings = {
            "MYSQL_PWD": provision_password,
            "MYSQL_TEST_LOGIN_FILE": "/tmp/goef-no-login.cnf",
        }
        container.wait(["sh", "-c", mysql.READY, "mysql-readiness", *client, "--execute=SELECT 1"], env=settings)
        identity = container.exec(
            client, stdin=f"SELECT {profile.probe}, @@version_comment;", env=settings,
        )
        if identity.strip() != f"{profile.version}\t{mysql.VARIANT}":
            raise LabError("MySQL provisioning target does not match the pin.")
        # token_hex generates the entire value. The login has fixture database
        # privileges and can only read this separate provisioner-owned marker.
        container.exec(
            client, stdin=f"""\
CREATE DATABASE goef_marker;
CREATE TABLE goef_marker.target (token VARCHAR(64) NOT NULL) ENGINE=InnoDB;
INSERT INTO goef_marker.target VALUES ('{token}');
GRANT SELECT ON goef_marker.target TO 'goef_driver'@'%';
""", env=settings,
        )
        yield container


@contextmanager
def sqlserver_container(lab, password, token, profile):
    if not lab.allow_sqlserver:
        raise Blocked("SQL Server requires --accept-sqlserver-eula for this local run.")
    provision_password = "GoEF9!" + secrets.token_hex(24)
    environment = {
        "ACCEPT_EULA": "Y",
        "MSSQL_PID": "Developer",
        "MSSQL_SA_PASSWORD": provision_password,
    }
    with lab.container(profile.image, environment) as container:
        settings = {"SQLCMDPASSWORD": provision_password}
        client = [*sqlserver.SQLCMD, "-d", "master"]
        container.wait([*client, "-Q", "SELECT 1;"], env=settings)
        identity = container.exec([*client, "-Q", sqlserver.SERVER_IDENTITY], env=settings)
        if identity.strip() != f"{profile.version}|CU28|Developer Edition (64-bit)":
            raise LabError("SQL Server provisioning target does not match the pin.")
        container.exec(
            [*client, "-Q", "CREATE DATABASE [goef_smoke] COLLATE Latin1_General_100_BIN2;"],
            env=settings,
        )
        container.exec(
            [*client, "-Q", "ALTER DATABASE [goef_smoke] SET COMPATIBILITY_LEVEL = 160;"],
            env=settings,
        )
        # Both interpolated values are generated internally from hex and a
        # fixed password prefix. Secrets are sent in stdin, off the argv.
        container.exec(client, stdin=f"""\
CREATE LOGIN [goef_driver] WITH PASSWORD = N'{password}';
GO
USE [goef_smoke];
GO
CREATE SCHEMA [goef_fixture] AUTHORIZATION [dbo];
GO
CREATE USER [goef_driver] FOR LOGIN [goef_driver] WITH DEFAULT_SCHEMA = [goef_fixture];
CREATE TABLE dbo.goef_driver_marker (token VARCHAR(64) NOT NULL);
INSERT INTO dbo.goef_driver_marker VALUES ('{token}');
GRANT SELECT ON dbo.goef_driver_marker TO [goef_driver];
GRANT CREATE TABLE TO [goef_driver];
GRANT ALTER, SELECT, INSERT, UPDATE, DELETE, REFERENCES ON SCHEMA::[goef_fixture] TO [goef_driver];
GO
""", env=settings)
        yield container


SERVER_CONTAINERS = {"postgres": postgres_container, "mysql": mysql_container, "sqlserver": sqlserver_container}


def build_binary(tempdir, target):
    env = {
        **inherited(("HOME", "USERPROFILE", "LOCALAPPDATA", "XDG_CACHE_HOME")),
        "GOENV": "off",
        "GOTOOLCHAIN": "local",
        "GOWORK": "off",
        "GOPROXY": "https://proxy.golang.org",
        "GOSUMDB": "sum.golang.org",
        "CGO_ENABLED": "0",
    }
    suffix = ".exe" if target == "host" and os.name == "nt" else ""
    binary = tempdir / f"driverlab-{target}{suffix}"
    if target == "linux":
        env.update(GOOS="linux", GOARCH="amd64")
    try:
        command(
            ["go", "test", "-c", "-mod=readonly", "-ldflags=-s -w", "-o", str(binary), "."],
            cwd=MODULE, env=env, timeout=600,
        )
    except CommandError:
        raise LabError(f"Could not build the {target} Go driver test binary.") from None
    if not binary.is_file():
        raise LabError(f"The {target} Go compiler produced no test binary.")
    return binary


def verified_results(output, provider, profile_name=None):
    profile = PROFILES[profile_name or DEFAULT_PROFILES[provider]]
    results = {}
    for line in output.splitlines():
        if not line.startswith(RESULT_PREFIX):
            continue
        try:
            item = json.loads(line.removeprefix(RESULT_PREFIX))
        except ValueError:
            raise LabError("The driver test emitted malformed scenario evidence.") from None
        if not isinstance(item, dict) or item.get("provider") != provider or item.get("status") != "PASS":
            raise LabError("The driver test emitted unverified scenario evidence.")
        scenario = item.get("scenario")
        if not isinstance(scenario, str) or scenario not in SCENARIOS or scenario in results:
            raise LabError("The driver test emitted an unknown or duplicate scenario.")
        facts = item.get("facts", {})
        if not isinstance(facts, dict):
            raise LabError("The driver test emitted invalid scenario facts.")
        if facts.get(profile.fact) != profile.version:
            raise LabError("The scenario did not report the pinned engine identity.")
        if scenario != "LAB-DRIVER-001-S01":
            expected = NATIVE_CODES[provider][0 if scenario == "LAB-DRIVER-001-S02" else 1]
            if facts.get("native_code") != expected:
                raise LabError("The scenario did not report the expected native constraint code.")
        facts = {
            key: value for key, value in facts.items()
            if key in FACT_KEYS and isinstance(value, str) and len(value) <= 256
            and all(c.isprintable() for c in value)
        }
        results[scenario] = {"provider": provider, "scenario": scenario, "status": "PASS", "facts": facts}
    if set(results) != SCENARIOS:
        raise LabError("The driver test did not report every required scenario.")
    return [results[key] for key in sorted(results)]


def run_provider(profile_name, binary, allow_sqlserver):
    profile = PROFILES[profile_name]
    provider = profile.provider
    phase = "build"
    try:
        if provider == "sqlserver" and not allow_sqlserver:
            raise Blocked("SQL Server requires --accept-sqlserver-eula for this local run.")
        if isinstance(binary, LabError):
            raise binary
        with TemporaryDirectory(prefix="goef-driver-cell-") as directory:
            path = Path(directory)
            env = {"GOEF_DRIVERLAB_PROVIDER": provider, "GOEF_DRIVERLAB_PROFILE": profile_name}
            if provider == "sqlite":
                phase = "driver execution"
                output = command([str(binary), *TEST_ARGS], cwd=path, env=env, timeout=200)
            else:
                password = "GoEF9!" + secrets.token_hex(24) if provider == "sqlserver" else secrets.token_hex(32)
                token = secrets.token_hex(32)
                lab = Lab(path, None, allow_sqlserver)
                phase = "provisioning"
                with SERVER_CONTAINERS[provider](lab, password, token, profile) as container:
                    phase = "copy test binary"
                    container.docker(["cp", str(binary), f"{container.id}:{BINARY}"], timeout=60)
                    phase = "set test binary permissions"
                    container.docker(["exec", "--user", "0", container.id, "chmod", "0555", BINARY])
                    phase = "driver execution"
                    output = container.exec(
                        [BINARY, *TEST_ARGS],
                        env={**env, "GOEF_DRIVERLAB_PASSWORD": password, "GOEF_DRIVERLAB_TOKEN": token},
                        timeout=200,
                    )
        phase = "evidence"
        results = verified_results(output, provider, profile_name)
        return {"provider": provider, "profile": profile_name, "status": "PASS", "scenarios": results}
    except Blocked as error:
        return {"provider": provider, "profile": profile_name, "status": "BLOCKED", "reason": safe_reason(error)}
    except Exception as error:
        return {"provider": provider, "profile": profile_name, "status": "FAIL", "phase": phase, "reason": safe_reason(error)}


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("providers", nargs="+", choices=(*PROVIDERS, "all"))
    parser.add_argument("--profile", choices=tuple(PROFILES))
    parser.add_argument("--accept-sqlserver-eula", action="store_true")
    args = parser.parse_args(argv)
    providers = PROVIDERS if "all" in args.providers else tuple(dict.fromkeys(args.providers))
    if args.profile and providers != (PROFILES[args.profile].provider,):
        parser.error("--profile requires a single matching provider")
    enabled = [p for p in providers if p != "sqlserver" or args.accept_sqlserver_eula]
    with TemporaryDirectory(prefix="goef-driver-build-") as directory:
        path = Path(directory)
        binaries = {}
        for target in sorted({"host" if p == "sqlite" else "linux" for p in enabled}):
            try:
                binaries[target] = build_binary(path, target)
            except LabError as error:
                binaries[target] = error
        failed = False
        with ThreadPoolExecutor(max_workers=min(4, len(providers))) as pool:
            tasks = [
                pool.submit(run_provider, args.profile or DEFAULT_PROFILES[p],
                            binaries.get("host" if p == "sqlite" else "linux"),
                            args.accept_sqlserver_eula)
                for p in providers
            ]
            for task in tasks:
                result = task.result()
                print(json.dumps(result, sort_keys=True, allow_nan=False), flush=True)
                failed |= result["status"] != "PASS"
    return int(failed)


if __name__ == "__main__":
    raise SystemExit(main())
