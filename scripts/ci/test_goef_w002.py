"""Regression tests for the local CI entry point."""

from contextlib import redirect_stdout
import io
import unittest
from unittest.mock import patch

import goef_w002 as runner


def postgres_report():
    scenario_facts = (
        {},
        {"native_code": "23505"},
        {"native_code": "23503"},
        {"cancel_cause": "context.Canceled", "pool_in_use_after": "0"},
        {
            "lock_deadline_cause": "context.DeadlineExceeded",
            "lock_timeout_native": "55P03",
            "lock_wait_observed": "both",
        },
        {
            "pre_dispatch_canceled": "context.Canceled",
            "pre_dispatch_deadline": "context.DeadlineExceeded",
            "pre_dispatch_calls": "0,0,1",
            "pre_dispatch_pool_in_use_after": "0",
        },
        {
            "acquire_cancel_cause": "context.Canceled",
            "acquire_pool_in_use_after": "0",
            "acquire_wait_count_delta": "1",
        },
    )
    return {
        "provider": "postgres",
        "profile": "postgres-18",
        "status": "PASS",
        "scenarios": [
            {
                "provider": "postgres",
                "scenario": f"LAB-DRIVER-001-S{number:02d}",
                "status": "PASS",
                "facts": {"engine_version": "18.7", "server_version_num": "180007", **facts},
            }
            for number, facts in enumerate(scenario_facts, start=1)
        ],
    }


class DriverMatrixTests(unittest.TestCase):
    def test_default_checks_all_reference_profiles(self):
        options = runner.parse_args([])
        plans = list(runner.driver_matrix_plan(options))

        self.assertEqual(len(plans), 1)
        self.assertEqual(runner.accepted_eula_flags(options), ())
        providers, profile, expected = plans[0]
        self.assertEqual(set(providers), {"postgres", "sqlite", "sqlserver", "mysql"})
        self.assertIsNone(profile)
        self.assertEqual(expected, {
            "postgres": "postgres-18",
            "sqlite": "sqlite-3.54",
            "sqlserver": "sqlserver-2022",
            "mysql": "mysql-8.4",
        })

    def test_compatibility_only_adds_profiles_for_selected_providers(self):
        options = runner.parse_args(["--providers", "postgres", "mysql", "--compatibility"])
        plans = list(runner.driver_matrix_plan(options))

        self.assertEqual(
            [profile for _, profile, _ in plans if profile is not None],
            ["postgres-16", "postgres-17", "postgres-19", "mysql-8.0"],
        )
        self.assertEqual(set(plans[0][0]), {"postgres", "mysql"})
        for providers, _, expected in plans:
            self.assertEqual(set(providers), set(expected))
            self.assertTrue(set(providers) <= {"postgres", "mysql"})

    def test_each_eula_only_enables_its_own_compatibility_profiles(self):
        cases = (
            ("--accept-sqlserver-eula", [None, "sqlserver-2022-140"], "sqlserver-2017"),
            ("--accept-legacy-sqlserver-eula", [None, "sqlserver-2017", "sqlserver-2019"], "sqlserver-2022-140"),
        )
        for flag, profiles, blocked in cases:
            with self.subTest(flag=flag):
                options = runner.parse_args(["--providers", "sqlserver", "--compatibility", flag])
                with patch.object(runner, "run_driver_matrix") as run:
                    with self.assertRaisesRegex(runner.CheckFailed, f"{blocked} is BLOCKED"):
                        runner.run_matrix(options, {})
                self.assertEqual([call.args[1] for call in run.call_args_list], profiles)
                self.assertEqual([call.args[-1] for call in run.call_args_list], [(flag,)] * len(profiles))


class DriverEvidenceTests(unittest.TestCase):
    def setUp(self):
        self.options = runner.parse_args([])

    def test_accepts_complete_pinned_driver_evidence(self):
        with redirect_stdout(io.StringIO()):
            runner.verify_driver_report(postgres_report(), "postgres-18", self.options)

    def test_rejects_missing_duplicate_or_wrong_engine_evidence(self):
        missing = postgres_report()
        missing["scenarios"].pop()
        duplicate = postgres_report()
        duplicate["scenarios"][-1] = duplicate["scenarios"][0]
        wrong_engine = postgres_report()
        wrong_engine["scenarios"][0]["facts"]["engine_version"] = "18.6"

        for defect, report in (
            ("missing scenario", missing),
            ("duplicate scenario", duplicate),
            ("wrong engine", wrong_engine),
            ("malformed report", [postgres_report()]),
        ):
            with self.subTest(defect=defect):
                with redirect_stdout(io.StringIO()), self.assertRaises((runner.CheckFailed, runner.LabError)):
                    runner.verify_driver_report(report, "postgres-18", self.options)

    def test_rejects_ambiguous_or_malformed_json(self):
        for payload in ('{"status": "PASS", "status": "FAIL"}', '{"count": NaN}', '[]', '{'):
            with self.subTest(payload=payload):
                with self.assertRaises(runner.CheckFailed):
                    runner.report_rows(payload)


if __name__ == "__main__":
    unittest.main()
