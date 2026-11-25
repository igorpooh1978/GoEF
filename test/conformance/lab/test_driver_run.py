"""Negative controls for the local driver matrix evidence gate."""

from contextlib import redirect_stdout
import io
import json
import unittest
from unittest.mock import patch

import driver_run


class DriverEvidenceTests(unittest.TestCase):
    def result_lines(self):
        return [
            {
                "provider": "sqlite",
                "scenario": f"LAB-DRIVER-001-S0{index}",
                "status": "PASS",
                "facts": {"engine_version": "3.54.1", "native_code": code, "ignored_secret": "fixture", "target_marker": "fixture"},
            }
            for index, code in enumerate(("", "2067", "787"), start=1)
        ]

    def output(self, rows):
        return "\n".join(driver_run.RESULT_PREFIX + json.dumps(row) for row in rows)

    def test_complete_evidence_is_required_and_redacted(self):
        rows = self.result_lines()
        result = driver_run.verified_results(self.output(rows), "sqlite")
        self.assertEqual(len(result), 3)
        self.assertNotIn("ignored_secret", result[0]["facts"])
        self.assertNotIn("target_marker", result[0]["facts"])
        with self.assertRaises(driver_run.LabError):
            driver_run.verified_results(self.output(rows[:-1]), "sqlite")
        with self.assertRaises(driver_run.LabError):
            driver_run.verified_results(self.output([*rows, rows[0]]), "sqlite")

    def test_native_code_version_and_provider_are_checked(self):
        for field, value in (("native_code", "0"), ("engine_version", "3.53.0")):
            with self.subTest(field=field):
                rows = self.result_lines()
                rows[1]["facts"][field] = value
                with self.assertRaises(driver_run.LabError):
                    driver_run.verified_results(self.output(rows), "sqlite")
        rows = self.result_lines()
        rows[2]["provider"] = "postgres"
        with self.assertRaises(driver_run.LabError):
            driver_run.verified_results(self.output(rows), "sqlite")

    def test_sqlserver_opt_in_is_required_before_build_or_startup(self):
        stdout = io.StringIO()
        with patch.object(driver_run, "build_binary") as build, redirect_stdout(stdout):
            self.assertEqual(driver_run.main(["sqlserver"]), 1)
        build.assert_not_called()
        result = json.loads(stdout.getvalue())
        self.assertEqual(result["provider"], "sqlserver")
        self.assertEqual(result["status"], "BLOCKED")

    def test_explicit_postgres_profile_matches_its_own_identity(self):
        rows = self.result_lines()
        for row, code in zip(rows, ("", "23505", "23503"), strict=True):
            row["provider"] = "postgres"
            row["facts"] = {"engine_version": "16.16", "native_code": code}
        output = self.output(rows)
        self.assertEqual(len(driver_run.verified_results(output, "postgres", "postgres-16")), 3)
        with self.assertRaises(driver_run.LabError):
            driver_run.verified_results(output, "postgres")

    def test_legacy_mysql_does_not_use_newer_identity_or_client_options(self):
        legacy = driver_run.PROFILES["mysql-8.0"]
        current = driver_run.PROFILES["mysql-8.4"]
        self.assertEqual(legacy.probe, "VERSION()")
        self.assertEqual(current.probe, "VERSION_WITH_UPDATE()")
        self.assertNotIn("--no-login-paths", legacy.client_flags)
        self.assertIn("--no-login-paths", current.client_flags)
