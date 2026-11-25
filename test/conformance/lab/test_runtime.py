"""Unit tests for resource ownership and failure reporting (GoEF #19)."""

from contextlib import redirect_stdout
import io
import json
import os
from pathlib import Path
import subprocess
from tempfile import TemporaryDirectory
import unittest
from unittest.mock import Mock, patch

import run
from runtime import Blocked, CommandError, CommandFailed, Container, Lab, LabError, command, local_endpoint


CID = "1" * 64
VOLUME = "2" * 64
IMAGE = "example/db@sha256:" + "a" * 64


class EndpointTests(unittest.TestCase):
    def test_remote_targets_are_refused(self):
        for endpoint in ("tcp://127.0.0.1:2375", "ssh://remote", "npipe:////remote/pipe/docker_engine", "unix://remote/socket"):
            with self.subTest(endpoint=endpoint), self.assertRaises(Blocked):
                local_endpoint(endpoint)

    def test_local_sockets(self):
        for endpoint in ("unix:///var/run/docker.sock", "npipe:////./pipe/dockerDesktopLinuxEngine"):
            self.assertEqual(local_endpoint(endpoint), endpoint)

    def test_context_and_host_must_be_local(self):
        with TemporaryDirectory() as directory:
            lab = Lab(Path(directory), None, False)
            with patch.dict(os.environ, {"DOCKER_HOST": "tcp://remote"}, clear=True), patch("runtime.command") as cmd:
                with self.assertRaises(Blocked):
                    lab._docker_client()
                cmd.assert_not_called()
            with patch.dict(os.environ, {}, clear=True), patch("runtime.command", return_value="ssh://remote") as cmd:
                with self.assertRaises(Blocked):
                    lab._docker_client()
                self.assertEqual(cmd.call_count, 1)

    def test_explicit_local_endpoint_and_daemon_architecture(self):
        with TemporaryDirectory() as directory, patch.dict(os.environ, {}, clear=True):
            lab = Lab(Path(directory), None, False)
            with patch("runtime.command", side_effect=["unix:///var/run/docker.sock", "linux/arm64"]) as cmd:
                with self.assertRaises(Blocked):
                    lab._docker_client()
                self.assertIn("--host", cmd.call_args.args[0])
                self.assertIn("unix:///var/run/docker.sock", cmd.call_args.args[0])


class ProcessTests(unittest.TestCase):
    def test_client_environment_and_diagnostics(self):
        with patch.dict(os.environ, {"DATABASE_URL": "secret", "PGHOST": "production", "PATH": "test"}):
            with patch("runtime.subprocess.run", return_value=subprocess.CompletedProcess([], 3, "secret", "secret")) as proc:
                with self.assertRaises(CommandFailed) as failure:
                    command(["client", "secret"], cwd=".", stdin="secret")
                self.assertEqual(failure.exception.returncode, 3)
                self.assertNotIn("secret", str(failure.exception))
                self.assertNotIn("DATABASE_URL", proc.call_args.kwargs["env"])
                self.assertNotIn("PGHOST", proc.call_args.kwargs["env"])
                self.assertEqual(proc.call_args.kwargs["input"], "secret")

    def test_timeout_distinct_from_client_exit(self):
        with patch("runtime.subprocess.run", side_effect=subprocess.TimeoutExpired(["secret"], 1)):
            with self.assertRaises(CommandError) as failure:
                command(["client"], cwd=".", timeout=1)
            self.assertNotIsInstance(failure.exception, CommandFailed)
            self.assertNotIn("secret", str(failure.exception))

    def test_expected_failure_requires_native_code_and_message(self):
        lab = Lab(Path("."), None, False)
        suffix = "FOREIGN KEY constraint failed (19)"
        for code, message, matches in (
            (1, f"secret: {suffix}\r\n", True),
            (0, suffix, False), (1, "secret", False), (2, suffix, False),
        ):
            result = subprocess.CompletedProcess([], code, "secret output", message)
            with self.subTest(code=code, matches=matches), patch("runtime.subprocess.run", return_value=result):
                if matches:
                    self.assertEqual(lab.local(["client"], expected_failure=(1, suffix)), "")
                else:
                    with self.assertRaises(LabError) as failure:
                        lab.local(["client"], expected_failure=(1, suffix))
                    self.assertNotIn("secret", str(failure.exception))

    def test_absence_requires_exact_daemon_diagnostic(self):
        for stderr in ("Error: No such container: " + CID, "Error response from daemon: get " + CID + ": no such volume"):
            with patch("runtime.subprocess.run", return_value=subprocess.CompletedProcess([], 1, "", stderr)):
                self.assertIsNone(command(["docker"], cwd=".", missing=CID))
        with patch("runtime.subprocess.run", return_value=subprocess.CompletedProcess([], 1, "", "connection refused")):
            with self.assertRaises(CommandFailed):
                command(["docker"], cwd=".", missing=CID)

    def test_exec_credentials_stay_in_environment(self):
        docker = Mock(return_value="row")
        env = {"TEST_PASSWORD": "secret"}
        self.assertEqual(Container(docker, CID).exec(["client"], env=env, stdin="query"), "row")
        self.assertEqual(docker.call_args.args[0], ["exec", "-i", "-e", "TEST_PASSWORD", CID, "client"])
        self.assertEqual(docker.call_args.kwargs["env"], env)

    def test_readiness_stops_at_deadline(self):
        docker = Mock(side_effect=CommandFailed(1))
        with patch("runtime.time.monotonic", side_effect=[100, 100, 101, 101]), patch("runtime.time.sleep"):
            with self.assertRaisesRegex(LabError, "readiness"):
                Container(docker, CID).wait(["probe"], timeout=1)
        self.assertEqual(docker.call_args.kwargs["timeout"], 1)


class AllocationTests(unittest.TestCase):
    def setUp(self):
        self.lab = Lab(Path("."), None, False)
        self.owner = "1234"
        patcher = patch("runtime.secrets.token_hex", return_value=self.owner)
        patcher.start()
        self.addCleanup(patcher.stop)
        self.metadata = f"{CID} {self.owner} {VOLUME}"

    def test_allocation_uses_no_network_and_removes_only_owned_objects(self):
        docker = Mock(side_effect=["linux/amd64", CID, CID, self.metadata, CID, None, None])
        with patch.object(self.lab, "_docker_client", return_value=docker):
            with self.lab.container(IMAGE, {"TEST_PASSWORD": "secret"}) as container:
                self.assertEqual(container.id, CID)
        commands = [c.args[0] for c in docker.call_args_list]
        self.assertEqual(commands[1][commands[1].index("--network") + 1], "none")
        self.assertNotIn("secret", commands[1])
        self.assertIn("TEST_PASSWORD", commands[1])
        self.assertFalse({"-p", "-P", "-v", "--mount", "--publish"}.intersection(commands[1]))
        self.assertEqual(commands[4], ["container", "rm", "--force", "--volumes", CID])
        self.assertEqual(commands[5][-1], CID)
        self.assertEqual(commands[6][-1], VOLUME)

    def test_start_failure_still_cleans(self):
        docker = Mock(side_effect=["linux/amd64", CID, CommandFailed(1), self.metadata, CID, None, None])
        with patch.object(self.lab, "_docker_client", return_value=docker):
            with self.assertRaises(CommandFailed), self.lab.container(IMAGE, {}):
                self.fail("Unstarted container reached worker")
        self.assertEqual(docker.call_args_list[4].args[0][-1], CID)

    def test_setup_and_cleanup_failures_are_both_reported(self):
        docker = Mock(side_effect=["linux/amd64", CID, CommandFailed(7), f"{CID} other"])
        with patch.object(self.lab, "_docker_client", return_value=docker):
            with self.assertRaises(LabError) as failure, self.lab.container(IMAGE, {}):
                self.fail("Unstarted container reached worker")
        self.assertIn("status 7", str(failure.exception))
        self.assertIn("ownership", str(failure.exception))

    def test_unexpected_worker_error_is_redacted_on_cleanup_failure(self):
        docker = Mock(side_effect=["linux/amd64", CID, CID, f"{CID} other"])
        with patch.object(self.lab, "_docker_client", return_value=docker):
            with self.assertRaises(LabError) as failure, self.lab.container(IMAGE, {}):
                raise RuntimeError("secret")
        self.assertIn("Unexpected", str(failure.exception))
        self.assertIn("ownership", str(failure.exception))
        self.assertNotIn("secret", str(failure.exception))

    def test_partial_create_resolves_name_and_rechecks_label(self):
        docker = Mock(side_effect=["linux/amd64", CommandError("Command timed out."), None, self.metadata, CID, None, None])
        with patch.object(self.lab, "_docker_client", return_value=docker), patch("runtime.time.sleep"):
            with self.assertRaisesRegex(CommandError, "timed out"), self.lab.container(IMAGE, {}):
                self.fail("Partially created container reached worker")
        self.assertEqual(docker.call_args_list[2].args[0][-1], "goef-lab-" + self.owner)
        self.assertEqual(docker.call_args_list[4].args[0][-1], CID)

    def test_foreign_container_is_untouched(self):
        docker = Mock(return_value=f"{CID} other")
        with self.assertRaisesRegex(LabError, "ownership"):
            self.lab._cleanup(docker, "goef-lab-" + self.owner, self.owner)
        self.assertEqual(docker.call_count, 1)

    def test_rm_retry_rechecks_id_and_owner(self):
        docker = Mock(side_effect=[self.metadata, CommandFailed(1), f"{CID} other"])
        with patch("runtime.time.sleep"), self.assertRaisesRegex(LabError, "ownership"):
            self.lab._cleanup(docker, CID, self.owner)
        self.assertEqual(docker.call_args_list[-1].args[0][-1], CID)
        self.assertEqual(sum("rm" in c.args[0] for c in docker.call_args_list), 1)

    def test_lost_rm_reply_preserves_volume_verification(self):
        docker = Mock(side_effect=[self.metadata, CommandError("timed out"), None, None])
        with patch("runtime.time.sleep"):
            self.lab._cleanup(docker, CID, self.owner)
        self.assertEqual(docker.call_args.args[0][-1], VOLUME)

    def test_unverified_cleanup_fails(self):
        for result in (VOLUME, CommandFailed(1)):
            docker = Mock(side_effect=[self.metadata, CID, None, result])
            with self.assertRaises(LabError):
                self.lab._cleanup(docker, CID, self.owner)

    def test_wrong_platform_or_mutable_tag_prevents_creation(self):
        docker = Mock(return_value="linux/arm64")
        with patch.object(self.lab, "_docker_client", return_value=docker):
            for image in (IMAGE, "example/db:latest"):
                with self.assertRaises(LabError), self.lab.container(image, {}):
                    self.fail("Invalid image allocated")
        self.assertEqual(docker.call_count, 1)


class ReportTests(unittest.TestCase):
    def test_eula_scope_and_redaction(self):
        with patch.dict(os.environ, {"ACCEPT_EULA": "Y"}), patch("run.importlib.import_module") as provider:
            self.assertEqual(run.run_provider("sqlserver", None, False)["status"], "BLOCKED")
            provider.assert_not_called()
            provider.return_value.smoke.side_effect = OSError("secret")
            report = run.run_provider("sqlserver", None, True)
            self.assertEqual(report["status"], "FAIL")
            self.assertNotIn("secret", str(report))

    def test_all_finishes_and_requests_nonzero_for_a_blocked_cell(self):
        provider = Mock()
        provider.smoke.return_value = {"ready": True}
        output = io.StringIO()
        with patch("run.importlib.import_module", return_value=provider), redirect_stdout(output):
            self.assertEqual(run.main(["all"]), 1)
        reports = [json.loads(line) for line in output.getvalue().splitlines()]
        self.assertEqual([row["provider"] for row in reports], list(run.PROVIDERS))
        self.assertEqual([row["status"] for row in reports], ["PASS", "PASS", "BLOCKED", "PASS"])


if __name__ == "__main__":
    unittest.main()
