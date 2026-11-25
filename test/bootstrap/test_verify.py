"""Negative controls for the W000 validator on the integrated fixtures."""

import argparse
from contextlib import redirect_stderr, redirect_stdout
import io
import json
from unittest import TestCase, mock

import verify as bootstrap


class RejectionTests(TestCase):
    def check_rejection(self, path, mutate, diagnostic):
        original = bootstrap.read
        replacement = mutate(original(path))

        def read_fixture(name):
            return replacement if name == path else original(name)

        stderr = io.StringIO()
        args = argparse.Namespace(write_bootstrap_traceability=False, evidence_sha=None)
        with mock.patch.object(bootstrap, "read", read_fixture), redirect_stderr(stderr), redirect_stdout(io.StringIO()):
            self.assertEqual(bootstrap.verify(args), 1)
        self.assertIn(diagnostic, stderr.getvalue())

    def test_missing_relative_link(self):
        self.check_rejection(
            "README.md",
            lambda text: text + "\n[Broken](docs/does-not-exist.md)\n",
            "missing or out-of-repository link docs/does-not-exist.md",
        )

    def test_unregistered_scenario(self):
        def mutate(text):
            value = json.loads(text)
            value["scenarios"][0]["scenarioId"] = "ABSENT-FEATURE-001-S99"
            return json.dumps(value)

        self.check_rejection("specs/traceability.json", mutate, "traceability has no Gherkin scenario")

    def test_fabricated_pass_at_current_revision(self):
        def mutate(text):
            value = json.loads(text)
            current = bootstrap.subprocess.check_output(
                ["git", "rev-parse", "HEAD"], cwd=bootstrap.ROOT, text=True,
            ).strip()
            value["scenarios"][0].update(status="PASS", test="test/imaginary.go:TestMissing", evidence=current)
            return json.dumps(value)

        self.check_rejection("specs/traceability.json", mutate, "runtime acceptance requires the W002 execution harness")

    def test_stale_evidence(self):
        def mutate(text):
            value = json.loads(text)
            value["scenarios"][0].update(status="PASS", test="test/imaginary.go:TestMissing", evidence="0" * 40)
            return json.dumps(value)

        self.check_rejection("specs/traceability.json", mutate, "proof does not identify validation revision")

    def test_native_promotion_without_executable_reproduction(self):
        def mutate(text):
            value = json.loads(text)
            value["mappings"][0].update(status="VERIFIED", driverVersion="0.0.0")
            reproduction = value["mappings"][0]["reproduction"]
            reproduction.update(status="OBSERVED", observed="synthetic assertion")
            return json.dumps(value)

        self.check_rejection(
            "docs/errors/provider-mappings/mysql.json", mutate, "native reproduction requires a real-engine harness",
        )

    def test_operator_source_and_policies(self):
        def mutate(text):
            value = json.loads(text)
            value["sources"]["runtime"]["sha"] = "0" * 40
            value["operators"][0].update(source="missing", semanticProfile="missing")
            value["operators"][0]["providerCells"]["sqlite"] = "missing"
            return json.dumps(value)

        self.check_rejection(
            "docs/parity/linq-operators.json", mutate, "runtime disagrees with the source lock",
        )
        self.check_rejection("docs/parity/linq-operators.json", mutate, "missing semantic profile")
        self.check_rejection("docs/parity/linq-operators.json", mutate, "unknown sqlite policy missing")

    def test_stale_operator_reference(self):
        self.check_rejection(
            "docs/reference/LINQ_OPERATORS.md",
            lambda _: "# Stale reference\n",
            "LINQ operator reference counts are out of date",
        )
