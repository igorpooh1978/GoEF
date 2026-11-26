"""Check the public result boundary for the MySQL in-flight SPI read fixture."""

import json
import unittest

from mysql_in_flight_read import RESULT_PREFIX, verified_result
from runtime import LabError


class ResultTests(unittest.TestCase):
    def setUp(self):
        self.result = {
            "profile": "mysql-8.0",
            "server_version": "8.0.46",
            "status": "PASS",
            "wait_observed": "exact_innodb_threads",
            "release_control": "row_then_eof",
            "cancel_cause": "context.Canceled",
            "cancel_phase": "query",
            "dispatch_count": "1_each",
            "pool_after_cancel": "0_open_0_in_use",
            "recovery": "fresh_session_row_unchanged",
        }

    def test_accepts_both_documented_cancellation_boundaries(self):
        for phase in ("query", "first_next"):
            with self.subTest(phase=phase):
                row = {**self.result, "cancel_phase": phase}
                self.assertEqual(verified_result(RESULT_PREFIX + json.dumps(row), "mysql-8.0"), row)

    def test_rejects_absent_repeated_and_malformed_results(self):
        marker = RESULT_PREFIX + json.dumps(self.result)
        for output in ("", f"{marker}\n{marker}", RESULT_PREFIX + "{", RESULT_PREFIX + "[]"):
            with self.subTest(output=output), self.assertRaises(LabError):
                verified_result(output, "mysql-8.0")

    def test_rejects_wrong_identity_native_timeout_unobserved_waits_and_extra_fields(self):
        for changed in (
            {"server_version": "8.4.13.1"},
            {"cancel_cause": "context.DeadlineExceeded"},
            {"cancel_phase": "clean_eof"},
            {"wait_observed": "timeout_only"},
            {"pool_after_cancel": "1_open_0_in_use"},
            {"diagnostic": "unapproved value"},
        ):
            with self.subTest(changed=changed), self.assertRaises(LabError):
                output = RESULT_PREFIX + json.dumps({**self.result, **changed})
                verified_result(output, "mysql-8.0")


if __name__ == "__main__":
    unittest.main()
