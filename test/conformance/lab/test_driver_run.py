"""Negative controls for the local driver matrix evidence gate."""

from contextlib import nullcontext, redirect_stdout
import io
import json
import os
import unittest
from unittest.mock import Mock, patch

import driver_run


class DriverEvidenceTests(unittest.TestCase):
    def result_lines(self, provider="sqlite", native_codes=("2067", "787"), **profile_facts):
        if not profile_facts:
            profile_facts = {"engine_version": "3.54.1"}
        outcomes = (
            {}, {"native_code": native_codes[0]}, {"native_code": native_codes[1]},
            {"cancel_cause": "context.Canceled", "pool_in_use_after": "0"},
        )
        return [
            {
                "provider": provider,
                "scenario": f"LAB-DRIVER-001-S0{index}",
                "status": "PASS",
                "facts": {**profile_facts, **outcome, "ignored_secret": "fixture", "target_marker": "fixture"},
            }
            for index, outcome in enumerate(outcomes, start=1)
        ]

    def output(self, rows):
        return "\n".join(driver_run.RESULT_PREFIX + json.dumps(row) for row in rows)

    def test_complete_evidence_is_required_and_redacted(self):
        rows = self.result_lines()
        result = driver_run.verified_results(self.output(rows), "sqlite")
        self.assertEqual(len(result), 4)
        self.assertNotIn("ignored_secret", result[0]["facts"])
        self.assertNotIn("target_marker", result[0]["facts"])
        self.assertEqual(result[3]["facts"], {
            "engine_version": "3.54.1", "cancel_cause": "context.Canceled", "pool_in_use_after": "0",
        })
        with self.assertRaises(driver_run.LabError):
            driver_run.verified_results(self.output(rows[:-1]), "sqlite")
        with self.assertRaises(driver_run.LabError):
            driver_run.verified_results(self.output([*rows, rows[3]]), "sqlite")

    def test_cancellation_requires_the_cause_and_released_pool(self):
        for field, wrong in (
            ("cancel_cause", None), ("cancel_cause", "context.DeadlineExceeded"),
            ("pool_in_use_after", None), ("pool_in_use_after", "1"), ("pool_in_use_after", 0),
            ("native_code", "2067"),
        ):
            with self.subTest(field=field, wrong=wrong):
                rows = self.result_lines()
                if wrong is None:
                    rows[3]["facts"].pop(field)
                else:
                    rows[3]["facts"][field] = wrong
                with self.assertRaisesRegex(driver_run.LabError, "cancellation"):
                    driver_run.verified_results(self.output(rows), "sqlite")

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
        rows = self.result_lines(
            "sqlserver", ("2627", "547"), version="14.0.3570.2",
            product_update_level="CU31", compatibility_level="140",
        )
        output = self.output(rows)
        self.assertEqual(len(driver_run.verified_results(output, "sqlserver", "sqlserver-2017")), 4)
        with self.assertRaises(driver_run.LabError):
            driver_run.verified_results(output, "sqlserver", "sqlserver-2019")
        for field, value in (("compatibility_level", "150"), ("product_update_level", "CU32")):
            with self.subTest(field=field):
                wrong = rows.copy()
                wrong[0] = {**rows[0], "facts": {**rows[0]["facts"], field: value}}
                with self.assertRaises(driver_run.LabError):
                    driver_run.verified_results(self.output(wrong), "sqlserver", "sqlserver-2017")

    def test_negative_certificate_serial_compatibility_is_scoped_to_sqlserver_2017_binary(self):
        accepted = {driver_run.SQLSERVER_EULA, driver_run.LEGACY_SQLSERVER_EULA}
        cases = ("sqlserver-2017", "sqlserver-2019", "sqlserver-2022", "sqlserver-2022-140", "postgres-18", "mysql-8.4")
        for profile_name in cases:
            with self.subTest(profile=profile_name):
                provider = driver_run.PROFILES[profile_name].provider
                container = Mock()
                container.exec.return_value = ""
                provision = Mock(return_value=nullcontext(container))
                with (
                    patch.dict(os.environ, {"GODEBUG": "host-placeholder"}),
                    patch.dict(driver_run.SERVER_CONTAINERS, {provider: provision}),
                    patch.object(driver_run, "verified_results", return_value=[]),
                ):
                    result = driver_run.run_provider(profile_name, "driverlab-binary", accepted)
                    self.assertEqual(os.environ["GODEBUG"], "host-placeholder")
                self.assertEqual(result["status"], "PASS")
                container.exec.assert_called_once()
                binary_env = container.exec.call_args.kwargs["env"]
                if profile_name == "sqlserver-2017":
                    self.assertEqual(binary_env["GODEBUG"], "x509negativeserial=1")
                else:
                    self.assertNotIn("GODEBUG", binary_env)

    def test_sqlserver_2022_evidence_requires_matching_level_and_generate_series_probe(self):
        cases = (
            ("sqlserver-2022", "160", "available", ""),
            ("sqlserver-2022-140", "140", "unavailable", "208"),
        )
        for profile, level, availability, probe_code in cases:
            with self.subTest(profile=profile):
                rows = self.result_lines(
                    "sqlserver", ("2627", "547"), version="16.0.4325.4", product_update_level="CU28",
                    compatibility_level=level, generate_series=availability,
                )
                for row in rows:
                    if probe_code:
                        row["facts"]["generate_series_native_code"] = probe_code
                result = driver_run.verified_results(self.output(rows), "sqlserver", profile)
                self.assertEqual(len(result), 4)
                self.assertEqual([row["facts"]["generate_series"] for row in result], [availability] * 4)
                other = "sqlserver-2022-140" if profile == "sqlserver-2022" else "sqlserver-2022"
                with self.assertRaises(driver_run.LabError):
                    driver_run.verified_results(self.output(rows), "sqlserver", other)
                if probe_code:
                    self.assertEqual([row["facts"]["generate_series_native_code"] for row in result], [probe_code] * 4)
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
                rows = self.result_lines("postgres", ("23505", "23503"), engine_version=version)
                output = self.output(rows)
                self.assertEqual(len(driver_run.verified_results(output, "postgres", profile)), 4)
                with self.assertRaises(driver_run.LabError):
                    driver_run.verified_results(output, "postgres")

    def test_legacy_mysql_does_not_use_newer_identity_or_client_options(self):
        legacy = driver_run.PROFILES["mysql-8.0"]
        current = driver_run.PROFILES["mysql-8.4"]
        self.assertEqual(legacy.probe, "VERSION()")
        self.assertEqual(current.probe, "VERSION_WITH_UPDATE()")
        self.assertNotIn("--no-login-paths", legacy.client_flags)
        self.assertIn("--no-login-paths", current.client_flags)
