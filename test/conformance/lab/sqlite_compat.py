"""Exercise the pinned SQLite runtimes and their common file schema (GoEF #31)."""

import argparse
from dataclasses import dataclass
import json
import os
from pathlib import Path
import re
import secrets
from tempfile import TemporaryDirectory

from driver_run import (
    ACQUIRE_CANCELLATION_SCENARIO, CANCELLATION_SCENARIO, LOCK_TIMEOUT_SCENARIO, MODULE,
    PRE_DISPATCH_SCENARIO, RESULT_PREFIX, SCENARIOS, driver_test_options,
    verified_acquire_cancellation_facts, verified_cancellation_facts, verified_lock_timeout_facts,
    verified_model_type_result, verified_owned_tx_result, verified_pre_dispatch_facts,
    verified_save_changes_results, verified_savepoint_tx_results, verified_tx_options_result,
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
GO_VERSION = "go1.27.4"


@dataclass(frozen=True)
class FileExample:
    scenario: str
    journal: str
    stages: tuple[tuple[str, str], ...]


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


def build(tempdir, name, profile):
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
    binary = tempdir / (name + (".exe" if os.name == "nt" else ""))
    command(
        ["go", "test", "-c", "-mod=readonly", *profile.build_flags, "-o", str(binary), "."],
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


def run_driver(tempdir, binary, name, profile, *, model_types, owned_tx, savepoint_tx, save_changes=False, tx_options=False):
    test_args, env = driver_test_options(
        model_types=model_types, owned_tx=owned_tx, savepoint_tx=savepoint_tx, save_changes=save_changes, tx_options=tx_options,
    )
    env.update(GOEF_DRIVERLAB_PROVIDER="sqlite", GOEF_DRIVERLAB_PROFILE=name)
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


def run_lab(tempdir, report, *, model_types=False, owned_tx=False, savepoint_tx=False, save_changes=False, tx_options=False):
    report["phase"] = "SQLite file feature discovery"
    examples = parse_file_feature(FILE_FEATURE.read_text(encoding="utf-8"))
    binaries = {}
    for name, profile in PROFILES.items():
        report["phase"] = f"build {name}"
        binaries[name], platform = build(tempdir, name, profile)
        report["phase"] = f"Gherkin {name}"
        results, extras = run_driver(
            tempdir, binaries[name], name, profile,
            model_types=model_types, owned_tx=owned_tx, savepoint_tx=savepoint_tx, save_changes=save_changes, tx_options=tx_options,
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
        directory = tempdir / f"{scenario}-{journal}"
        directory.mkdir(mode=0o700)
        token = secrets.token_hex(32)
        with (directory / ".goef-sqlite-exchange").open("x", encoding="ascii") as marker:
            marker.write(token)
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


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--model-types", action="store_true", help="also run the model type round-trip scenario")
    parser.add_argument("--owned-tx", action="store_true", help="also run the owned transaction scenario")
    parser.add_argument("--savepoint-tx", action="store_true", help="also run savepoint and external transaction scenarios")
    parser.add_argument("--save-changes", action="store_true", help="also run SaveChanges transaction scenarios")
    parser.add_argument("--tx-options", action="store_true", help="also verify transaction isolation options")
    args = parser.parse_args(argv)
    report = {
        "status": "FAIL", "go_version": GO_VERSION, "minimum_profile": MINIMUM_PROFILE, "profiles": {},
        "driver_scenarios": [], "checks": [], "phase": "allocate temporary directory",
    }
    try:
        with TemporaryDirectory(prefix="goef-sqlite-compat-") as directory:
            run_lab(Path(directory), report, model_types=args.model_types, owned_tx=args.owned_tx,
                    savepoint_tx=args.savepoint_tx, save_changes=args.save_changes, tx_options=args.tx_options)
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
