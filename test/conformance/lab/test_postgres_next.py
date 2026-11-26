"""Check selection, evidence and cleanup for the PostgreSQL Next runner."""

from contextlib import contextmanager, redirect_stderr, redirect_stdout
import io
import json
import unittest
from unittest.mock import Mock, patch

import postgres_next as runner
from runtime import LabError


def evidence_rows():
    common = {
        "engine_version": "18.7", "server_version_num": "180007",
        "first": "1", "phase": "native-next", "lock_wait": "observed",
    }
    return [
        {
            "provider": "postgres", "scenario": "cancel-native-next", "status": "PASS",
            "facts": common | {
                "terminal": "context.Canceled",
                "backend": "exited-before-unlock",
                "recovery": "original-row",
            },
        },
        {
            "provider": "postgres", "scenario": "holder-release-control", "status": "PASS",
            "facts": common | {"second": "2", "terminal": "clean-eof"},
        },
    ]


def output(rows):
    return "\n".join(runner.RESULT_PREFIX + json.dumps(item) for item in rows)


class PostgresNextTests(unittest.TestCase):
    def test_evidence_requires_both_exact_results(self):
        profile = runner.POSTGRES_PROFILES["postgres-18"]
        rows = evidence_rows()
        self.assertEqual(runner.verified_results(output(rows[::-1]), profile), rows)

        wrong = {**rows[0], "facts": {**rows[0]["facts"], "terminal": "clean-eof"}}
        private = {**rows[0], "facts": {**rows[0]["facts"], "credential": "synthetic-secret"}}
        for name, invalid in (
            ("missing", output(rows[:1])),
            ("duplicate", output(rows + rows[:1])),
            ("wrong cause", output([wrong, rows[1]])),
            ("unexpected fact", output([private, rows[1]])),
            ("malformed", runner.RESULT_PREFIX + "{"),
        ):
            with self.subTest(name=name), self.assertRaises(LabError) as caught:
                runner.verified_results(invalid, profile)
            self.assertNotIn("synthetic-secret", str(caught.exception))
        with self.assertRaises(LabError):
            runner.verified_results(output(rows), runner.POSTGRES_PROFILES["postgres-16"])

    def test_profile_must_be_explicit_and_postgres(self):
        for args in ([], ["--profile", "sqlite-3.54"]):
            with self.subTest(args=args), patch.object(runner, "build_binary") as build, redirect_stderr(io.StringIO()):
                with self.assertRaises(SystemExit) as caught:
                    runner.main(args)
                self.assertEqual(caught.exception.code, 2)
                build.assert_not_called()

    def test_builds_once_for_distinct_selected_profiles(self):
        stream = io.StringIO()
        with (
            patch.object(runner, "build_binary", return_value="owned-binary") as build,
            patch.object(runner, "run_profile", side_effect=[{"status": "FAIL"}, {"status": "PASS"}]) as run,
            redirect_stdout(stream),
        ):
            result = runner.main(["--profile", "postgres-16", "--profile", "postgres-17", "--profile", "postgres-16"])
        self.assertEqual(result, 1)
        build.assert_called_once()
        self.assertEqual([call.args for call in run.call_args_list], [
            ("postgres-16", "owned-binary"), ("postgres-17", "owned-binary"),
        ])
        self.assertEqual(len(stream.getvalue().splitlines()), 2)

    def test_cleanup_failure_prevents_a_pass(self):
        container = Mock(id="owned")
        container.exec.return_value = output(evidence_rows())

        @contextmanager
        def bad_cleanup(*_):
            yield container
            raise LabError("Could not verify test cleanup.")

        with patch.object(runner, "Lab"), patch.object(runner, "postgres_container", bad_cleanup):
            result = runner.run_profile("postgres-18", "owned-binary")
        self.assertEqual(result["status"], "FAIL")
        self.assertEqual(result["phase"], "cleanup")
        self.assertNotIn("scenarios", result)


if __name__ == "__main__":
    unittest.main()
