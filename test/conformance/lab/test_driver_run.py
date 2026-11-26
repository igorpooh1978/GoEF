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

    def test_sqlserver_current_and_legacy_eula_opt_ins_are_independent(self):
        cases = (
            ("sqlserver-2017", "--accept-sqlserver-eula", "--accept-legacy-sqlserver-eula"),
            ("sqlserver-2019", "--accept-sqlserver-eula", "--accept-legacy-sqlserver-eula"),
            ("sqlserver-2022", "--accept-legacy-sqlserver-eula", "--accept-sqlserver-eula"),
            ("sqlserver-2022-140", "--accept-legacy-sqlserver-eula", "--accept-sqlserver-eula"),
        )
        for profile, supplied, required in cases:
            with self.subTest(profile=profile):
                stdout = io.StringIO()
                with (
                    patch.object(driver_run, "build_binary") as build,
                    patch.object(driver_run.Lab, "container") as container,
                    redirect_stdout(stdout),
                ):
                    self.assertEqual(driver_run.main(["sqlserver", "--profile", profile, supplied]), 1)
                build.assert_not_called()
                container.assert_not_called()
                result = json.loads(stdout.getvalue())
                self.assertEqual(result["status"], "BLOCKED")
                self.assertIn(required, result["reason"])

    def test_sqlserver_profile_requires_exact_build_and_database_compatibility(self):
        rows = self.result_lines()
        for row, code in zip(rows, ("", "2627", "547"), strict=True):
            row["provider"] = "sqlserver"
            row["facts"] = {
                "version": "14.0.3570.2", "product_update_level": "CU31",
                "compatibility_level": "140", "native_code": code,
            }
        output = self.output(rows)
        self.assertEqual(len(driver_run.verified_results(output, "sqlserver", "sqlserver-2017")), 3)
        with self.assertRaises(driver_run.LabError):
            driver_run.verified_results(output, "sqlserver", "sqlserver-2019")
        for field, value in (("compatibility_level", "150"), ("product_update_level", "CU32")):
            with self.subTest(field=field):
                wrong = rows.copy()
                wrong[0] = {**rows[0], "facts": {**rows[0]["facts"], field: value}}
                with self.assertRaises(driver_run.LabError):
                    driver_run.verified_results(self.output(wrong), "sqlserver", "sqlserver-2017")

    def test_sqlserver_2022_evidence_requires_matching_level_and_generate_series_probe(self):
        cases = (
            ("sqlserver-2022", "160", "available", ""),
            ("sqlserver-2022-140", "140", "unavailable", "208"),
        )
        for profile, level, availability, probe_code in cases:
            with self.subTest(profile=profile):
                rows = self.result_lines()
                for row, code in zip(rows, ("", "2627", "547"), strict=True):
                    row["provider"] = "sqlserver"
                    row["facts"] = {
                        "version": "16.0.4325.4", "product_update_level": "CU28",
                        "compatibility_level": level, "native_code": code,
                        "generate_series": availability,
                    }
                    if probe_code:
                        row["facts"]["generate_series_native_code"] = probe_code
                result = driver_run.verified_results(self.output(rows), "sqlserver", profile)
                self.assertEqual(len(result), 3)
                self.assertEqual([row["facts"]["generate_series"] for row in result], [availability] * 3)
                other = "sqlserver-2022-140" if profile == "sqlserver-2022" else "sqlserver-2022"
                with self.assertRaises(driver_run.LabError):
                    driver_run.verified_results(self.output(rows), "sqlserver", other)
                if probe_code:
                    self.assertEqual([row["facts"]["generate_series_native_code"] for row in result], [probe_code] * 3)
                    rows[1]["facts"]["generate_series_native_code"] = "229"
                    with self.assertRaises(driver_run.LabError):
                        driver_run.verified_results(self.output(rows), "sqlserver", profile)
                    rows[1]["facts"]["generate_series_native_code"] = probe_code
                rows[1]["facts"].pop("generate_series")
                with self.assertRaises(driver_run.LabError):
                    driver_run.verified_results(self.output(rows), "sqlserver", profile)

    def test_explicit_postgres_profile_matches_its_own_identity(self):
        profiles = (("postgres-16", "16.16"), ("postgres-17", "17.12"), ("postgres-19", "19.0"))
        for profile, version in profiles:
            with self.subTest(profile=profile):
                rows = self.result_lines()
                for row, code in zip(rows, ("", "23505", "23503"), strict=True):
                    row["provider"] = "postgres"
                    row["facts"] = {"engine_version": version, "native_code": code}
                output = self.output(rows)
                self.assertEqual(len(driver_run.verified_results(output, "postgres", profile)), 3)
                with self.assertRaises(driver_run.LabError):
                    driver_run.verified_results(output, "postgres")

    def test_legacy_mysql_does_not_use_newer_identity_or_client_options(self):
        legacy = driver_run.PROFILES["mysql-8.0"]
        current = driver_run.PROFILES["mysql-8.4"]
        self.assertEqual(legacy.probe, "VERSION()")
        self.assertEqual(current.probe, "VERSION_WITH_UPDATE()")
        self.assertNotIn("--no-login-paths", legacy.client_flags)
        self.assertIn("--no-login-paths", current.client_flags)
