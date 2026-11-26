"""Ensure WAL recovery evidence rejects missing frames, invalid stages and clean checkpoints."""

from pathlib import Path
from tempfile import TemporaryDirectory
import unittest
from unittest.mock import patch

import sqlite_wal_recovery as lab


class SQLiteWALRecoveryTests(unittest.TestCase):
    def test_commit_exit_is_expected_only_for_writer(self):
        profile = lab.PROFILES[lab.MINIMUM_PROFILE]
        with patch.object(lab, "command", return_value="") as command:
            lab.run_stage(Path("fixture"), "worker", profile, "commit-exit", "token")
            self.assertEqual(
                command.call_args.kwargs["expected_failure"],
                (lab.COMMITTED_EXIT, f"{lab.RESULT_PREFIX}3.51.3/commit-exit"),
            )
            env = command.call_args.kwargs["env"]
            self.assertEqual(env["GOEF_SQLITE_WAL_RECOVERY_STAGE"], "commit-exit")
            self.assertEqual(env["GOEF_SQLITE_COMPAT_TOKEN"], "token")

    def test_checkpoint_requires_exactly_one_matching_result(self):
        profile = lab.PROFILES[lab.CURRENT]
        marker = f"{lab.RESULT_PREFIX}3.54.1/checkpoint"
        with patch.object(lab, "command", return_value=marker) as command:
            lab.run_stage(Path("fixture"), "worker", profile, "checkpoint", "token")
            self.assertIsNone(command.call_args.kwargs["expected_failure"])
        for output in ("", marker + "\n" + marker, marker.replace("3.54.1", "3.51.3")):
            with self.subTest(output=output):
                with patch.object(lab, "command", return_value=output):
                    with self.assertRaisesRegex(lab.LabError, "pinned runtime"):
                        lab.run_stage(Path("fixture"), "worker", profile, "checkpoint", "token")

    def test_files_must_show_recovery_at_each_boundary(self):
        for variant, expected in (
            ("missing WAL", "committed change must still be in WAL"),
            ("premature checkpoint", "committed change must still be in WAL"),
            ("WAL retains frames", "checkpoint must move the committed change"),
            ("unchanged after recovery", "checkpoint must move the committed change"),
            ("complete", None),
        ):
            with self.subTest(variant=variant), TemporaryDirectory() as root:
                scenario, origin, peer = lab.DIRECTIONS[0]
                directory = Path(root) / scenario
                stages = []

                def run_stage(directory, binary, profile, stage, token):
                    stages.append(stage)
                    main = directory / lab.MAIN_FILE
                    wal = directory / lab.WAL_FILE
                    if stage == "seed":
                        main.write_bytes(b"seed")
                    elif stage == "commit-exit":
                        if variant != "missing WAL":
                            wal.write_bytes(b"committed WAL frames" * 4)
                        if variant == "premature checkpoint":
                            main.write_bytes(b"migrated")
                    elif stage == "checkpoint":
                        if variant != "WAL retains frames":
                            wal.write_bytes(b"")
                        if variant != "unchanged after recovery":
                            main.write_bytes(b"migrated")

                with patch.object(lab, "run_stage", side_effect=run_stage):
                    report = {}
                    binaries = {origin: "origin-worker", peer: "peer-worker"}
                    if expected is not None:
                        with self.assertRaisesRegex(lab.LabError, expected):
                            lab.run_direction(directory, binaries, scenario, origin, peer, report)
                        self.assertNotIn("reopen", stages)
                    else:
                        result = lab.run_direction(directory, binaries, scenario, origin, peer, report)
                        self.assertEqual(stages, ["seed", "commit-exit", "checkpoint", "reopen"])
                        self.assertEqual((result["writer"], result["checkpointing_peer"]), (origin, peer))
                        self.assertEqual(result["status"], "PASS")


if __name__ == "__main__":
    unittest.main()
