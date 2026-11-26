"""Check SQLite driver evidence and the executable file feature binding."""

import json
import unittest
from unittest.mock import patch

import sqlite_compat as lab


class SQLiteDriverEvidenceTests(unittest.TestCase):
    def result_lines(self, profile):
        common = {
            "engine_version": profile.engine_version, "source_id": profile.source_id,
            "journal_mode": "wal", "foreign_keys": "1", "busy_timeout_ms": "1000",
            "compile_options_count": "55", "compile_options_sha256": "a" * 64,
        }
        outcomes = (
            {}, {"native_code": "2067"}, {"native_code": "787"},
            {"cancel_cause": "context.Canceled", "pool_in_use_after": "0"},
        )
        return [
            {
                "provider": "sqlite", "scenario": f"LAB-DRIVER-001-S0{index}", "status": "PASS",
                "facts": {**common, **outcome, "ignored_secret": "fixture"},
            }
            for index, outcome in enumerate(outcomes, 1)
        ]

    def output(self, rows):
        return "\n".join(lab.RESULT_PREFIX + json.dumps(row) for row in rows)

    def test_cancellation_is_required_on_both_linked_runtimes(self):
        for name, profile in lab.PROFILES.items():
            with self.subTest(profile=name):
                rows = self.result_lines(profile)
                results = lab.verified_driver_results(self.output(rows), name, profile)
                self.assertEqual(len(results), 4)
                self.assertEqual(results[3]["profile"], name)
                facts = results[3]["facts"]
                self.assertEqual(facts["cancel_cause"], "context.Canceled")
                self.assertEqual(facts["pool_in_use_after"], "0")
                self.assertNotIn("native_code", facts)
                self.assertNotIn("ignored_secret", facts)
                for altered in (rows[:-1], [*rows, rows[3]]):
                    with self.assertRaises(lab.LabError):
                        lab.verified_driver_results(self.output(altered), name, profile)

    def test_cancellation_rejects_invalid_facts(self):
        name = lab.MINIMUM_PROFILE
        profile = lab.PROFILES[name]
        for field, value in (
            ("cancel_cause", "context.DeadlineExceeded"), ("pool_in_use_after", "1"),
            ("pool_in_use_after", 0), ("native_code", "787"),
        ):
            with self.subTest(field=field, value=value):
                rows = self.result_lines(profile)
                rows[3]["facts"][field] = value
                with self.assertRaisesRegex(lab.LabError, "cancellation"):
                    lab.verified_driver_results(self.output(rows), name, profile)


class SQLiteFileFeatureTests(unittest.TestCase):
    def setUp(self):
        self.source = lab.FILE_FEATURE.read_text(encoding="utf-8")

    def test_both_directions_bind_to_the_example_rows_and_worker_stages(self):
        older, current = lab.MINIMUM_PROFILE, lab.CURRENT
        examples = lab.parse_file_feature(self.source)
        self.assertEqual(
            examples,
            [
                lab.FileExample("LAB-SQLITE-FILE-001-S01", mode, (("create", older), ("mutate", current), ("reopen", older)))
                for mode in ("delete", "wal")
            ] + [
                lab.FileExample("LAB-SQLITE-FILE-001-S02", mode, (("create", current), ("mutate", older), ("reopen", current)))
                for mode in ("delete", "wal")
            ],
        )
        reordered = self.source.replace("| delete       |\n      | wal          |", "| wal |\n      | delete |", 1)
        self.assertEqual([case.journal for case in lab.parse_file_feature(reordered)], ["wal", "delete", "delete", "wal"])

    def test_incomplete_or_unbound_contract_prevents_execution(self):
        for original, replacement in (
            ("| wal          |", ""),
            ("| wal          |", "| delete |"),
            ("| wal          |", "| memory |"),
            ("| journal_mode |", "| journal_mode | unused |"),
            ("Given SQLite 3.51.3 has created", "Given SQLite 3.54.1 has created"),
            ("When SQLite 3.54.1 reads the rows", "When SQLite 3.53.0 reads the rows"),
            ("Then the existing schema, rows, audit records and migration history are preserved", "Then the rows exist"),
            ("Then the existing schema, rows, audit records and migration history are preserved", ""),
            ("And SQLite 3.54.1 rejects a noncoercible stored generated value with its native datatype constraint code", ""),
            ("And integrity and foreign-key checks succeed", "And nothing else is verified"),
        ):
            with self.subTest(original=original, replacement=replacement):
                modified = self.source.replace(original, replacement, 1)
                self.assertNotEqual(modified, self.source)
                with patch.object(lab, "FILE_FEATURE") as feature, patch.object(lab, "build") as build:
                    feature.read_text.return_value = modified
                    with self.assertRaises(lab.LabError):
                        lab.run_lab(None, {})
                    build.assert_not_called()

    def test_exchange_stage_requires_the_expected_engine_marker(self):
        profile = lab.PROFILES[lab.MINIMUM_PROFILE]
        args = (None, "sqlite", profile, "wal", "create", "unused")
        expected = lab.FILE_RESULT_PREFIX + "3.51.3/wal/create"
        with patch.object(lab, "command", return_value=expected):
            self.assertEqual(lab.run_exchange_stage(*args), ("3.51.3", None))
        for output in ("", expected + "\n" + expected, expected.replace("3.51.3", "3.54.1")):
            with self.subTest(output=output), patch.object(lab, "command", return_value=output):
                with self.assertRaises(lab.LabError):
                    lab.run_exchange_stage(*args)

    def test_mutate_stage_requires_the_native_datatype_code(self):
        args = (None, "sqlite", lab.PROFILES[lab.MINIMUM_PROFILE], "delete", "mutate", "unused")
        marker = lab.FILE_RESULT_PREFIX + "3.51.3/delete/mutate"
        with patch.object(lab, "command", return_value=marker + "/3091"):
            self.assertEqual(lab.run_exchange_stage(*args), ("3.51.3", "3091"))
        for output in (marker, marker + "/19", marker + "/2067"):
            with self.subTest(output=output), patch.object(lab, "command", return_value=output):
                with self.assertRaises(lab.LabError):
                    lab.run_exchange_stage(*args)
