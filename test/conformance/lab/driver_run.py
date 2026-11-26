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
SQLSERVER_EULA = "--accept-sqlserver-eula"
LEGACY_SQLSERVER_EULA = "--accept-legacy-sqlserver-eula"


@dataclass(frozen=True)
class SQLServerSettings:
    compatibility_level: int
    update_level: str
    eula_flag: str
    sqlcmd_path: str = sqlserver.SQLCMD[0]
    generate_series: str = ""


@dataclass(frozen=True)
class Profile:
    provider: str
    fact: str
    version: str
    image: str | None = None
    probe: str = ""
    client_flags: tuple[str, ...] = ()
    sqlserver_settings: SQLServerSettings | None = None


PROFILES = {
    "postgres-18": Profile("postgres", "engine_version", "18.7", postgres.IMAGE, "180007"),
    "postgres-19": Profile(
        "postgres", "engine_version", "19.0",
        "docker.io/library/postgres@sha256:3fb624f672a1cb628d0bdc0f533cffad31345421eedf16ab1f85ed1ea0d07d2e",
        "190000",
    ),
    "postgres-17": Profile(
        "postgres", "engine_version", "17.12",
        "docker.io/library/postgres@sha256:e711be2c7ff64c9eb8883fc87571621be6caa2f943d825a49e5d20ac5b0221d7",
        "170012",
    ),
    "postgres-16": Profile(
        "postgres", "engine_version", "16.16",
        "docker.io/library/postgres@sha256:e8a7960781a4aeb21928aa4565124f0e469bd27e0bcc4321d8b9102145312f84",
        "160016",
    ),
    "sqlite-3.54": Profile("sqlite", "engine_version", "3.54.1"),
    "sqlserver-2022": Profile(
        "sqlserver", "version", "16.0.4325.4", sqlserver.IMAGE,
        sqlserver_settings=SQLServerSettings(160, "CU28", SQLSERVER_EULA, generate_series="available"),
    ),
    "sqlserver-2022-140": Profile(
        "sqlserver", "version", "16.0.4325.4", sqlserver.IMAGE,
        sqlserver_settings=SQLServerSettings(140, "CU28", SQLSERVER_EULA, generate_series="unavailable"),
    ),
    "sqlserver-2019": Profile(
        "sqlserver", "version", "15.0.4510.2",
        "mcr.microsoft.com/mssql/server@sha256:debdc21628cddd1a4433371c601c8dee9bad3533ae8b18fe9dc8c5596368b10d",
        sqlserver_settings=SQLServerSettings(150, "CU32", LEGACY_SQLSERVER_EULA),
    ),
    "sqlserver-2017": Profile(
        "sqlserver", "version", "14.0.3570.2",
        "mcr.microsoft.com/mssql/server@sha256:5f6fd78f8752390dbd3584b91dee463f1f4b2b30beef9af3fd35753d32940c52",
        sqlserver_settings=SQLServerSettings(140, "CU31", LEGACY_SQLSERVER_EULA, "/opt/mssql-tools/bin/sqlcmd"),
    ),
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
    "generate_series", "generate_series_native_code",
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
    expected = profile.sqlserver_settings
    if not lab.allow_sqlserver:
        raise Blocked(f"SQL Server requires {expected.eula_flag} for this local run.")
    provision_password = "GoEF9!" + secrets.token_hex(24)
    environment = {
        "ACCEPT_EULA": "Y",
        "MSSQL_PID": "Developer",
        "MSSQL_SA_PASSWORD": provision_password,
    }
    with lab.container(profile.image, environment) as container:
        settings = {"SQLCMDPASSWORD": provision_password}
        client = [expected.sqlcmd_path, *sqlserver.SQLCMD[1:], "-N", "-d", "master"]
        container.wait([*client, "-Q", "SELECT 1;"], env=settings)
        identity = container.exec([*client, "-Q", sqlserver.SERVER_IDENTITY], env=settings)
        if identity.strip() != f"{profile.version}|{expected.update_level}|Developer Edition (64-bit)":
            raise LabError("SQL Server provisioning target does not match the pin.")
        container.exec(
            [*client, "-Q", "CREATE DATABASE [goef_smoke] COLLATE Latin1_General_100_BIN2;"],
            env=settings,
        )
        container.exec(
            [*client, "-Q", f"ALTER DATABASE [goef_smoke] SET COMPATIBILITY_LEVEL = {expected.compatibility_level};"],
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
        sqlserver_settings = profile.sqlserver_settings
        if sqlserver_settings is not None:
            if (
                facts.get("compatibility_level") != str(sqlserver_settings.compatibility_level)
                or facts.get("product_update_level") != sqlserver_settings.update_level
            ):
                raise LabError("The scenario did not report the pinned SQL Server settings.")
            expected_series = sqlserver_settings.generate_series
            if expected_series and facts.get("generate_series") != expected_series:
                raise LabError("The scenario did not report the expected SQL Server GENERATE_SERIES availability.")
            if expected_series == "unavailable" and facts.get("generate_series_native_code") != "208":
                raise LabError("The scenario did not report SQL Server native code 208 for GENERATE_SERIES.")
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


def missing_eula(profile, accepted_eulas):
    settings = profile.sqlserver_settings
    if settings is not None and settings.eula_flag not in accepted_eulas:
        return settings.eula_flag
    return None


def run_provider(profile_name, binary, accepted_eulas):
    profile = PROFILES[profile_name]
    provider = profile.provider
    phase = "build"
    try:
        required_flag = missing_eula(profile, accepted_eulas)
        if required_flag is not None:
            raise Blocked(f"SQL Server requires {required_flag} for this local run.")
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
                lab = Lab(path, None, provider == "sqlserver")
                phase = "provisioning"
                with SERVER_CONTAINERS[provider](lab, password, token, profile) as container:
                    phase = "copy test binary"
                    container.docker(["cp", str(binary), f"{container.id}:{BINARY}"], timeout=60)
                    phase = "set test binary permissions"
                    container.docker(["exec", "--user", "0", container.id, "chmod", "0555", BINARY])
                    phase = "driver execution"
                    binary_env = {**env, "GOEF_DRIVERLAB_PASSWORD": password, "GOEF_DRIVERLAB_TOKEN": token}
                    if profile_name == "sqlserver-2017":
                        # The isolated 2017 image can generate a TLS certificate with a negative serial.
                        binary_env["GODEBUG"] = "x509negativeserial=1"
                    output = container.exec([BINARY, *TEST_ARGS], env=binary_env, timeout=200)
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
    parser.add_argument(SQLSERVER_EULA, action="store_true", help="accept the SQL Server 2022 Developer EULA")
    parser.add_argument(LEGACY_SQLSERVER_EULA, action="store_true", help="accept the selected SQL Server 2017 or 2019 Developer EULA")
    args = parser.parse_args(argv)
    providers = PROVIDERS if "all" in args.providers else tuple(dict.fromkeys(args.providers))
    if args.profile and providers != (PROFILES[args.profile].provider,):
        parser.error("--profile requires a single matching provider")
    accepted_eulas = {
        flag for flag, accepted in (
            (SQLSERVER_EULA, args.accept_sqlserver_eula),
            (LEGACY_SQLSERVER_EULA, args.accept_legacy_sqlserver_eula),
        ) if accepted
    }
    selected = {provider: args.profile or DEFAULT_PROFILES[provider] for provider in providers}
    enabled = [
        provider for provider, name in selected.items()
        if missing_eula(PROFILES[name], accepted_eulas) is None
    ]
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
                pool.submit(run_provider, name,
                            binaries.get("host" if provider == "sqlite" else "linux"),
                            accepted_eulas)
                for provider, name in selected.items()
            ]
            for task in tasks:
                result = task.result()
                print(json.dumps(result, sort_keys=True, allow_nan=False), flush=True)
                failed |= result["status"] != "PASS"
    return int(failed)


if __name__ == "__main__":
    raise SystemExit(main())
