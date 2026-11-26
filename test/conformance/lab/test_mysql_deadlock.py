"""Check the public evidence boundary for the focused MySQL deadlock lab."""

import json
import unittest

from mysql_deadlock import RESULT_PREFIX, verified_result
from runtime import LabError


class ResultTests(unittest.TestCase):
    def setUp(self):
        self.record = {
            "profile": "mysql-8.0",
            "server_version": "8.0.46",
            "status": "PASS",
            "native_code": "1213/40001",
            "wait_observed": "requester_and_blocker",
            "victim_rollback": "verified_before_explicit_rollback",
            "survivor_commit": "verified",
            "fresh_victim_commit": "verified",
        }

    def test_accepts_exact_result(self):
        output = RESULT_PREFIX + json.dumps(self.record)
        self.assertEqual(verified_result(output, "mysql-8.0"), self.record)

    def test_rejects_missing_duplicate_or_malformed_result(self):
        record = RESULT_PREFIX + json.dumps(self.record)
        for output in ("", record + "\n" + record, RESULT_PREFIX + "{"):
            with self.subTest(output=output), self.assertRaises(LabError):
                verified_result(output, "mysql-8.0")

    def test_rejects_wrong_profile_native_timeout_and_unapproved_fields(self):
        for changed in (
            {"server_version": "8.4.13.1"},
            {"native_code": "1205/HY000"},
            {"diagnostic": "unapproved value"},
        ):
            with self.subTest(changed=changed), self.assertRaises(LabError):
                output = RESULT_PREFIX + json.dumps({**self.record, **changed})
                verified_result(output, "mysql-8.0")


if __name__ == "__main__":
    unittest.main()
