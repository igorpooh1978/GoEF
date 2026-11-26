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
from runtime import (
    Blocked, CleanupFailed, CommandError, CommandFailed, CommandTimedOut, Lab, LabError, ReadinessTimedOut,
    command, inherited, safe_reason,
)


PROVIDERS = ("postgres", "sqlite", "sqlserver", "mysql")
SCENARIOS = {f"LAB-DRIVER-001-S{i:02d}" for i in range(1, 8)}
CANCELLATION_SCENARIO = "LAB-DRIVER-001-S04"
CANCELLATION_FACTS = {"cancel_cause": "context.Canceled", "pool_in_use_after": "0"}
LOCK_TIMEOUT_SCENARIO = "LAB-DRIVER-001-S05"
LOCK_TIMEOUT_OUTCOME_FIELDS = {"lock_deadline_cause", "lock_wait_observed", "lock_timeout_native"}
PRE_DISPATCH_SCENARIO = "LAB-DRIVER-001-S06"
PRE_DISPATCH_FACTS = {
    "pre_dispatch_canceled": "context.Canceled",
    "pre_dispatch_deadline": "context.DeadlineExceeded",
    "pre_dispatch_calls": "0,0,1",
    "pre_dispatch_pool_in_use_after": "0",
}
ACQUIRE_CANCELLATION_SCENARIO = "LAB-DRIVER-001-S07"
ACQUIRE_CANCELLATION_FACTS = {
    "acquire_cancel_cause": "context.Canceled",
    "acquire_wait_count_delta": "1",
    "acquire_pool_in_use_after": "0",
}
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
LOCK_TIMEOUT_CODES = {"postgres": "55P03", "sqlite": "5", "sqlserver": "1222", "mysql": "1205/HY000"}

TEST_ARGS = ("-test.run=^TestDriverGherkin$", "-test.v", "-test.count=1", "-test.timeout=3m")
MODEL_TYPES_RESULT_PREFIX = "GOEF_MODEL_TYPES_RESULT="
OWNED_TX_RESULT_PREFIX = "GOEF_OWNED_TX_RESULT="
SAVEPOINT_TX_RESULT_PREFIX = "GOEF_SAVEPOINT_TX_RESULT="
SAVEPOINT_TX_SCENARIOS = ("TX-OWN-001-S01", "TX-SAVEPOINT-001-S03")
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
        # token_hex generates the entire value. Outside the fixture database,
        # the login reads only the marker and the table used to observe waits.
        container.exec(
            client, stdin=f"""\
CREATE DATABASE goef_marker;
CREATE TABLE goef_marker.target (token VARCHAR(64) NOT NULL) ENGINE=InnoDB;
INSERT INTO goef_marker.target VALUES ('{token}');
GRANT SELECT ON goef_marker.target TO 'goef_driver'@'%';
GRANT SELECT ON performance_schema.data_lock_waits TO 'goef_driver'@'%';
""", env=settings,
        )
        yield container


class _SQLServerIdentityMismatch(LabError):
    REASON = "SQL Server provisioning target does not match the pin."

    def __init__(self):
        super().__init__(self.REASON)


@contextmanager
def sqlserver_container(lab, password, token, profile, *, lock_observer_password, report_stage=None):
    expected = profile.sqlserver_settings
    if not lab.allow_sqlserver:
        raise Blocked(f"SQL Server requires {expected.eula_flag} for this local run.")
    if report_stage is None:
        report_stage = lambda _stage: None
    report_stage("docker-startup")
    provision_password = "GoEF9!" + secrets.token_hex(24)
    environment = {
        "ACCEPT_EULA": "Y",
        "MSSQL_PID": "Developer",
        "MSSQL_SA_PASSWORD": provision_password,
    }
    with lab.container(profile.image, environment) as container:
        settings = {"SQLCMDPASSWORD": provision_password}
        client = [expected.sqlcmd_path, *sqlserver.SQLCMD[1:], "-N", "-d", "master"]
        report_stage("engine-readiness")
        container.wait([*client, "-Q", "SELECT 1;"], env=settings)
        report_stage("engine-identity")
        identity = container.exec([*client, "-Q", sqlserver.SERVER_IDENTITY], env=settings)
        if identity.strip() != f"{profile.version}|{expected.update_level}|Developer Edition (64-bit)":
            raise _SQLServerIdentityMismatch()
        report_stage("fixture-setup")
        container.exec(
            [*client, "-Q", "CREATE DATABASE [goef_smoke] COLLATE Latin1_General_100_BIN2;"],
            env=settings,
        )
        container.exec(
            [*client, "-Q", f"ALTER DATABASE [goef_smoke] SET COMPATIBILITY_LEVEL = {expected.compatibility_level};"],
            env=settings,
        )
        # The login passwords and marker are generated internally. Secrets
        # are passed on stdin. Only the observer has the diagnostic grant.
        observer_permission = (
            "VIEW SERVER PERFORMANCE STATE" if int(profile.version.split(".", 1)[0]) >= 16 else "VIEW SERVER STATE"
        )
        container.exec(client, stdin=f"""\
CREATE LOGIN [goef_driver] WITH PASSWORD = N'{password}';
GO
CREATE LOGIN [goef_lock_observer] WITH PASSWORD = N'{lock_observer_password}';
GO
GRANT {observer_permission} TO [goef_lock_observer];
GO
USE [goef_smoke];
GO
CREATE SCHEMA [goef_fixture] AUTHORIZATION [dbo];
GO
CREATE USER [goef_driver] FOR LOGIN [goef_driver] WITH DEFAULT_SCHEMA = [goef_fixture];
CREATE USER [goef_lock_observer] FOR LOGIN [goef_lock_observer];
CREATE TABLE dbo.goef_driver_marker (token VARCHAR(64) NOT NULL);
INSERT INTO dbo.goef_driver_marker VALUES ('{token}');
GRANT SELECT ON dbo.goef_driver_marker TO [goef_driver], [goef_lock_observer];
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


def verified_cancellation_facts(facts):
    if "native_code" in facts or any(facts.get(key) != value for key, value in CANCELLATION_FACTS.items()):
        raise LabError("The scenario did not report the expected cancellation cause and released connection pool.")
    return {key: facts[key] for key in CANCELLATION_FACTS}


def verified_lock_timeout_facts(facts, provider):
    expected = {
        "lock_deadline_cause": "context.DeadlineExceeded",
        "lock_wait_observed": "both",
        "lock_timeout_native": LOCK_TIMEOUT_CODES[provider],
    }
    if "native_code" in facts or any(facts.get(key) != value for key, value in expected.items()):
        raise LabError("The scenario did not report both observed waits and the expected lock timeout outcomes.")
    return {key: facts[key] for key in expected}


def verified_pre_dispatch_facts(facts):
    unrelated = {"native_code", *LOCK_TIMEOUT_OUTCOME_FIELDS}
    if unrelated.intersection(facts) or any(facts.get(key) != value for key, value in PRE_DISPATCH_FACTS.items()):
        raise LabError("The scenario did not report the expected pre-dispatch causes, call counts and released pool.")
    return {key: facts[key] for key in PRE_DISPATCH_FACTS}


def verified_acquire_cancellation_facts(facts):
    unrelated = {"native_code", *CANCELLATION_FACTS, *LOCK_TIMEOUT_OUTCOME_FIELDS, *PRE_DISPATCH_FACTS}
    if unrelated.intersection(facts) or any(facts.get(key) != value for key, value in ACQUIRE_CANCELLATION_FACTS.items()):
        raise LabError("The scenario did not report canceled acquisition, one wait and a released pool.")
    return {key: facts[key] for key in ACQUIRE_CANCELLATION_FACTS}


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
        outcome = {}
        if scenario == CANCELLATION_SCENARIO:
            outcome = verified_cancellation_facts(facts)
        elif scenario == LOCK_TIMEOUT_SCENARIO:
            outcome = verified_lock_timeout_facts(facts, provider)
        elif scenario == PRE_DISPATCH_SCENARIO:
            outcome = verified_pre_dispatch_facts(facts)
        elif scenario == ACQUIRE_CANCELLATION_SCENARIO:
            outcome = verified_acquire_cancellation_facts(facts)
        elif scenario != "LAB-DRIVER-001-S01":
            expected = NATIVE_CODES[provider][0 if scenario == "LAB-DRIVER-001-S02" else 1]
            if facts.get("native_code") != expected:
                raise LabError("The scenario did not report the expected native constraint code.")
        facts = {
            key: value for key, value in facts.items()
            if key in FACT_KEYS and isinstance(value, str) and len(value) <= 256
            and all(c.isprintable() for c in value)
        }
        facts.update(outcome)
        results[scenario] = {"provider": provider, "scenario": scenario, "status": "PASS", "facts": facts}
    if set(results) != SCENARIOS:
        raise LabError("The driver test did not report every required scenario.")
    return [results[key] for key in sorted(results)]


def driver_test_options(*, model_types=False, owned_tx=False, savepoint_tx=False):
    tests = ["TestDriverGherkin"]
    environment = {}
    if model_types:
        tests.append("TestModelTypeRoundTrip")
        environment["GOEF_DRIVERLAB_MODEL_TYPES"] = "1"
    if owned_tx:
        tests.append("TestOwnedTransaction")
        environment["GOEF_DRIVERLAB_OWNED_TX"] = "1"
    if savepoint_tx:
        tests.append("TestSavepointAndExternalTransaction")
        environment["GOEF_DRIVERLAB_SAVEPOINT_TX"] = "1"
    if not environment:
        return TEST_ARGS, environment
    return (f"-test.run=^({'|'.join(tests)})$", *TEST_ARGS[1:]), environment


def _verified_profile_scenarios(output, prefix, scenarios, provider, profile, engine_version):
    scope = ", ".join(scenarios)
    markers = [
        line.removeprefix(prefix)
        for line in output.splitlines() if line.startswith(prefix)
    ]
    if len(markers) != len(scenarios):
        raise LabError(f"{scope} must emit one result per scenario.")
    try:
        rows = [json.loads(marker) for marker in markers]
    except ValueError:
        raise LabError(f"{scope} emitted malformed evidence.") from None
    expected = [{
        "provider": provider, "profile": profile, "engine_version": engine_version,
        "scenario": scenario, "status": "PASS",
    } for scenario in scenarios]
    if any(rows.count(row) != 1 for row in expected):
        raise LabError(f"{scope} did not verify the selected profile and outcome.")
    return expected


def verified_model_type_result(output, provider, profile, engine_version):
    return _verified_profile_scenarios(output, MODEL_TYPES_RESULT_PREFIX, ("MODEL-TYPE-001-S02",), provider, profile, engine_version)[0]


def verified_owned_tx_result(output, provider, profile, engine_version):
    return _verified_profile_scenarios(output, OWNED_TX_RESULT_PREFIX, ("TX-OWN-001-S02",), provider, profile, engine_version)[0]


def verified_savepoint_tx_results(output, provider, profile, engine_version):
    return _verified_profile_scenarios(output, SAVEPOINT_TX_RESULT_PREFIX, SAVEPOINT_TX_SCENARIOS, provider, profile, engine_version)


def missing_eula(profile, accepted_eulas):
    settings = profile.sqlserver_settings
    if settings is not None and settings.eula_flag not in accepted_eulas:
        return settings.eula_flag
    return None


def sqlserver_provisioning_reason(error):
    # Select fixed diagnostics without exposing exception messages or subprocess data.
    if isinstance(error, CleanupFailed):
        return f"{sqlserver_provisioning_reason(error.primary)} Cleanup failed."
    if isinstance(error, CommandTimedOut):
        return "Command timed out."
    if isinstance(error, ReadinessTimedOut):
        return "Engine readiness deadline expired."
    if isinstance(error, _SQLServerIdentityMismatch):
        return _SQLServerIdentityMismatch.REASON
    if isinstance(error, CommandFailed) and type(error.returncode) is int:
        return f"Command exited with status {error.returncode}."
    if isinstance(error, Blocked):
        return "A SQL Server provisioning prerequisite is unavailable."
    return "SQL Server provisioning failed."


def run_provider(profile_name, binary, accepted_eulas, *, model_types=False, owned_tx=False, savepoint_tx=False):
    profile = PROFILES[profile_name]
    provider = profile.provider
    phase = "build"
    provisioning_stage = None

    def report_stage(stage):
        nonlocal provisioning_stage
        provisioning_stage = stage

    try:
        required_flag = missing_eula(profile, accepted_eulas)
        if required_flag is not None:
            raise Blocked(f"SQL Server requires {required_flag} for this local run.")
        if isinstance(binary, LabError):
            raise binary
        with TemporaryDirectory(prefix="goef-driver-cell-") as directory:
            path = Path(directory)
            env = {"GOEF_DRIVERLAB_PROVIDER": provider, "GOEF_DRIVERLAB_PROFILE": profile_name}
            test_args, extra_env = driver_test_options(model_types=model_types, owned_tx=owned_tx, savepoint_tx=savepoint_tx)
            env.update(extra_env)
            if provider == "sqlite":
                phase = "driver execution"
                output = command([str(binary), *test_args], cwd=path, env=env, timeout=200)
            else:
                password = "GoEF9!" + secrets.token_hex(24) if provider == "sqlserver" else secrets.token_hex(32)
                token = secrets.token_hex(32)
                lab = Lab(path, None, provider == "sqlserver")
                phase = "provisioning"
                provision_options = {}
                if provider == "sqlserver":
                    lock_observer_password = "GoEF9!" + secrets.token_hex(24)
                    provision_options["lock_observer_password"] = lock_observer_password
                    provision_options["report_stage"] = report_stage
                    env["GOEF_DRIVERLAB_SQLSERVER_OBSERVER_PASSWORD"] = lock_observer_password
                with SERVER_CONTAINERS[provider](lab, password, token, profile, **provision_options) as container:
                    phase = "copy test binary"
                    container.docker(["cp", str(binary), f"{container.id}:{BINARY}"], timeout=60)
                    phase = "set test binary permissions"
                    container.docker(["exec", "--user", "0", container.id, "chmod", "0555", BINARY])
                    phase = "driver execution"
                    binary_env = {**env, "GOEF_DRIVERLAB_PASSWORD": password, "GOEF_DRIVERLAB_TOKEN": token}
                    if profile_name == "sqlserver-2017":
                        # The isolated 2017 image can generate a TLS certificate with a negative serial.
                        binary_env["GODEBUG"] = "x509negativeserial=1"
                    output = container.exec([BINARY, *test_args], env=binary_env, timeout=200)
        phase = "evidence"
        results = verified_results(output, provider, profile_name)
        report = {"provider": provider, "profile": profile_name, "status": "PASS", "scenarios": results}
        if model_types:
            report["model_types"] = verified_model_type_result(output, provider, profile_name, profile.version)
        if owned_tx:
            report["owned_tx"] = verified_owned_tx_result(output, provider, profile_name, profile.version)
        if savepoint_tx:
            report["savepoint_tx"] = verified_savepoint_tx_results(output, provider, profile_name, profile.version)
        return report
    except Exception as error:
        blocked = isinstance(error, Blocked)
        report = {"provider": provider, "profile": profile_name, "status": "BLOCKED" if blocked else "FAIL"}
        if not blocked:
            report["phase"] = phase
        if phase == "provisioning" and provisioning_stage is not None:
            report["stage"] = provisioning_stage
            report["reason"] = sqlserver_provisioning_reason(error)
        else:
            report["reason"] = safe_reason(error)
        return report


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("providers", nargs="+", choices=(*PROVIDERS, "all"))
    parser.add_argument("--profile", choices=tuple(PROFILES))
    parser.add_argument("--model-types", action="store_true", help="also run the model type round-trip scenario")
    parser.add_argument("--owned-tx", action="store_true", help="also run the owned transaction scenario")
    parser.add_argument("--savepoint-tx", action="store_true", help="also run savepoint and external transaction scenarios")
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
                            accepted_eulas, model_types=args.model_types, owned_tx=args.owned_tx,
                            savepoint_tx=args.savepoint_tx)
                for provider, name in selected.items()
            ]
            for task in tasks:
                result = task.result()
                print(json.dumps(result, sort_keys=True, allow_nan=False), flush=True)
                failed |= result["status"] != "PASS"
    return int(failed)


if __name__ == "__main__":
    raise SystemExit(main())
