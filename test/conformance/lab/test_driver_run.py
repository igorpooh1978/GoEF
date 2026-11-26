"""Negative controls for the local driver matrix evidence gate."""

from contextlib import nullcontext, redirect_stdout
import io
import json
import os
import subprocess
import unittest
from unittest.mock import MagicMock, Mock, patch

import driver_run
import sqlite_compat


class SQLServerProvisioningDiagnosticsTests(unittest.TestCase):
    def mock_lab(self):
        container = Mock()
        container.exec.return_value = "16.0.4325.4|CU28|Developer Edition (64-bit)"
        lab = Mock(allow_sqlserver=True)
        lab.container.return_value = nullcontext(container)
        return lab, container

    def timeout(self):
        # Model a real subprocess timeout that carries private argv and output.
        with patch("runtime.subprocess.run", side_effect=subprocess.TimeoutExpired(
            ["sqlcmd", "private-password"], 15, stderr="private-SQL",
        )):
            driver_run.command(["sqlcmd"], cwd=".")

    def test_reports_each_provisioning_stage_for_a_timeout(self):
        for stage in ("docker-startup", "engine-readiness", "engine-identity", "fixture-setup"):
            with self.subTest(stage=stage):
                lab, container = self.mock_lab()
                if stage == "docker-startup":
                    lab.container.return_value = MagicMock()
                    lab.container.return_value.__enter__.side_effect = self.timeout
                elif stage == "engine-readiness":
                    container.wait.side_effect = lambda *_args, **_kwargs: self.timeout()
                elif stage == "engine-identity":
                    container.exec.side_effect = lambda *_args, **_kwargs: self.timeout()
                else:
                    def execute(argv, **_kwargs):
                        if argv[-1] == driver_run.sqlserver.SERVER_IDENTITY:
                            return container.exec.return_value
                        self.timeout()

                    container.exec.side_effect = execute
                with patch.object(driver_run, "Lab", return_value=lab):
                    result = driver_run.run_provider(
                        "sqlserver-2022", "driverlab-binary", {driver_run.SQLSERVER_EULA},
                    )
                self.assertEqual(result, {
                    "provider": "sqlserver", "profile": "sqlserver-2022", "status": "FAIL",
                    "phase": "provisioning", "stage": stage, "reason": "Command timed out.",
                })
                container.docker.assert_not_called()

    def test_identity_mismatch_keeps_its_safe_reason(self):
        lab, container = self.mock_lab()
        container.exec.return_value = "unexpected-private-build"
        with patch.object(driver_run, "Lab", return_value=lab):
            result = driver_run.run_provider(
                "sqlserver-2022", "driverlab-binary", {driver_run.SQLSERVER_EULA},
            )
        self.assertEqual(result, {
            "provider": "sqlserver", "profile": "sqlserver-2022", "status": "FAIL",
            "phase": "provisioning", "stage": "engine-identity",
            "reason": "SQL Server provisioning target does not match the pin.",
        })
        self.assertEqual(container.exec.call_count, 1)
        container.docker.assert_not_called()

    def test_unreviewed_provisioning_exception_is_redacted(self):
        lab, container = self.mock_lab()
        container.wait.side_effect = driver_run.LabError("private-password; private-SQL")
        with patch.object(driver_run, "Lab", return_value=lab):
            result = driver_run.run_provider(
                "sqlserver-2022", "driverlab-binary", {driver_run.SQLSERVER_EULA},
            )
        self.assertEqual(result["status"], "FAIL")
        self.assertEqual(result["phase"], "provisioning")
        self.assertEqual(result["stage"], "engine-readiness")
        self.assertNotIn("private-", json.dumps(result))

    def test_readiness_and_cleanup_failure_remain_visible_without_private_details(self):
        lab, container = self.mock_lab()
        container.wait.side_effect = driver_run.CleanupFailed(
            driver_run.ReadinessTimedOut("private-readiness"),
            driver_run.LabError("private-cleanup"),
        )
        with patch.object(driver_run, "Lab", return_value=lab):
            result = driver_run.run_provider(
                "sqlserver-2022", "driverlab-binary", {driver_run.SQLSERVER_EULA},
            )
        self.assertEqual(result, {
            "provider": "sqlserver", "profile": "sqlserver-2022", "status": "FAIL",
            "phase": "provisioning", "stage": "engine-readiness",
            "reason": "Engine readiness deadline expired. Cleanup failed.",
        })

    def test_startup_prerequisite_stays_blocked_without_private_details(self):
        lab, container = self.mock_lab()
        lab.container.return_value = MagicMock()
        lab.container.return_value.__enter__.side_effect = driver_run.Blocked("private-endpoint")
        with patch.object(driver_run, "Lab", return_value=lab):
            result = driver_run.run_provider(
                "sqlserver-2022", "driverlab-binary", {driver_run.SQLSERVER_EULA},
            )
        self.assertEqual(result, {
            "provider": "sqlserver", "profile": "sqlserver-2022", "status": "BLOCKED",
            "stage": "docker-startup", "reason": "A SQL Server provisioning prerequisite is unavailable.",
        })
        container.docker.assert_not_called()

    def test_success_report_keeps_its_existing_shape(self):
        lab, container = self.mock_lab()
        scenarios = [{"provider": "sqlserver", "scenario": "LAB-DRIVER-001-S01", "status": "PASS"}]
        stdout = io.StringIO()
        with (
            patch.object(driver_run, "Lab", return_value=lab),
            patch.object(driver_run, "build_binary", return_value="driverlab-binary"),
            patch.object(driver_run, "verified_results", return_value=scenarios),
            redirect_stdout(stdout),
        ):
            self.assertEqual(driver_run.main(["sqlserver", "--accept-sqlserver-eula"]), 0)
        self.assertEqual(json.loads(stdout.getvalue()), {
            "provider": "sqlserver", "profile": "sqlserver-2022", "status": "PASS", "scenarios": scenarios,
        })
        container.wait.assert_called_once()
        self.assertEqual(container.docker.call_count, 2)


class OptionalScenarioEvidenceTests(unittest.TestCase):
    def test_requires_one_exact_result_for_requested_profile(self):
        expected = {
            "provider": "sqlite", "profile": "sqlite-3.51", "engine_version": "3.51.3",
            "scenario": "MODEL-TYPE-001-S02", "status": "PASS",
        }

        def line(value):
            return "GOEF_MODEL_TYPES_RESULT=" + json.dumps(value)

        def verify(output):
            return driver_run.verified_model_type_result(output, "sqlite", "sqlite-3.51", "3.51.3")

        marker = line(expected)
        self.assertEqual(verify(f"=== RUN   TestModelTypeRoundTrip\n{marker}\nPASS"), expected)
        for output in (
            "PASS", f"{marker}\n{marker}", "GOEF_MODEL_TYPES_RESULT={invalid",
            line({**expected, "profile": "sqlite-3.54"}),
            line({**expected, "engine_version": "3.54.1"}),
            line({**expected, "status": "SKIP"}),
            line({**expected, "unexpected": "private-data"}),
        ):
            with self.subTest(output=output), self.assertRaises(driver_run.LabError):
                verify(output)

    def test_owned_transaction_report_requires_the_requested_engine_and_scenario(self):
        expected = {
            "provider": "postgres", "profile": "postgres-18", "engine_version": "18.7",
            "scenario": "TX-OWN-001-S02", "status": "PASS",
        }

        def verify(*rows):
            output = "\n".join("GOEF_OWNED_TX_RESULT=" + json.dumps(row) for row in rows)
            return driver_run.verified_owned_tx_result(output, "postgres", "postgres-18", "18.7")

        self.assertEqual(verify(expected), expected)
        for rows in (
            (), (expected, expected), ({**expected, "scenario": "TX-OWN-001-S01"},),
            ({**expected, "engine_version": "18.6"},), ({**expected, "status": "SKIP"},),
        ):
            with self.subTest(rows=rows), self.assertRaises(driver_run.LabError):
                verify(*rows)

    def test_optional_cases_can_run_together(self):
        arguments, environment = driver_run.driver_test_options(
            model_types=True, owned_tx=True, savepoint_tx=True, save_changes=True,
        )
        self.assertEqual(arguments, (
            "-test.run=^(TestDriverGherkin|TestModelTypeRoundTrip|TestOwnedTransaction|"
            "TestSavepointAndExternalTransaction|TestSaveChangesTransaction)$",
            *driver_run.TEST_ARGS[1:],
        ))
        self.assertEqual(environment, {
            "GOEF_DRIVERLAB_MODEL_TYPES": "1", "GOEF_DRIVERLAB_OWNED_TX": "1", "GOEF_DRIVERLAB_SAVEPOINT_TX": "1",
            "GOEF_SAVE_CHANGES": "1",
        })
        self.assertEqual(driver_run.driver_test_options(), (driver_run.TEST_ARGS, {}))

    def test_savepoint_and_external_results_are_both_required(self):
        expected = [{
            "provider": "sqlite", "profile": "sqlite-3.51", "engine_version": "3.51.3",
            "scenario": scenario, "status": "PASS",
        } for scenario in ("TX-OWN-001-S01", "TX-SAVEPOINT-001-S03")]

        def verify(rows):
            output = "\n".join("GOEF_SAVEPOINT_TX_RESULT=" + json.dumps(row) for row in rows)
            return driver_run.verified_savepoint_tx_results(output, "sqlite", "sqlite-3.51", "3.51.3")

        self.assertEqual(verify(reversed(expected)), expected)
        for rows in (
            expected[:1], [expected[0], expected[0]], [*expected, expected[1]],
            [expected[0], {**expected[1], "scenario": "TX-SAVEPOINT-001-S01"}],
            [expected[0], {**expected[1], "profile": "sqlite-3.54"}],
            [expected[0], {**expected[1], "unexpected": "private-data"}],
        ):
            with self.subTest(rows=rows), self.assertRaises(driver_run.LabError):
                verify(rows)

    def test_transaction_options_requires_exact_profile(self):
        row = {"provider": "sqlite", "profile": "sqlite-3.51", "engine_version": "3.51.3",
               "scenario": "TX-ISOLATION-001-S01", "status": "PASS"}
        prefix = "GOEF_TX_OPTIONS_RESULT="
        verify = driver_run.verified_tx_options_result
        self.assertEqual(verify(prefix + json.dumps(row), "sqlite", "sqlite-3.51", "3.51.3"), row)
        for output in ("", "\n".join([prefix + json.dumps(row)] * 2),
                       prefix + json.dumps({**row, "engine_version": "3.54.1"})):
            with self.subTest(output=output), self.assertRaises(driver_run.LabError):
                verify(output, "sqlite", "sqlite-3.51", "3.51.3")
        args, env = driver_run.driver_test_options(tx_options=True)
        self.assertEqual(args[0], "-test.run=^(TestDriverGherkin|TestTransactionOptions)$")
        self.assertEqual(env, {"GOEF_TX_OPTIONS": "1"})


class SaveChangesEvidenceTests(unittest.TestCase):
    def rows(self, profile="sqlite-3.54", engine_version="3.54.1"):
        return [{
            "provider": "sqlite", "profile": profile, "engine_version": engine_version,
            "scenario": scenario, "status": "PASS",
        } for scenario in ("TX-OWN-001-S03", "TX-SAVEPOINT-001-S01", "TX-SAVEPOINT-001-S02")]

    def evidence(self, rows):
        return "\n".join("GOEF_SAVE_CHANGES_RESULT=" + json.dumps(row) for row in rows)

    def test_save_changes_requires_three_exact_results(self):
        expected = self.rows()

        def verify(output):
            return driver_run.verified_save_changes_results(output, "sqlite", "sqlite-3.54", "3.54.1")

        self.assertEqual(verify(self.evidence(reversed(expected))), expected)
        for rows in (expected[:-1], [*expected[:-1], expected[0]], [*expected, expected[-1]]):
            with self.subTest(rows=rows), self.assertRaises(driver_run.LabError):
                verify(self.evidence(rows))
        for field, wrong in (
            ("provider", "postgres"), ("profile", "sqlite-3.51"), ("engine_version", "3.51.3"),
            ("scenario", "TX-SAVEPOINT-001-S03"), ("status", "SKIP"), ("unexpected", "private-data"),
        ):
            with self.subTest(field=field):
                with self.assertRaises(driver_run.LabError) as failure:
                    verify(self.evidence([*expected[:-1], {**expected[-1], field: wrong}]))
                self.assertNotIn("private-data", str(failure.exception))
        malformed = self.evidence(expected[:-1]) + "\nGOEF_SAVE_CHANGES_RESULT={private-data"
        with self.assertRaises(driver_run.LabError) as failure:
            verify(malformed)
        self.assertNotIn("private-data", str(failure.exception))

    def test_selected_driver_profile_requires_save_changes_evidence(self):
        expected = self.rows()
        stdout = io.StringIO()
        with (
            patch.object(driver_run, "build_binary", return_value="driverlab-binary"),
            patch.object(driver_run, "command", side_effect=[self.evidence(expected), "PASS"]) as command,
            patch.object(driver_run, "verified_results", return_value=[]),
            redirect_stdout(stdout),
        ):
            self.assertEqual(driver_run.main(["sqlite", "--save-changes"]), 0)
            self.assertEqual(driver_run.main(["sqlite", "--save-changes"]), 1)

        for call in command.call_args_list:
            self.assertEqual(call.args[0][1], "-test.run=^(TestDriverGherkin|TestSaveChangesTransaction)$")
            self.assertEqual(call.kwargs["env"]["GOEF_SAVE_CHANGES"], "1")
        passed, failed = [json.loads(line) for line in stdout.getvalue().splitlines()]
        self.assertEqual(passed["save_changes"], expected)
        self.assertEqual((failed["status"], failed["phase"]), ("FAIL", "evidence"))

    def test_compatibility_runner_requires_save_changes_on_both_linked_runtimes(self):
        expected = []
        outputs = []
        profiles = []
        for name, profile in sqlite_compat.PROFILES.items():
            rows = self.rows(name, profile.engine_version)
            expected.extend(rows)
            outputs.append(self.evidence(rows))
            profiles.append(name)

        def run(supplied_outputs):
            stdout = io.StringIO()
            with (
                patch.object(sqlite_compat, "parse_file_feature", return_value=[]),
                patch.object(sqlite_compat, "run_rebuild_checks"),
                patch.object(sqlite_compat, "build", return_value=("driverlab-binary", "windows/amd64")),
                patch.object(sqlite_compat, "command", side_effect=supplied_outputs) as command,
                patch.object(sqlite_compat, "verified_driver_results", return_value=[{"facts": {}}]),
                redirect_stdout(stdout),
            ):
                result = sqlite_compat.main(["--save-changes"])
            return result, json.loads(stdout.getvalue()), command.call_args_list

        result, report, calls = run(outputs)
        self.assertEqual(result, 0)
        self.assertEqual(report["save_changes"], expected)
        self.assertEqual([call.kwargs["env"]["GOEF_DRIVERLAB_PROFILE"] for call in calls], profiles)
        self.assertTrue(all(call.kwargs["env"]["GOEF_SAVE_CHANGES"] == "1" for call in calls))

        result, report, _ = run([outputs[0], "PASS"])
        self.assertEqual(result, 1)
        self.assertEqual(report["phase"], f"Gherkin {profiles[1]}")


class DecimalBindingEvidenceTests(unittest.TestCase):
    prefix = "GOEF_DECIMAL_BINDING_RESULT="

    def row(self, profile="sqlite-3.54", engine_version="3.54.1"):
        return {
            "provider": "sqlite", "profile": profile, "engine_version": engine_version,
            "scenario": "MODEL-TYPE-001-S01", "status": "PASS",
        }

    def evidence(self, row):
        return self.prefix + json.dumps(row)

    def field_row(self, row):
        return {**row, "scenario": "MODEL-TYPE-001-S05"}

    def field_evidence(self, row):
        return "GOEF_DECIMAL_FIELD_BINDING_RESULT=" + json.dumps(self.field_row(row))

    def test_decimal_binding_verifies_one_exact_result(self):
        row = self.row()

        def verify(output):
            return driver_run.verified_decimal_binding_result(output, "sqlite", "sqlite-3.54", "3.54.1")

        marker = self.evidence(row)
        self.assertEqual(verify(marker), row)
        for output in ("", f"{marker}\n{marker}", self.prefix + "{private-data", self.prefix + "[]"):
            with self.subTest(output=output), self.assertRaises(driver_run.LabError) as failure:
                verify(output)
            self.assertNotIn("private-data", str(failure.exception))

        for field, wrong in (
            ("provider", "postgres"), ("profile", "sqlite-3.51"), ("engine_version", "3.51.3"),
            ("scenario", "MODEL-TYPE-001-S02"), ("status", "SKIP"), ("unexpected", "private-data"),
        ):
            with self.subTest(field=field), self.assertRaises(driver_run.LabError) as failure:
                verify(self.evidence({**row, field: wrong}))
            self.assertNotIn("private-data", str(failure.exception))

    def test_decimal_binding_is_opt_in_and_can_run_with_transaction_options(self):
        args, env = driver_run.driver_test_options(tx_options=True)
        self.assertEqual(args[0], "-test.run=^(TestDriverGherkin|TestTransactionOptions)$")
        self.assertEqual(env, {"GOEF_TX_OPTIONS": "1"})

        args, env = driver_run.driver_test_options(tx_options=True, decimal_binding=True)
        self.assertEqual(args[0], "-test.run=^(TestDriverGherkin|TestTransactionOptions|TestDecimalBindingRoundTrip)$")
        self.assertEqual(env, {"GOEF_TX_OPTIONS": "1", "GOEF_DECIMAL_BINDING": "1"})

    def test_driver_matrix_requires_decimal_evidence_when_selected(self):
        expected = self.row()
        stdout = io.StringIO()
        with (
            patch.object(driver_run, "build_binary", return_value="driverlab-binary"),
            patch.object(driver_run, "command", side_effect=[
                self.evidence(expected) + "\n" + self.field_evidence(expected),
                self.evidence(expected),
                "PASS",
            ]) as command,
            patch.object(driver_run, "verified_results", return_value=[]),
            redirect_stdout(stdout),
        ):
            self.assertEqual(driver_run.main(["sqlite", "--decimal-binding"]), 0)
            self.assertEqual(driver_run.main(["sqlite", "--decimal-binding"]), 1)
            self.assertEqual(driver_run.main(["sqlite"]), 0)

        for call in command.call_args_list[:2]:
            self.assertEqual(call.args[0][1], "-test.run=^(TestDriverGherkin|TestDecimalBindingRoundTrip)$")
            self.assertEqual(call.kwargs["env"]["GOEF_DECIMAL_BINDING"], "1")
        self.assertNotIn("GOEF_DECIMAL_BINDING", command.call_args_list[2].kwargs["env"])
        passed, failed, unrequested = [json.loads(line) for line in stdout.getvalue().splitlines()]
        self.assertEqual(passed["decimal_binding"], expected)
        self.assertEqual(passed["decimal_field_binding"], self.field_row(expected))
        self.assertEqual((failed["status"], failed["phase"]), ("FAIL", "evidence"))
        self.assertNotIn("decimal_binding", unrequested)
        self.assertNotIn("decimal_field_binding", unrequested)

    def test_sqlite_compatibility_requires_decimal_evidence_on_both_linked_runtimes(self):
        expected = [self.row(name, profile.engine_version) for name, profile in sqlite_compat.PROFILES.items()]
        outputs = [self.evidence(row) + "\n" + self.field_evidence(row) for row in expected]

        def run(supplied_outputs):
            stdout = io.StringIO()
            with (
                patch.object(sqlite_compat, "parse_file_feature", return_value=[]),
                patch.object(sqlite_compat, "run_rebuild_checks"),
                patch.object(sqlite_compat, "build", return_value=("driverlab-binary", "windows/amd64")),
                patch.object(sqlite_compat, "command", side_effect=supplied_outputs) as command,
                patch.object(sqlite_compat, "verified_driver_results", return_value=[{"facts": {}}]),
                redirect_stdout(stdout),
            ):
                result = sqlite_compat.main(["--decimal-binding"])
            return result, json.loads(stdout.getvalue()), command.call_args_list

        result, report, calls = run(outputs)
        self.assertEqual(result, 0)
        self.assertEqual(report["decimal_binding"], expected)
        self.assertEqual(report["decimal_field_binding"], [self.field_row(row) for row in expected])
        self.assertEqual(
            [call.kwargs["env"]["GOEF_DRIVERLAB_PROFILE"] for call in calls],
            list(sqlite_compat.PROFILES),
        )
        self.assertTrue(all(call.kwargs["env"]["GOEF_DECIMAL_BINDING"] == "1" for call in calls))
        result, report, _ = run([outputs[0], self.evidence(expected[1])])
        self.assertEqual(result, 1)
        self.assertEqual(report["phase"], f"Gherkin {expected[1]['profile']}")


class CommitOutcomeEvidenceTests(unittest.TestCase):
    def test_requested_commit_outcome_requires_both_exact_results(self):
        expected = [
            {
                "provider": "sqlite", "profile": "sqlite-3.54", "engine_version": "3.54.1",
                "scenario": scenario, "status": "PASS",
            }
            for scenario in ("TX-OUTCOME-001-S03", "TX-OUTCOME-001-S04")
        ]
        prefix = "GOEF_COMMIT_OUTCOME_RESULT="
        markers = [prefix + json.dumps(row) for row in expected]
        outputs = [
            "\n".join(markers),
            markers[0],
            "\n".join([markers[0], markers[0]]),
            "\n".join([markers[0], prefix + json.dumps({**expected[1], "engine_version": "3.51.3"})]),
            "",
        ]
        stdout = io.StringIO()
        with (
            patch.object(driver_run, "build_binary", return_value="driverlab-binary"),
            patch.object(driver_run, "command", side_effect=outputs) as command,
            patch.object(driver_run, "verified_results", return_value=[]),
            redirect_stdout(stdout),
        ):
            for code in (0, 1, 1, 1):
                self.assertEqual(driver_run.main(["sqlite", "--commit-outcome"]), code)
            self.assertEqual(driver_run.main(["sqlite"]), 0)

        passed, *failed, unrequested = [json.loads(line) for line in stdout.getvalue().splitlines()]
        self.assertEqual(passed["commit_outcome"], expected)
        self.assertTrue(all((row["status"], row["phase"]) == ("FAIL", "evidence") for row in failed))
        self.assertEqual(unrequested["status"], "PASS")
        self.assertNotIn("commit_outcome", unrequested)
        for call in command.call_args_list[:-1]:
            self.assertEqual(call.args[0][1], "-test.run=^(TestDriverGherkin|TestCommitOutcome)$")
            self.assertEqual(call.kwargs["env"]["GOEF_COMMIT_OUTCOME"], "1")


class DriverEvidenceTests(unittest.TestCase):
    def result_lines(self, provider="sqlite", native_codes=("2067", "787"), **profile_facts):
        if not profile_facts:
            profile_facts = {"engine_version": "3.54.1"}
        lock_native = {"postgres": "55P03", "sqlite": "5", "sqlserver": "1222", "mysql": "1205/HY000"}[provider]
        outcomes = (
            {}, {"native_code": native_codes[0]}, {"native_code": native_codes[1]},
            {"cancel_cause": "context.Canceled", "pool_in_use_after": "0"},
            {"lock_deadline_cause": "context.DeadlineExceeded", "lock_wait_observed": "both", "lock_timeout_native": lock_native},
            {
                "pre_dispatch_canceled": "context.Canceled", "pre_dispatch_deadline": "context.DeadlineExceeded",
                "pre_dispatch_calls": "0,0,1", "pre_dispatch_pool_in_use_after": "0",
            },
            {"acquire_cancel_cause": "context.Canceled", "acquire_wait_count_delta": "1", "acquire_pool_in_use_after": "0"},
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
        self.assertEqual(len(result), 7)
        self.assertNotIn("ignored_secret", result[0]["facts"])
        self.assertNotIn("target_marker", result[0]["facts"])
        self.assertEqual(result[3]["facts"], {
            "engine_version": "3.54.1", "cancel_cause": "context.Canceled", "pool_in_use_after": "0",
        })
        with self.assertRaises(driver_run.LabError):
            driver_run.verified_results(self.output([*rows[:3], *rows[4:]]), "sqlite")
        with self.assertRaises(driver_run.LabError):
            driver_run.verified_results(self.output([*rows, rows[3]]), "sqlite")

    def test_lock_timeout_is_required_and_redacted(self):
        rows = self.result_lines("mysql", ("1062/23000", "1452/23000"), server_version="8.4.13.1")
        result = driver_run.verified_results(self.output(rows), "mysql")
        self.assertEqual(result[4]["facts"], {
            "server_version": "8.4.13.1", "lock_deadline_cause": "context.DeadlineExceeded",
            "lock_wait_observed": "both", "lock_timeout_native": "1205/HY000",
        })
        for altered in ([*rows[:4], *rows[5:]], [*rows, rows[4]]):
            with self.assertRaises(driver_run.LabError):
                driver_run.verified_results(self.output(altered), "mysql")

    def test_pre_dispatch_evidence_is_required_and_redacted(self):
        rows = self.result_lines("mysql", ("1062/23000", "1452/23000"), server_version="8.4.13.1")
        result = driver_run.verified_results(self.output(rows), "mysql")
        self.assertEqual(result[5]["facts"], {
            "server_version": "8.4.13.1", "pre_dispatch_canceled": "context.Canceled",
            "pre_dispatch_deadline": "context.DeadlineExceeded", "pre_dispatch_calls": "0,0,1",
            "pre_dispatch_pool_in_use_after": "0",
        })
        for altered in ([*rows[:5], rows[6]], [*rows, rows[5]]):
            with self.assertRaises(driver_run.LabError):
                driver_run.verified_results(self.output(altered), "mysql")

    def test_acquire_cancellation_evidence_is_required_and_redacted(self):
        rows = self.result_lines("mysql", ("1062/23000", "1452/23000"), server_version="8.4.13.1")
        result = driver_run.verified_results(self.output(rows), "mysql")
        self.assertEqual(result[6]["facts"], {
            "server_version": "8.4.13.1", "acquire_cancel_cause": "context.Canceled",
            "acquire_wait_count_delta": "1", "acquire_pool_in_use_after": "0",
        })
        for altered in (rows[:-1], [*rows, rows[6]]):
            with self.assertRaises(driver_run.LabError):
                driver_run.verified_results(self.output(altered), "mysql")

    def test_acquire_cancellation_rejects_incomplete_or_unrelated_evidence(self):
        for field, wrong in (
            ("acquire_cancel_cause", None), ("acquire_cancel_cause", "context.DeadlineExceeded"),
            ("acquire_wait_count_delta", "0"), ("acquire_wait_count_delta", 1),
            ("acquire_pool_in_use_after", "1"), ("acquire_pool_in_use_after", 0),
            ("native_code", "5"), ("cancel_cause", "context.Canceled"),
            ("lock_wait_observed", "both"), ("pre_dispatch_calls", "0,0,1"),
        ):
            with self.subTest(field=field, wrong=wrong):
                rows = self.result_lines()
                if wrong is None:
                    rows[6]["facts"].pop(field)
                else:
                    rows[6]["facts"][field] = wrong
                with self.assertRaisesRegex(driver_run.LabError, "acquisition"):
                    driver_run.verified_results(self.output(rows), "sqlite")

    def test_pre_dispatch_rejects_incomplete_or_unrelated_evidence(self):
        for field, wrong in (
            ("pre_dispatch_canceled", "context.DeadlineExceeded"),
            ("pre_dispatch_deadline", "context.Canceled"),
            ("pre_dispatch_calls", "1,0,1"), ("pre_dispatch_calls", [0, 0, 1]),
            ("pre_dispatch_pool_in_use_after", 0), ("pre_dispatch_pool_in_use_after", None),
            ("native_code", "5"), ("lock_deadline_cause", "context.DeadlineExceeded"),
            ("lock_wait_observed", "both"), ("lock_timeout_native", "5"),
        ):
            with self.subTest(field=field, wrong=wrong):
                rows = self.result_lines()
                if wrong is None:
                    rows[5]["facts"].pop(field)
                else:
                    rows[5]["facts"][field] = wrong
                with self.assertRaisesRegex(driver_run.LabError, "pre-dispatch"):
                    driver_run.verified_results(self.output(rows), "sqlite")

    def test_lock_timeout_rejects_missing_or_incorrect_facts(self):
        for field, wrong in (
            ("lock_deadline_cause", None), ("lock_deadline_cause", "context.Canceled"),
            ("lock_wait_observed", None), ("lock_wait_observed", "caller"),
            ("lock_timeout_native", None), ("lock_timeout_native", "55P03"),
            ("lock_timeout_native", 5), ("native_code", "5"),
        ):
            with self.subTest(field=field, wrong=wrong):
                rows = self.result_lines()
                if wrong is None:
                    rows[4]["facts"].pop(field)
                else:
                    rows[4]["facts"][field] = wrong
                with self.assertRaisesRegex(driver_run.LabError, "lock timeout"):
                    driver_run.verified_results(self.output(rows), "sqlite")

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
        self.assertEqual(len(driver_run.verified_results(output, "sqlserver", "sqlserver-2017")), 7)
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
                self.assertEqual(len(result), 7)
                self.assertEqual([row["facts"]["generate_series"] for row in result], [availability] * 7)
                other = "sqlserver-2022-140" if profile == "sqlserver-2022" else "sqlserver-2022"
                with self.assertRaises(driver_run.LabError):
                    driver_run.verified_results(self.output(rows), "sqlserver", other)
                if probe_code:
                    self.assertEqual([row["facts"]["generate_series_native_code"] for row in result], [probe_code] * 7)
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
                self.assertEqual(len(driver_run.verified_results(output, "postgres", profile)), 7)
                with self.assertRaises(driver_run.LabError):
                    driver_run.verified_results(output, "postgres")

    def test_legacy_mysql_does_not_use_newer_identity_or_client_options(self):
        legacy = driver_run.PROFILES["mysql-8.0"]
        current = driver_run.PROFILES["mysql-8.4"]
        self.assertEqual(legacy.probe, "VERSION()")
        self.assertEqual(current.probe, "VERSION_WITH_UPDATE()")
        self.assertNotIn("--no-login-paths", legacy.client_flags)
        self.assertIn("--no-login-paths", current.client_flags)
