"""Run local W002 checks and the pinned GoEF database conformance matrix."""

import argparse
from collections import Counter
import json
import os
from pathlib import Path
import re
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[2]
LAB = ROOT / "test/conformance/lab"
sys.path.insert(0, str(LAB))

import driver_run
from runtime import LabError
import sqlite_compat


GO_VERSION = sqlite_compat.GO_VERSION
COMPATIBILITY_PROFILES = (
    "postgres-16", "postgres-17", "postgres-19", "mysql-8.0",
    "sqlserver-2017", "sqlserver-2019", "sqlserver-2022-140",
)
DRIVER_PINS = {
    "github.com/jackc/pgx/v5": "v5.13.0",
    "modernc.org/sqlite": "v1.63.0",
    "github.com/microsoft/go-mssqldb": "v1.11.0",
    "github.com/go-sql-driver/mysql": "v1.10.0",
}
FEATURE_SCENARIOS = {
    "model_types": {"model_types": ("MODEL-TYPE-001-S02",)},
    "owned_tx": {"owned_tx": ("TX-OWN-001-S02",)},
    "savepoint_tx": {"savepoint_tx": driver_run.SAVEPOINT_TX_SCENARIOS},
    "save_changes": {"save_changes": driver_run.SAVE_CHANGES_SCENARIOS},
    "tx_options": {"tx_options": ("TX-ISOLATION-001-S01",)},
    "decimal_binding": {
        "decimal_binding": ("MODEL-TYPE-001-S01",),
        "decimal_field_binding": ("MODEL-TYPE-001-S05",),
    },
    "commit_outcome": {"commit_outcome": tuple(f"TX-OUTCOME-001-S{i:02d}" for i in range(1, 6))},
    "production_adapters": {"production_adapters": driver_run.PRODUCTION_ADAPTER_SCENARIOS},
}


class CheckFailed(Exception):
    """A local check failed or did not report the required evidence."""


def parse_args(argv=None):
    parser = argparse.ArgumentParser(description=__doc__, allow_abbrev=False)
    parser.add_argument("--providers", nargs="+", choices=(*driver_run.PROVIDERS, "all"),
                        default=driver_run.PROVIDERS, help="select live database suites (default: all)")
    parser.add_argument("--compatibility", action="store_true", help="include older and newer pinned profiles")
    parser.add_argument(driver_run.SQLSERVER_EULA, action="store_true",
                        help="accept the SQL Server 2022 Developer EULA for this run")
    parser.add_argument(driver_run.LEGACY_SQLSERVER_EULA, action="store_true",
                        help="accept the SQL Server 2017 and 2019 Developer EULAs for this run")
    for feature, help_text in (
        ("model_types", "run the model type round trip"),
        ("owned_tx", "run the owned transaction scenarios"),
        ("savepoint_tx", "run the savepoint and external transaction scenarios"),
        ("save_changes", "run the SaveChanges transaction scenarios"),
        ("tx_options", "run the explicit isolation and read-only scenarios"),
        ("decimal_binding", "run the exact decimal binding scenarios"),
        ("commit_outcome", "run the synthetic commit outcome faults"),
        ("production_adapters", "run adapter module checks and the production adapter suite"),
    ):
        parser.add_argument("--" + feature.replace("_", "-"), action="store_true", help=help_text)
    args = parser.parse_args(argv)
    args.providers = driver_run.PROVIDERS if "all" in args.providers else tuple(dict.fromkeys(args.providers))
    return args


def driver_matrix_plan(args):
    baseline = {provider: driver_run.DEFAULT_PROFILES[provider] for provider in args.providers}
    yield args.providers, None, baseline
    if args.compatibility:
        for name in COMPATIBILITY_PROFILES:
            provider = driver_run.PROFILES[name].provider
            if provider in baseline:
                yield (provider,), name, {provider: name}


def run_check(argv, env, *, cwd=ROOT, capture=False, check=True):
    label = " ".join((Path(argv[0]).name, *argv[1:]))
    print(f"RUN {label}", flush=True)
    result = subprocess.run(
        argv, cwd=cwd, env=env, stdout=subprocess.PIPE if capture else None,
        encoding="utf-8", check=False,
    )
    if check:
        if result.returncode:
            raise CheckFailed(f"{label} exited with code {result.returncode}.")
        print(f"PASS {label}", flush=True)
    return result


def module_checks(env, production_adapters):
    version = run_check(["go", "version"], env, capture=True).stdout.strip()
    if not re.fullmatch(rf"go version {re.escape(GO_VERSION)} \S+", version):
        raise CheckFailed(f"The local Go toolchain must be exactly {GO_VERSION}.")
    print(f"PASS {version}", flush=True)

    for arguments in (
        ("test/bootstrap/verify.py",),
        ("-m", "unittest", "discover", "-s", "test/bootstrap", "-p", "test_verify.py"),
        ("-m", "unittest", "discover", "-s", "scripts/ci", "-p", "test_*.py"),
        ("-m", "unittest", "discover", "-s", "test/conformance/lab", "-p", "test*.py"),
    ):
        run_check([sys.executable, "-B", *arguments], env)

    modules = [ROOT, driver_run.MODULE]
    if production_adapters:
        modules.extend(ROOT / "providers" / name for name in driver_run.PROVIDERS)
    for module in modules:
        print(f"Go module: {module.relative_to(ROOT)}", flush=True)
        if module != ROOT:
            run_check(["go", "mod", "verify"], env, cwd=module)
        if module == driver_run.MODULE:
            verify_driver_pins(env)
        for arguments in (
            ("mod", "tidy", "-diff"),
            ("build", "-mod=readonly", "./..."),
            ("vet", "-mod=readonly", "./..."),
            ("test", "-mod=readonly", "-shuffle=on", "-count=1", "./..."),
        ):
            run_check(["go", *arguments], env, cwd=module)


def verify_driver_pins(env):
    template = "{{.Path}} {{.Version}}{{if .Replace}} (replaced){{end}}{{if .Indirect}} (indirect){{end}}"
    result = run_check(
        ["go", "list", "-mod=readonly", "-m", "-f", template, *DRIVER_PINS],
        env, cwd=driver_run.MODULE, capture=True,
    )
    expected = [f"{path} {version}" for path, version in DRIVER_PINS.items()]
    if Counter(result.stdout.splitlines()) != Counter(expected):
        raise CheckFailed("The driver module must select these exact direct dependencies: " + ", ".join(expected))
    for pin in expected:
        print(f"PASS driver pin {pin}", flush=True)


def unique_object(pairs):
    data = {}
    for key, value in pairs:
        if key in data:
            raise ValueError("Duplicate JSON key.")
        data[key] = value
    return data


def reject_json_constant(_):
    raise ValueError("Nonstandard JSON constant.")


def report_rows(stdout):
    rows = []
    for line in stdout.splitlines():
        try:
            report = json.loads(line, object_pairs_hook=unique_object, parse_constant=reject_json_constant)
        except ValueError:
            raise CheckFailed("The database runner emitted invalid JSON evidence.") from None
        if not isinstance(report, dict):
            raise CheckFailed("The database runner emitted a result other than a JSON object.")
        rows.append(report)
    return rows


def exact_rows(rows, expected, description):
    def canonical(row):
        return json.dumps(row, sort_keys=True, allow_nan=False)

    if not isinstance(rows, list) or Counter(map(canonical, rows)) != Counter(map(canonical, expected)):
        raise CheckFailed(f"The database runner did not report the expected {description}.")


def verify_feature_results(report, provider, versions, options, *, linked_sqlite=False):
    for feature, reports in FEATURE_SCENARIOS.items():
        if not getattr(options, feature):
            continue
        for key, scenarios in reports.items():
            expected = [
                {
                    "provider": provider, "profile": name, "engine_version": version,
                    "scenario": scenario, "status": "PASS",
                }
                for name, version in versions.items() for scenario in scenarios
            ]
            rows = report.get(key)
            if not linked_sqlite and len(scenarios) == 1:
                rows = [rows]
            exact_rows(rows, expected, key)
    if not options.production_adapters:
        return
    coverage = report.get("production_coverage")
    if linked_sqlite:
        if not isinstance(coverage, list) or any(not isinstance(row, dict) for row in coverage):
            raise CheckFailed("Missing SQLite production coverage.")
        exact_rows([row.get("profile") for row in coverage], list(versions), "SQLite coverage profiles")
    else:
        coverage = [coverage]
    for row in coverage:
        for name in ("adapter", "transport"):
            counts = row.get(name) if isinstance(row, dict) else None
            covered = counts.get("covered") if isinstance(counts, dict) else None
            total = counts.get("total") if isinstance(counts, dict) else None
            if type(covered) is not int or type(total) is not int or not 0 < covered <= total:
                raise CheckFailed(f"Invalid production coverage for {name}.")


def driver_markers(rows):
    if not isinstance(rows, list):
        raise CheckFailed("The database runner is missing the driver scenario list.")
    return "\n".join(driver_run.RESULT_PREFIX + json.dumps(row) for row in rows)


def verify_driver_report(report, expected_profile, options):
    profile = driver_run.PROFILES[expected_profile]
    if not isinstance(report, dict) or (
        report.get("profile") != expected_profile or report.get("provider") != profile.provider
    ):
        raise CheckFailed(f"The driver runner must report profile {expected_profile}.")
    if report.get("status") not in ("PASS", "FAIL", "BLOCKED"):
        raise CheckFailed("The driver runner reported an invalid status.")
    if report["status"] != "PASS":
        print(json.dumps(report, sort_keys=True), flush=True)
        raise CheckFailed(f"{expected_profile} is {report['status']}.")
    results = driver_run.verified_results(driver_markers(report.get("scenarios")), profile.provider, expected_profile)
    if profile.provider == "postgres" and any(row["facts"].get("server_version_num") != profile.probe for row in results):
        raise CheckFailed(f"{expected_profile} did not report the exact PostgreSQL version number.")
    verify_feature_results(report, profile.provider, {expected_profile: profile.version}, options)
    print(json.dumps(report, sort_keys=True), flush=True)


def run_driver_matrix(providers, name, profiles, options, env, suite_flags, accepted_eulas):
    selection = [*providers, "--profile", name] if name is not None else list(providers)
    result = run_check(
        [sys.executable, "-B", "test/conformance/lab/driver_run.py", *selection, *suite_flags, *accepted_eulas],
        env, capture=True, check=False,
    )
    reports = report_rows(result.stdout)
    exact_rows([report.get("provider") for report in reports], list(profiles), "driver providers")
    failures = []
    for report in reports:
        try:
            verify_driver_report(report, profiles[report["provider"]], options)
        except (CheckFailed, LabError) as error:
            failures.append(str(error))
    if result.returncode:
        failures.append(f"Driver runner exited with code {result.returncode}.")
    if failures:
        raise CheckFailed("\n".join(failures))
    label = name or ", ".join(providers)
    print(f"PASS W002 driver suites: {label}", flush=True)


def verify_sqlite_report(report, options):
    if not isinstance(report, dict) or report.get("status") not in ("PASS", "FAIL", "BLOCKED"):
        raise CheckFailed("SQLite compatibility reported an invalid status.")
    if report["status"] != "PASS":
        print(json.dumps(report, sort_keys=True), flush=True)
        raise CheckFailed(f"SQLite compatibility is {report['status']}.")
    if report.get("minimum_profile") != sqlite_compat.MINIMUM_PROFILE or report.get("go_version") != GO_VERSION:
        raise CheckFailed("SQLite compatibility reported the wrong minimum profile or Go toolchain.")
    profiles = report.get("profiles")
    if not isinstance(profiles, dict) or profiles.keys() != sqlite_compat.PROFILES.keys():
        raise CheckFailed("SQLite compatibility must report both exact linked runtimes.")
    for name, profile in sqlite_compat.PROFILES.items():
        observed = profiles[name]
        if not isinstance(observed, dict) or any(
            observed.get(key) != getattr(profile, key)
            for key in ("engine_version", "source_id", "driver_version", "libc_version")
        ):
            raise CheckFailed(f"SQLite compatibility reported different pins for {name}.")

    driver_rows = report.get("driver_scenarios")
    if not isinstance(driver_rows, list) or any(not isinstance(row, dict) for row in driver_rows):
        raise CheckFailed("SQLite compatibility is missing the driver scenarios.")
    exact_rows([row.get("profile") for row in driver_rows],
               [name for name in profiles for _ in driver_run.SCENARIOS], "linked-runtime driver profiles")
    for name, profile in sqlite_compat.PROFILES.items():
        rows = [row for row in driver_rows if row.get("profile") == name]
        sqlite_compat.verified_driver_results(driver_markers(rows), name, profile)

    exact_rows(report.get("checks"), [
        {
            "scenario": scenario, "journal_mode": mode, "status": "PASS",
            "engines": [sqlite_compat.PROFILES[name].engine_version for name in (source, target, source)],
            "strict_generated_native_code": sqlite_compat.STRICT_GENERATED_NATIVE_CODE,
        }
        for scenario, source, target in sqlite_compat.DIRECTIONS for mode in sqlite_compat.FILE_JOURNALS
    ], "SQLite file checks")
    for key, scenario, interrupted_exit in (
        ("rebuild_checks", sqlite_compat.REBUILD_SCENARIO, sqlite_compat.REBUILD_INTERRUPTED_EXIT),
        ("executor_rebuild_checks", sqlite_compat.EXECUTOR_REBUILD_SCENARIO, sqlite_compat.EXECUTOR_REBUILD_INTERRUPTED_EXIT),
    ):
        exact_rows(report.get(key), [
            {
                "scenario": scenario, "profile": name, "engine_version": profile.engine_version,
                "journal_mode": mode, "interrupted_exit": interrupted_exit, "status": "PASS",
            }
            for name, profile in sqlite_compat.PROFILES.items() for mode in sqlite_compat.FILE_JOURNALS
        ], key)
    versions = {name: profile.engine_version for name, profile in sqlite_compat.PROFILES.items()}
    verify_feature_results(report, "sqlite", versions, options, linked_sqlite=True)
    print(json.dumps(report, sort_keys=True), flush=True)


def run_sqlite_compatibility(options, env, suite_flags):
    result = run_check(
        [sys.executable, "-B", "test/conformance/lab/sqlite_compat.py", *suite_flags],
        env, capture=True, check=False,
    )
    reports = report_rows(result.stdout)
    if len(reports) != 1:
        raise CheckFailed(f"SQLite compatibility must emit one JSON report. Exit code {result.returncode}.")
    verify_sqlite_report(reports[0], options)
    if result.returncode:
        raise CheckFailed(f"SQLite compatibility exited with code {result.returncode}.")
    print("PASS W002 SQLite linked-runtime, file compatibility and interrupted rebuild suites", flush=True)


def accepted_eula_flags(args):
    return tuple(
        flag for accepted, flag in (
            (args.accept_sqlserver_eula, driver_run.SQLSERVER_EULA),
            (args.accept_legacy_sqlserver_eula, driver_run.LEGACY_SQLSERVER_EULA),
        )
        if accepted
    )


def run_matrix(args, env):
    suite_flags = ["--" + feature.replace("_", "-") for feature in FEATURE_SCENARIOS if getattr(args, feature)]
    accepted_eulas = accepted_eula_flags(args)
    failures = []
    distinct_profiles = set()
    for providers, name, profiles in driver_matrix_plan(args):
        distinct_profiles.update(profiles.values())
        missing = driver_run.missing_eula(driver_run.PROFILES[name], accepted_eulas) if name else None
        if missing:
            failures.append(f"{name} is BLOCKED. Supply {missing} for this run.")
            continue
        try:
            run_driver_matrix(providers, name, profiles, args, env, suite_flags, accepted_eulas)
        except (CheckFailed, LabError) as error:
            failures.append(str(error))
    if args.compatibility and "sqlite" in args.providers:
        try:
            run_sqlite_compatibility(args, env, suite_flags)
            distinct_profiles.update(sqlite_compat.PROFILES)
        except (CheckFailed, LabError) as error:
            failures.append(str(error))
    if failures:
        raise CheckFailed("\n".join(failures))
    label = "compatibility matrix" if args.compatibility else "database suites"
    print(f"PASS W002 {label} with {len(distinct_profiles)} distinct engine profiles", flush=True)


def main(argv=None):
    args = parse_args(argv)
    env = dict(os.environ, GOTOOLCHAIN="local", GOWORK="off", PYTHONIOENCODING="utf-8")
    env.pop("GOEF_DRIVERLAB_PROVIDER", None)
    try:
        module_checks(env, args.production_adapters)
        run_matrix(args, env)
    except (CheckFailed, LabError, OSError, UnicodeError) as error:
        print(f"FAIL {error}", file=sys.stderr, flush=True)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
