"""Validate the checked-in GoEF bootstrap sources and their cross-references."""

import argparse
from collections import Counter
from graphlib import CycleError, TopologicalSorter
import json
from pathlib import Path
import re
import subprocess
import sys
from urllib.parse import unquote, urlsplit

from jsonschema import FormatChecker
from jsonschema.validators import validator_for
import yaml


ROOT = Path(__file__).resolve().parents[2]
PROVIDERS = {"postgres", "sqlite", "sqlserver", "mysql"}
ISSUE_URL = "https://github.com/igorpooh1978/GoEF/issues/"
FEATURE_ID = r"[A-Z][A-Z0-9-]+-\d{3}"
MARKDOWN_LINK = re.compile(r"\[[^\]\n]+\]\(\s*(<[^>\n]+>|[^)\s]+)")
GH_ANCHOR = re.compile(r"[^\w -]", re.UNICODE)


def read(path):
    return (ROOT / path).read_text(encoding="utf-8-sig")


def unique_object(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError(f"duplicate JSON key {key}")
        result[key] = value
    return result


def read_json(path):
    return json.loads(read(path), object_pairs_hook=unique_object)


def repo_files():
    command = ["git", "ls-files", "--cached", "--others", "--exclude-standard", "-z"]
    output = subprocess.check_output(command, cwd=ROOT).decode("utf-8")
    return sorted({
        name for name in output.split("\0")
        if name and not name.startswith(".codegraph/") and (ROOT / name).is_file()
    })


def schema_check(path, schema_path, problems):
    document = read_json(path)
    schema = read_json(schema_path)
    validator_type = validator_for(schema)
    validator_type.check_schema(schema)
    validator = validator_type(schema, format_checker=FormatChecker())
    for error in validator.iter_errors(document):
        location = "/".join(map(str, error.absolute_path))
        problems.append(f"{path}:{location}: {error.message}")
    return document


def unique_records(records, key, label, problems):
    indexed = {}
    for record in records:
        value = record[key]
        if value in indexed:
            problems.append(f"{label}: duplicate {key} {value}")
        indexed[value] = record
    return indexed


def issue_link(value):
    suffix = str(value).removeprefix(ISSUE_URL)
    if suffix.isdecimal() and int(suffix) > 0:
        return ISSUE_URL + suffix
    raise ValueError(f"invalid issue reference {value!r}")


def scenario_index(files, problems):
    scenarios = {}
    feature_paths = {}
    for path in files:
        if not path.startswith("specs/features/") or not path.endswith(".feature"):
            continue
        feature_id = None
        tags = []
        for line in read(path).splitlines():
            text = line.strip()
            if text.startswith("@"):
                tags.extend(text.split())
                continue
            if not text or text.startswith("#"):
                continue
            if text.startswith("Feature:"):
                ids = [tag[1:] for tag in tags if re.fullmatch("@" + FEATURE_ID, tag)]
                if len(ids) != 1 or feature_id is not None:
                    problems.append(f"{path}: a single Feature with one stable ID is required")
                else:
                    feature_id = ids[0]
                    if feature_id in feature_paths:
                        problems.append(f"{path}: repeated feature {feature_id}")
                    feature_paths[feature_id] = path
            elif re.match(r"(?:Scenario(?: Outline| Template)?|Example):", text):
                ids = [tag[1:] for tag in tags if re.fullmatch("@" + FEATURE_ID + r"-S\d{2,}", tag)]
                if feature_id is None or len(ids) != 1 or not ids[0].startswith(feature_id + "-S"):
                    problems.append(f"{path}: scenario missing a unique ID for its Feature")
                else:
                    scenario_id = ids[0]
                    if scenario_id in scenarios:
                        problems.append(f"{path}: repeated scenario {scenario_id}")
                    scenarios[scenario_id] = (feature_id, path)
            tags.clear()
        if feature_id is None:
            problems.append(f"{path}: no named Feature was found")
    return scenarios


def check_local_path(target, source, problems):
    value = urlsplit(target)
    if value.scheme or value.netloc or not (value.path or value.fragment):
        return
    root = ROOT.resolve()
    if value.path:
        resolved = (ROOT / source).parent.joinpath(unquote(value.path)).resolve()
    else:
        resolved = (ROOT / source).resolve()
    if not resolved.is_relative_to(root) or not resolved.exists():
        problems.append(f"{source}: missing or out-of-repository link {target}")
        return
    if value.fragment and resolved.suffix.lower() == ".md":
        fragment = unquote(value.fragment).lower()
        headings = re.findall(r"^#{1,6}\s+(.+?)\s*#*\s*$", resolved.read_text(encoding="utf-8-sig"), re.M)
        counts = Counter()
        anchors = set()
        for heading in headings:
            slug = GH_ANCHOR.sub("", heading.lower()).replace(" ", "-")
            count = counts[slug]
            anchors.add(f"{slug}-{count}" if count else slug)
            counts[slug] += 1
        if fragment not in anchors:
            problems.append(f"{source}: missing Markdown heading {target}")


def check_markdown(files, problems):
    for path in files:
        if not path.endswith(".md"):
            continue
        fenced = False
        for line in read(path).splitlines():
            if line.lstrip().startswith(("\x60\x60\x60", "~~~")):
                fenced = not fenced
                continue
            if fenced:
                continue
            line = re.sub(r"(\x60+)[^\n]*?\1", "", line)
            for match in MARKDOWN_LINK.finditer(line):
                check_local_path(match[1].strip("<>"), path, problems)


def check_issue_forms(problems):
    root = ".github/ISSUE_TEMPLATE/"
    for name in ("feature", "bug", "research", "provider", "documentation"):
        path = root + name + ".yml"
        form = yaml.safe_load(read(path))
        if not form.get("name") or not form.get("description") or not form.get("body"):
            problems.append(f"{path}: a named form needs a description and questions")
            continue
        ids = [field.get("id") for field in form["body"] if field.get("type") != "markdown"]
        if any(not item for item in ids) or len(ids) != len(set(ids)):
            problems.append(f"{path}: missing or repeated issue-form question IDs")
    config = yaml.safe_load(read(root + "config.yml"))
    if not isinstance(config.get("blank_issues_enabled"), bool):
        problems.append("issue chooser: blank_issues_enabled must be Boolean")


def check_proof(record, label, expected_sha, problems):
    if record.get("status") not in {"PASS", "FAIL", "VERIFIED", "EXECUTABLE", "VERIFIED_UNSUPPORTED"}:
        return
    evidence = record.get("evidence") or record.get("evidenceSha")
    tests = record.get("executableTests") if "executableTests" in record else record.get("test") or record.get("tests")
    if not evidence or not tests:
        problems.append(f"{label}: claimed acceptance lacks test or evidence")
    else:
        if isinstance(evidence, dict):
            evidence = evidence.get("sha") or evidence.get("commit") or evidence.get("sourceSha")
        if evidence != expected_sha:
            problems.append(f"{label}: proof does not identify validation revision {expected_sha}")
    problems.append(f"{label}: runtime acceptance requires the W002 execution harness")


def operator_summary(registry):
    operators = registry["operators"]
    lines = [
        "| Mode | Source member groups | Signatures |",
        "| --- | ---: | ---: |",
    ]
    for mode in ("objects", "expressions", "queryable"):
        mode_operators = [operator for operator in operators if operator["mode"] == mode]
        signatures = sum(len(operator["overloads"]) for operator in mode_operators)
        lines.append(f"| {mode.title()} | {len(mode_operators)} | {signatures} |")
    lines.extend([
        "",
        "| Queryable provider | Planned groups | Inapplicable groups |",
        "| --- | ---: | ---: |",
    ])
    queryable = [operator for operator in operators if operator["mode"] == "queryable"]
    for provider in sorted(PROVIDERS):
        planned = sum(operator["providerCells"][provider] == "planned" for operator in queryable)
        lines.append(f"| {provider} | {planned} | {len(queryable) - planned} |")
    return "\n".join(lines)


def check_operator_references(registry, locks, problems):
    lock_ids = {
        "runtime": "dotnet-runtime",
        "efcore": "efcore",
        "go-linq-v5": "go-linq-stable",
        "go-linq-research": "go-linq-research",
    }
    if set(registry["sources"]) != set(lock_ids):
        problems.append("LINQ registry: listing does not match the four baseline sources")
    for source_id, lock_id in lock_ids.items():
        item = registry["sources"].get(source_id)
        locked = locks[lock_id]
        if not item or item["sha"] != locked["sha"] or item["repository"] != locked["repository"] \
                or item["ref"] != (locked["tag"] or "research snapshot"):
            problems.append(f"LINQ registry: {source_id} disagrees with the source lock")

    for operator in registry["operators"]:
        label = operator["id"]
        if operator["source"] not in registry["sources"]:
            problems.append(f"{label}: missing source")
        if operator["semanticProfile"] not in registry["semanticProfiles"]:
            problems.append(f"{label}: missing semantic profile")
        for provider, policy in operator["providerCells"].items():
            if policy not in registry["providerPolicies"]:
                problems.append(f"{label}: unknown {provider} policy {policy}")

    heading = "## Registry counts"
    sections = read("docs/reference/LINQ_OPERATORS.md").split(heading, 1)
    if len(sections) != 2 or sections[1].strip() != operator_summary(registry):
        problems.append("LINQ operator reference counts are out of date with the canonical registry")


def verify(args):
    revision = args.evidence_sha or "HEAD"
    if revision != "HEAD" and not re.fullmatch(r"[0-9a-f]{40}", revision):
        raise ValueError("--evidence-sha must be a full commit SHA")
    expected_sha = subprocess.check_output(
        ["git", "rev-parse", "--verify", revision + "^{commit}"], cwd=ROOT, text=True
    ).strip()
    files = repo_files()
    problems = []
    for path in files:
        if path.endswith(".json"):
            read_json(path)
    check_issue_forms(problems)
    schema_check("docs/upstream/mcp-sources.json", "docs/upstream/mcp-sources.schema.json", problems)

    features = schema_check("docs/parity/features.json", "docs/parity/feature.schema.json", problems)
    feature_index = unique_records(features["features"], "id", "feature registry", problems)
    operators = schema_check("docs/parity/linq-operators.json", "docs/parity/linq-operators.schema.json", problems)
    operator_index = unique_records(operators["operators"], "id", "operator registry", problems)
    errors = schema_check("docs/errors/registry.json", "docs/errors/registry.schema.json", problems)
    error_index = unique_records(errors["errors"], "id", "error registry", problems)
    error_rows = re.findall(r"^\| GOEF-.*$", read("docs/reference/ERROR_CODES.md"), re.M)
    expected_errors = [
        "| {id} | {sentinel} | {category} | {retryClass} |".format(**error_index[identifier])
        for identifier in sorted(error_index)
    ]
    if error_rows != expected_errors:
        problems.append("error reference rows are out of date with the canonical registry")
    scenarios = scenario_index(files, problems)
    locks = unique_records(read_json("docs/upstream/lock.json")["sources"], "id", "upstream locks", problems)
    for source_id, locked in locks.items():
        if not re.fullmatch(r"[a-f0-9]{40}", locked["sha"]):
            problems.append(f"{source_id}: the source lock needs a full Git commit ID")

    try:
        tuple(TopologicalSorter({key: value["dependencies"] for key, value in feature_index.items()}).static_order())
    except CycleError as error:
        problems.append(f"feature dependencies contain a cycle: {error}")
    check_operator_references(operators, locks, problems)

    for feature_id, feature in feature_index.items():
        if set(feature["providers"]) != PROVIDERS:
            problems.append(f"{feature_id}: required provider cells missing or unexpected")
        for dependency in feature["dependencies"]:
            if dependency not in feature_index:
                problems.append(f"{feature_id}: missing dependency {dependency}")
        for upstream in feature["upstream"]:
            locked = locks.get(upstream["lockId"])
            if not locked or any(upstream[key] != locked[key] for key in ("sha", "tag", "repository")):
                problems.append(f"{feature_id}: upstream reference disagrees with source lock {upstream['lockId']}")
        for scenario in feature["scenarios"]:
            if scenario not in scenarios or scenarios[scenario][0] != feature_id:
                problems.append(f"{feature_id}: missing or misassigned scenario {scenario}")
        for doc in feature["docs"]:
            check_local_path(doc, "README.md", problems)
        for provider, cell in feature["providers"].items():
            check_proof(cell, f"{feature_id}/{provider}", expected_sha, problems)
        check_proof(feature, feature_id, expected_sha, problems)

    def check_scenarios(ids, label):
        for scenario in ids:
            if scenario not in scenarios:
                problems.append(f"{label}: unknown scenario {scenario}")

    unique_records(
        (overload for operator in operator_index.values() for overload in operator["overloads"]),
        "id", "LINQ overloads", problems,
    )
    for operator_id, operator in operator_index.items():
        if operator.get("featureId") not in feature_index:
            problems.append(f"{operator_id}: unknown parent feature {operator.get('featureId')}")
        if set(operator.get("providerCells", {})) != PROVIDERS:
            problems.append(f"{operator_id}: missing or unexpected provider cells")
        check_scenarios(operator.get("scenarioIds", []), operator_id)
        check_proof(operator, operator_id, expected_sha, problems)

    mapping_total = 0
    for provider in sorted(PROVIDERS):
        mappings = schema_check(
            f"docs/errors/provider-mappings/{provider}.json", "docs/errors/registry.schema.json", problems
        )
        if mappings["provider"] != provider:
            problems.append(f"error mappings: {provider} filename and provider disagree")
        indexed = unique_records(mappings["mappings"], "id", provider, problems)
        mapping_total += len(indexed)
        for mapping_id, mapping in indexed.items():
            error = error_index.get(mapping["errorId"])
            if error is None or mapping_id not in error["providerMappings"][provider]:
                problems.append(f"{mapping_id}: missing reverse reference to {mapping['errorId']}")
            elif mapping["expectedGoError"] != error["sentinel"]:
                problems.append(f"{mapping_id}: mapped sentinel disagrees with error registry")
            check_proof(mapping, mapping_id, expected_sha, problems)
            if mapping["reproduction"]["status"] == "OBSERVED":
                problems.append(f"{mapping_id}: native reproduction requires a real-engine harness")
        for error_id, error in error_index.items():
            for mapping_id in error["providerMappings"][provider]:
                if mapping_id not in indexed or indexed[mapping_id]["errorId"] != error_id:
                    problems.append(f"{error_id}: missing or misassigned mapping {mapping_id}")

    for error_id, error in error_index.items():
        check_proof(error, error_id, expected_sha, problems)
        for feature_id in error["featureIds"]:
            if feature_id not in feature_index:
                problems.append(f"{error_id}: missing feature {feature_id}")
        check_scenarios(error["scenarioIds"], error_id)
        if error["replacement"] and error["replacement"] not in error_index:
            problems.append(f"{error_id}: replacement does not exist")
        for doc in error["docs"]:
            check_local_path(doc, "README.md", problems)

    expected_rows = []
    for scenario_id, (feature_id, path) in sorted(scenarios.items()):
        if feature_id not in feature_index:
            problems.append(f"{scenario_id}: missing feature registry record {feature_id}")
            continue
        feature = feature_index[feature_id]
        expected_rows.append({
            "featureId": feature_id,
            "scenarioId": scenario_id,
            "spec": path,
            "issues": sorted(set(map(issue_link, feature["issues"]))),
            "docs": sorted(set(doc for doc in feature["docs"] if str(doc).endswith(".md"))) or ["specs/README.md"],
            "test": None,
            "evidence": None,
            "status": "SPECIFIED",
        })
    trace_path = "specs/traceability.json"
    if args.write_bootstrap_traceability and not problems:
        if (ROOT / trace_path).is_file():
            existing = read_json(trace_path)["scenarios"]
            if any(row["status"] != "SPECIFIED" or row["test"] or row["evidence"] for row in existing):
                raise ValueError("refresh would remove existing scenario execution evidence")
        (ROOT / trace_path).write_text(
            json.dumps({"schemaVersion": 1, "scenarios": expected_rows}, indent=2) + "\n",
            encoding="utf-8", newline="\n",
        )
    if (ROOT / trace_path).is_file():
        trace = schema_check(trace_path, "specs/traceability.schema.json", problems)
        trace_index = unique_records(trace["scenarios"], "scenarioId", "traceability", problems)
        for row in expected_rows:
            scenario_id = row["scenarioId"]
            trace_row = trace_index.get(scenario_id)
            if trace_row is None:
                problems.append(f"{scenario_id}: missing traceability")
                continue
            for key in ("featureId", "spec", "issues", "docs"):
                if trace_row[key] != row[key]:
                    problems.append(f"{scenario_id}: traceability {key} disagrees with artifacts")
            for doc in trace_row["docs"]:
                check_local_path(doc, "README.md", problems)
            check_proof(trace_row, scenario_id, expected_sha, problems)
        for orphan in sorted(set(trace_index) - scenarios.keys()):
            problems.append(f"{orphan}: traceability has no Gherkin scenario")
    else:
        problems.append(f"{trace_path}: missing; generate after fixing other findings")

    check_markdown(files, problems)
    for problem in problems:
        print(problem, file=sys.stderr)
    if problems:
        print(f"Bootstrap validation failed with {len(problems)} finding(s).", file=sys.stderr)
        return 1

    modes = Counter(record["mode"] for record in operator_index.values())
    print(f"Validated {len(feature_index)} features, {len(operator_index)} LINQ records "
          f"({dict(sorted(modes.items()))}), {len(error_index)} error IDs, {mapping_total} native rules, "
          f"and {len(scenarios)} Gherkin scenarios.")
    return 0


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--write-bootstrap-traceability", action="store_true")
    parser.add_argument("--evidence-sha", help="expected evidence commit (defaults to HEAD)")
    try:
        return verify(parser.parse_args())
    except (KeyError, OSError, TypeError, ValueError, subprocess.CalledProcessError, yaml.YAMLError) as error:
        print(f"Bootstrap validation could not complete: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
