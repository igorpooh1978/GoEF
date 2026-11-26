"""Exercise the pinned SQLite runtimes and their common file schema (GoEF #31)."""

import argparse
from dataclasses import dataclass
from itertools import takewhile
import json
import os
from pathlib import Path
import re
import secrets
from tempfile import TemporaryDirectory

from driver_run import (
    ACQUIRE_CANCELLATION_SCENARIO, CANCELLATION_SCENARIO, COVERAGE_ARGS, LOCK_TIMEOUT_SCENARIO, MODULE,
    PRE_DISPATCH_SCENARIO, RESULT_PREFIX, SCENARIOS, driver_test_options, production_coverage,
    verified_acquire_cancellation_facts, verified_cancellation_facts, verified_commit_outcome_results,
    verified_decimal_binding_result, verified_decimal_field_binding_result,
    verified_lock_timeout_facts, verified_model_type_result, verified_owned_tx_result, verified_pre_dispatch_facts,
    verified_production_adapter_results, verified_save_changes_results, verified_savepoint_tx_results, verified_tx_options_result,
)
from runtime import Blocked, LabError, command, inherited, safe_reason


@dataclass(frozen=True)
class Profile:
    engine_version: str
    source_id: str
    driver_version: str
    libc_version: str
    build_flags: tuple[str, ...] = ()


MINIMUM_PROFILE = "sqlite-3.51"
CURRENT = "sqlite-3.54"
PROFILES = {
    MINIMUM_PROFILE: Profile(
        "3.51.3",
        "2026-03-13 10:38:09 737ae4a34738ffa0c3ff7f9bb18df914dd1cad163f28fd6b6e114a344fe6d618",
        "v1.47.0",
        "v1.70.0",
        ("-modfile=sqlite351.mod", "-tags=goef_sqlite_351"),
    ),
    CURRENT: Profile(
        "3.54.1",
        "2026-10-16 14:45:20 555e07ef33f69a0957d24d8e555b702464a131d6f1607f4c3c1e4551e8e6ca76",
        "v1.63.0",
        "v1.80.1",
    ),
}
DIRECTIONS = (
    ("LAB-SQLITE-FILE-001-S01", MINIMUM_PROFILE, CURRENT),
    ("LAB-SQLITE-FILE-001-S02", CURRENT, MINIMUM_PROFILE),
)
FILE_FEATURE = MODULE / "features" / "sqlite_files.feature"
FILE_JOURNALS = {"delete", "wal"}
NATIVE_CODES = {
    "LAB-DRIVER-001-S02": "2067",
    "LAB-DRIVER-001-S03": "787",
}
FILE_RESULT_PREFIX = "GOEF_SQLITE_FILE_RESULT="
STRICT_GENERATED_NATIVE_CODE = "3091"
REBUILD_FEATURE = MODULE / "features" / "sqlite_rebuild.feature"
REBUILD_SCENARIO = "LAB-SQLITE-REBUILD-001-S01"
REBUILD_RESULT_PREFIX = "GOEF_SQLITE_REBUILD_RESULT="
REBUILD_INTERRUPTED_EXIT = 68
EXECUTOR_REBUILD_FEATURE = MODULE.parents[2] / "specs" / "features" / "migrations" / "mig-sqlite-001.feature"
EXECUTOR_REBUILD_SCENARIO = "MIG-SQLITE-001-S03"
EXECUTOR_REBUILD_RESULT_PREFIX = "GOEF_SQLITE_EXECUTOR_REBUILD_RESULT="
EXECUTOR_REBUILD_INTERRUPTED_EXIT = 69
GO_VERSION = "go1.27.4"


@dataclass(frozen=True)
class FileExample:
    scenario: str
    journal: str
    stages: tuple[tuple[str, str], ...]


@dataclass(frozen=True)
class RebuildExample:
    profile: str
    journal: str
    stages: tuple[str, ...]


def parse_file_feature(source):
    lines = iter(line.strip() for line in source.splitlines() if line.strip())

    def expect(expected):
        if next(lines, None) != expected:
            raise LabError("The SQLite file feature does not match the executable scenario contract.")

    expect("@LAB-SQLITE-FILE-001")
    expect("Feature: SQLite file compatibility across linked runtimes")
    expect("Both pinned runtimes can reopen and change a file whose schema uses features they share.")
    examples = []
    for scenario, origin, peer in DIRECTIONS:
        origin_version = PROFILES[origin].engine_version
        peer_version = PROFILES[peer].engine_version
        expect(f"@{scenario}")
        expect(f"Scenario Outline: SQLite {origin_version} reopens a file migrated by SQLite {peer_version}")
        # Each group of bound steps is checked by one child process.
        bindings = (
            ("create", origin, (
                f"Given SQLite {origin_version} has created a synthetic database in <journal_mode> mode",
                "And the file contains related rows, an audit trigger and one migration history entry",
            )),
            ("mutate", peer, (
                f"When SQLite {peer_version} reads the rows and rolls back a migration after a unique violation",
                "Then the existing schema, rows, audit records and migration history are preserved",
                f"And SQLite {peer_version} rejects a noncoercible stored generated value with its native datatype constraint code",
                f"When SQLite {peer_version} commits the compatible additive migration and closes the file",
            )),
            ("reopen", origin, (
                f"Then SQLite {origin_version} reopens the same file and reads the new schema and committed rows",
                "And the audit records and migration history include the committed changes only",
                "And integrity and foreign-key checks succeed",
            )),
        )
        for _, _, steps in bindings:
            for step in steps:
                expect(step)
        stages = tuple((stage, profile) for stage, profile, _ in bindings)
        expect("Examples:")
        if not re.fullmatch(r"\|\s*journal_mode\s*\|", next(lines, "")):
            raise LabError("The SQLite file feature requires a journal_mode examples column.")
        remaining = FILE_JOURNALS.copy()
        for _ in FILE_JOURNALS:
            row = re.fullmatch(r"\|\s*([a-z]+)\s*\|", next(lines, ""))
            if row is None or row[1] not in remaining:
                raise LabError("The SQLite file feature requires exactly one DELETE and one WAL example per direction.")
            journal = row[1]
            remaining.remove(journal)
            examples.append(FileExample(scenario, journal, stages))
    if next(lines, None) is not None:
        raise LabError("The SQLite file feature contains an unbound line.")
    return examples


def parse_rebuild_feature(source):
    lines = iter(line.strip() for line in source.splitlines() if line.strip())

    def expect(expected):
        if next(lines, None) != expected:
            raise LabError("The SQLite rebuild feature contains a missing or unbound step.")

    expect("@LAB-SQLITE-REBUILD-001")
    expect("Feature: Recover a SQLite table rebuild interrupted before commit")
    expect(f"@{REBUILD_SCENARIO}")
    expect("Scenario Outline: An interrupted rebuild preserves the original schema, rows and history")
    bindings = (
        ("seed", ("Given SQLite <engine> has seeded a synthetic database in <journal_mode> mode",)),
        ("interrupt", (
            "When a worker copies the writable columns, swaps the table, recreates its index and trigger, and writes within its transaction",
            "And the worker verifies the rebuilt table and exits before recording history or committing",
        )),
        ("resume", (
            "Then a new process verifies the original schema, rows, sequence, dependencies and history",
            "When that process commits the rebuild and history in one transaction",
        )),
        ("reopen", ("Then an independent process reads the rebuilt table, preserved rows and exactly one new history entry",)),
    )
    for _, steps in bindings:
        for step in steps:
            expect(step)
    expect("Examples:")
    return parse_rebuild_examples(lines, tuple(stage for stage, _ in bindings))


def parse_executor_rebuild_feature(source):
    spec_lines = [line.strip() for line in source.splitlines() if line.strip()]
    tag = f"@{EXECUTOR_REBUILD_SCENARIO}"
    if spec_lines.count(tag) != 1:
        raise LabError("The SQLite migration executor rebuild scenario must appear exactly once.")
    lines = takewhile(
        lambda line: not re.fullmatch(r"@MIG-SQLITE-001-S[0-9]+", line),
        spec_lines[spec_lines.index(tag) + 1:],
    )
    bindings = (
        ("seed", ("Given a file-backed SQLite <engine> database in <journal_mode> mode with a verified applied lineage",)),
        ("interrupt", ("When a process exits after the GoEF executor checks the rebuilt table and before it publishes history",)),
        ("resume", (
            "Then another executor acquires the writer lock and checks the original schema, values and history",
            "And the executor rejects ambiguous state and commits a proven unapplied rebuild with its history exactly once",
        )),
        ("reopen", ("And a separate reader verifies the committed values, history and foreign-key integrity",)),
    )
    if next(lines, None) != "Scenario Outline: Resume a reviewed transactional rebuild after process exit":
        raise LabError("The SQLite migration executor scenario does not match the executable contract.")
    for _, steps in bindings:
        for step in steps:
            if next(lines, None) != step:
                raise LabError("The SQLite migration executor scenario contains a missing or unbound step.")
    if next(lines, None) != "Examples:":
        raise LabError("The SQLite migration executor scenario examples are missing.")
    return parse_rebuild_examples(lines, tuple(stage for stage, _ in bindings))


def parse_rebuild_examples(lines, stages):
    if not re.fullmatch(r"\|\s*engine\s*\|\s*journal_mode\s*\|", next(lines, "")):
        raise LabError("The SQLite rebuild feature requires engine and journal_mode examples columns.")

    profile_names = {profile.engine_version: name for name, profile in PROFILES.items()}
    remaining = {(version, mode) for version in profile_names for mode in FILE_JOURNALS}
    examples = []
    for _ in range(len(remaining)):
        row = re.fullmatch(r"\|\s*([0-9.]+)\s*\|\s*([a-z]+)\s*\|", next(lines, ""))
        if row is None or row.groups() not in remaining:
            raise LabError("The SQLite rebuild feature requires one DELETE and one WAL example per pinned engine.")
        version, journal = row.groups()
        remaining.remove((version, journal))
        examples.append(RebuildExample(profile_names[version], journal, stages))
    if next(lines, None) is not None:
        raise LabError("The SQLite rebuild feature contains an unbound line.")
    return examples


def build(tempdir, name, profile, *, coverage=False):
    environment = {
        **inherited(("HOME", "USERPROFILE", "LOCALAPPDATA", "XDG_CACHE_HOME")),
        "GOENV": "off",
        "GOTOOLCHAIN": "local",
        "GOWORK": "off",
        "GOPROXY": "https://proxy.golang.org",
        "GOSUMDB": "sum.golang.org",
        "CGO_ENABLED": "0",
    }
    platform = command(["go", "env", "GOVERSION", "GOOS", "GOARCH"], cwd=MODULE, env=environment).splitlines()
    if len(platform) != 3 or platform[0] != GO_VERSION:
        raise Blocked("The SQLite compatibility lab requires Go 1.27.4.")
    suffix = ("-covered" if coverage else "") + (".exe" if os.name == "nt" else "")
    binary = tempdir / (name + suffix)
    command(
        ["go", "test", "-c", "-mod=readonly", *profile.build_flags, "-o", str(binary),
         *(COVERAGE_ARGS if coverage else ()), "."],
        cwd=MODULE, env=environment, timeout=600,
    )
    if not binary.is_file():
        raise LabError("The Go compiler produced no SQLite compatibility binary.")
    # Verify what is present in the compiled binary, as well as the runtime pin
    # checked later by both the Gherkin scenarios and the file exchange worker.
    build_info = command(["go", "version", "-m", str(binary)], cwd=MODULE, env=environment)
    for module, version in (
        ("modernc.org/sqlite", profile.driver_version), ("modernc.org/libc", profile.libc_version),
    ):
        matches = [line.split() for line in build_info.splitlines() if line.lstrip().startswith(f"dep\t{module}\t")]
        if len(matches) != 1 or len(matches[0]) != 4 or matches[0][2] != version or not matches[0][3].startswith("h1:"):
            raise LabError("The Go test binary does not contain the pinned SQLite dependencies.")
    return binary, "/".join(platform[1:])


def verified_driver_results(output, name, profile):
    results = {}
    required_settings = {
        "engine_version": profile.engine_version,
        "source_id": profile.source_id,
        "journal_mode": "wal",
        "foreign_keys": "1",
        "busy_timeout_ms": "1000",
    }
    for line in output.splitlines():
        if not line.startswith(RESULT_PREFIX):
            continue
        try:
            report = json.loads(line.removeprefix(RESULT_PREFIX))
        except ValueError:
            raise LabError("The SQLite driver emitted malformed scenario evidence.") from None
        if not isinstance(report, dict) or report.get("provider") != "sqlite" or report.get("status") != "PASS":
            raise LabError("The SQLite driver did not report a verified scenario.")
        scenario = report.get("scenario")
        facts = report.get("facts")
        if not isinstance(scenario, str) or scenario not in SCENARIOS or scenario in results or not isinstance(facts, dict):
            raise LabError("The SQLite driver reported invalid or duplicate scenario evidence.")
        if any(facts.get(key) != value for key, value in required_settings.items()):
            raise LabError("The SQLite driver did not report the pinned engine identity and settings.")
        count = facts.get("compile_options_count")
        digest = facts.get("compile_options_sha256")
        if not isinstance(count, str) or not re.fullmatch(r"[1-9][0-9]{0,3}", count):
            raise LabError("The SQLite driver did not report a valid compile-option count.")
        if not isinstance(digest, str) or not re.fullmatch(r"[0-9a-f]{64}", digest):
            raise LabError("The SQLite driver did not report a valid compile-option digest.")
        native = NATIVE_CODES.get(scenario)
        outcome = {}
        if scenario == CANCELLATION_SCENARIO:
            outcome = verified_cancellation_facts(facts)
        elif scenario == LOCK_TIMEOUT_SCENARIO:
            outcome = verified_lock_timeout_facts(facts, "sqlite")
        elif scenario == PRE_DISPATCH_SCENARIO:
            outcome = verified_pre_dispatch_facts(facts)
        elif scenario == ACQUIRE_CANCELLATION_SCENARIO:
            outcome = verified_acquire_cancellation_facts(facts)
        elif (native is None and "native_code" in facts) or (native is not None and facts.get("native_code") != native):
            raise LabError("The SQLite driver did not report the expected native constraint outcome.")
        results[scenario] = {
            "profile": name,
            "provider": "sqlite",
            "scenario": scenario,
            "status": "PASS",
            "facts": {
                **required_settings,
                "compile_options_count": count,
                "compile_options_sha256": digest,
                **({"native_code": native} if native is not None else {}),
                **outcome,
            },
        }
    if set(results) != SCENARIOS:
        raise LabError("The SQLite driver did not report all seven required scenarios.")
    return [results[scenario] for scenario in sorted(results)]


def run_driver(tempdir, binary, name, profile, *, model_types, owned_tx, savepoint_tx,
               save_changes=False, tx_options=False, decimal_binding=False, commit_outcome=False,
               production_adapters=False):
    test_args, env = driver_test_options(
        model_types=model_types, owned_tx=owned_tx, savepoint_tx=savepoint_tx,
        save_changes=save_changes, tx_options=tx_options, decimal_binding=decimal_binding,
        commit_outcome=commit_outcome, production_adapters=production_adapters,
    )
    env.update(GOEF_DRIVERLAB_PROVIDER="sqlite", GOEF_DRIVERLAB_PROFILE=name)
    coverage_path = tempdir / f"{name}-production.cover"
    if production_adapters:
        test_args = (*test_args, f"-test.coverprofile={coverage_path}")
    output = command(
        [str(binary), *test_args], cwd=tempdir, env=env, timeout=200,
    )
    extras = {}
    if model_types:
        extras["model_types"] = [verified_model_type_result(output, "sqlite", name, profile.engine_version)]
    if owned_tx:
        extras["owned_tx"] = [verified_owned_tx_result(output, "sqlite", name, profile.engine_version)]
    if savepoint_tx:
        extras["savepoint_tx"] = verified_savepoint_tx_results(output, "sqlite", name, profile.engine_version)
    if save_changes:
        extras["save_changes"] = verified_save_changes_results(output, "sqlite", name, profile.engine_version)
    if tx_options:
        extras["tx_options"] = [verified_tx_options_result(output, "sqlite", name, profile.engine_version)]
    if decimal_binding:
        extras["decimal_binding"] = [verified_decimal_binding_result(output, "sqlite", name, profile.engine_version)]
        extras["decimal_field_binding"] = [
            verified_decimal_field_binding_result(output, "sqlite", name, profile.engine_version),
        ]
    if commit_outcome:
        extras["commit_outcome"] = verified_commit_outcome_results(output, "sqlite", name, profile.engine_version)
    if production_adapters:
        extras["production_adapters"] = verified_production_adapter_results(output, "sqlite", name, profile.engine_version)
        extras["production_coverage"] = [{"profile": name, **production_coverage(coverage_path, "sqlite")}]
    return verified_driver_results(output, name, profile), extras


def run_exchange_stage(directory, binary, profile, journal, stage, token):
    output = command(
        [str(binary), "-test.v", "-test.run", "^TestSQLiteCompatibilityStep$", "-test.timeout", "30s"],
        cwd=directory, timeout=35,
        env={
            "GOEF_SQLITE_COMPAT_JOURNAL": journal,
            "GOEF_SQLITE_COMPAT_STAGE": stage,
            "GOEF_SQLITE_COMPAT_TOKEN": token,
        },
    )
    markers = [line for line in output.splitlines() if line.startswith(FILE_RESULT_PREFIX)]
    marker = f"{FILE_RESULT_PREFIX}{profile.engine_version}/{journal}/{stage}"
    native_code = STRICT_GENERATED_NATIVE_CODE if stage == "mutate" else None
    if native_code is not None:
        marker += f"/{native_code}"
    if markers != [marker]:
        raise LabError("The SQLite file test did not verify the requested stage, engine, and outcome.")
    return profile.engine_version, native_code


def allocate_file_case(tempdir, case):
    directory = tempdir / case
    directory.mkdir(mode=0o700)
    token = secrets.token_hex(32)
    with (directory / ".goef-sqlite-exchange").open("x", encoding="ascii") as marker:
        marker.write(token)
    return directory, token


def run_rebuild_stage(directory, binary, profile, journal, stage, token, *, executor=False):
    if executor:
        test, stage_key = "TestSQLiteExecutorRebuildStep", "GOEF_SQLITE_EXECUTOR_REBUILD_STAGE"
        prefix, exit_code = EXECUTOR_REBUILD_RESULT_PREFIX, EXECUTOR_REBUILD_INTERRUPTED_EXIT
    else:
        test, stage_key = "TestSQLiteRebuildStep", "GOEF_SQLITE_REBUILD_STAGE"
        prefix, exit_code = REBUILD_RESULT_PREFIX, REBUILD_INTERRUPTED_EXIT
    expected = f"{prefix}{profile.engine_version}/{journal}/{stage}"
    interrupted = stage == "interrupt"
    output = command(
        [str(binary), "-test.v", "-test.run", f"^{test}$", "-test.timeout", "30s"],
        cwd=directory, timeout=35,
        env={
            "GOEF_SQLITE_COMPAT_JOURNAL": journal,
            "GOEF_SQLITE_COMPAT_TOKEN": token,
            stage_key: stage,
        },
        expected_failure=(exit_code, expected) if interrupted else None,
    )
    if not interrupted:
        markers = [line for line in output.splitlines() if line.startswith(prefix)]
        if markers != [expected]:
            raise LabError("The SQLite rebuild did not report the requested stage and pinned engine.")


def run_rebuild_checks(tempdir, binaries, examples, report, *, executor=False):
    if executor:
        scenario, exit_code = EXECUTOR_REBUILD_SCENARIO, EXECUTOR_REBUILD_INTERRUPTED_EXIT
        result_key = "executor_rebuild_checks"
    else:
        scenario, exit_code = REBUILD_SCENARIO, REBUILD_INTERRUPTED_EXIT
        result_key = "rebuild_checks"
    for example in examples:
        name, journal = example.profile, example.journal
        profile = PROFILES[name]
        case = f"{scenario} {name} {journal}"
        report["phase"] = f"{case} setup"
        directory, token = allocate_file_case(tempdir, f"{scenario}-{name}-{journal}")
        for stage in example.stages:
            report["phase"] = f"{case} {stage}"
            run_rebuild_stage(directory, binaries[name], profile, journal, stage, token, executor=executor)
        report[result_key].append({
            "scenario": scenario, "profile": name, "engine_version": profile.engine_version,
            "journal_mode": journal, "interrupted_exit": exit_code, "status": "PASS",
        })


def run_lab(tempdir, report, *, model_types=False, owned_tx=False, savepoint_tx=False,
            save_changes=False, tx_options=False, decimal_binding=False, commit_outcome=False,
            production_adapters=False):
    report["phase"] = "SQLite file feature discovery"
    examples = parse_file_feature(FILE_FEATURE.read_text(encoding="utf-8"))
    report["phase"] = "SQLite rebuild feature discovery"
    rebuild_examples = parse_rebuild_feature(REBUILD_FEATURE.read_text(encoding="utf-8"))
    report["phase"] = "SQLite migration executor feature discovery"
    executor_examples = parse_executor_rebuild_feature(EXECUTOR_REBUILD_FEATURE.read_text(encoding="utf-8"))
    binaries = {}
    for name, profile in PROFILES.items():
        report["phase"] = f"build {name}"
        binaries[name], platform = build(tempdir, name, profile)
        driver_binary = binaries[name]
        if production_adapters:
            # The intentional process-exit workers require an exact stderr marker.
            # Run them with the ordinary binary, separate from coverage output.
            driver_binary, _ = build(tempdir, name, profile, coverage=True)
        report["phase"] = f"Gherkin {name}"
        results, extras = run_driver(
            tempdir, driver_binary, name, profile,
            model_types=model_types, owned_tx=owned_tx, savepoint_tx=savepoint_tx,
            save_changes=save_changes, tx_options=tx_options, decimal_binding=decimal_binding,
            commit_outcome=commit_outcome, production_adapters=production_adapters,
        )
        report["profiles"][name] = {
            "engine_version": profile.engine_version,
            "source_id": profile.source_id,
            "driver_version": profile.driver_version,
            "libc_version": profile.libc_version,
            "platform": platform,
            "facts": results[0]["facts"],
        }
        report["driver_scenarios"].extend(results)
        for key, rows in extras.items():
            report.setdefault(key, []).extend(rows)

    for example in examples:
        scenario, journal = example.scenario, example.journal
        report["phase"] = f"{scenario} {journal} setup"
        directory, token = allocate_file_case(tempdir, f"{scenario}-{journal}")
        engines = []
        strict_generated_native_code = None
        for stage, name in example.stages:
            report["phase"] = f"{scenario} {journal} {stage}"
            engine, native_code = run_exchange_stage(directory, binaries[name], PROFILES[name], journal, stage, token)
            engines.append(engine)
            if native_code is not None:
                strict_generated_native_code = native_code
        report["checks"].append({
            "scenario": scenario, "journal_mode": journal, "engines": engines,
            "strict_generated_native_code": strict_generated_native_code, "status": "PASS",
        })
    run_rebuild_checks(tempdir, binaries, rebuild_examples, report)
    run_rebuild_checks(tempdir, binaries, executor_examples, report, executor=True)


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--model-types", action="store_true", help="also run the model type round-trip scenario")
    parser.add_argument("--owned-tx", action="store_true", help="also run the owned transaction scenario")
    parser.add_argument("--savepoint-tx", action="store_true", help="also run savepoint and external transaction scenarios")
    parser.add_argument("--save-changes", action="store_true", help="also run SaveChanges transaction scenarios")
    parser.add_argument("--tx-options", action="store_true", help="also verify transaction isolation options")
    parser.add_argument("--decimal-binding", action="store_true", help="also verify exact decimal storage round trips")
    parser.add_argument("--commit-outcome", action="store_true", help="also verify synthetic commit outcome faults")
    parser.add_argument("--production-adapters", action="store_true", help="also run the production adapter application suite")
    args = parser.parse_args(argv)
    report = {
        "status": "FAIL", "go_version": GO_VERSION, "minimum_profile": MINIMUM_PROFILE, "profiles": {},
        "driver_scenarios": [], "checks": [], "rebuild_checks": [], "executor_rebuild_checks": [],
        "phase": "allocate temporary directory",
    }
    try:
        with TemporaryDirectory(prefix="goef-sqlite-compat-") as directory:
            run_lab(Path(directory), report, model_types=args.model_types, owned_tx=args.owned_tx,
                    savepoint_tx=args.savepoint_tx, save_changes=args.save_changes, tx_options=args.tx_options,
                    decimal_binding=args.decimal_binding, commit_outcome=args.commit_outcome,
                    production_adapters=args.production_adapters)
            report["phase"] = "cleanup"
    except Blocked as error:
        report.update(status="BLOCKED", reason=safe_reason(error))
    except (LabError, OSError, UnicodeError) as error:
        report["reason"] = safe_reason(error)
    else:
        report["status"] = "PASS"
        del report["phase"]
    print(json.dumps(report, sort_keys=True, allow_nan=False), flush=True)
    return {"PASS": 0, "FAIL": 1, "BLOCKED": 2}[report["status"]]


if __name__ == "__main__":
    raise SystemExit(main())
