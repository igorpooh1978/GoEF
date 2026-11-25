"""Isolated CLI smoke runtime for GoEF issue #19."""

from contextlib import contextmanager
import os
from pathlib import Path
import re
import secrets
import subprocess
import time


OWNER = "org.goef.lab.owner"
FULL_ID = re.compile(r"[0-9a-f]{64}")
PLATFORM = "linux/amd64"
METADATA = (
    '{{.Id}} {{index .Config.Labels "' + OWNER + '"}} '
    '{{range .Mounts}}{{if eq .Type "volume"}}{{.Name}} {{end}}{{end}}'
)


class LabError(Exception):
    """A reviewed diagnostic suitable for the public smoke report."""


class Blocked(LabError):
    """An unavailable prerequisite prevented execution."""


class CommandError(LabError):
    """A command returned a nonzero status or exceeded its deadline."""


class CommandFailed(CommandError):
    """Only the exit code is retained from an unsuccessful child process."""

    def __init__(self, returncode):
        self.returncode = returncode
        super().__init__(f"Command exited with status {returncode}.")


def safe_reason(error):
    return str(error) if isinstance(error, LabError) else "Unexpected lab failure."


def local_endpoint(endpoint):
    if re.fullmatch(r"unix:///(?!/)[^\s?#]+", endpoint) or endpoint.lower() in (
        "npipe:////./pipe/docker_engine", "npipe:////./pipe/dockerdesktoplinuxengine",
    ):
        return endpoint
    raise Blocked("A local Docker Unix socket or Docker Desktop named pipe is required.")


def inherited(names):
    return {key: os.environ[key] for key in names if key in os.environ}


def command(argv, *, cwd, env=None, stdin=None, timeout=15, missing=None, expected_failure=None):
    # Do not let database clients discover workstation connection variables.
    child_env = inherited(("PATH", "SYSTEMROOT", "WINDIR", "SYSTEMDRIVE", "TMPDIR", "TEMP", "TMP"))
    child_env.update(env or {})
    try:
        result = subprocess.run(
            argv, cwd=cwd, env=child_env, input=stdin,
            stdin=subprocess.DEVNULL if stdin is None else None,
            stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True,
            encoding="utf-8", errors="replace", timeout=timeout, check=False,
        )
    except FileNotFoundError:
        raise Blocked("A required executable is unavailable.") from None
    except subprocess.TimeoutExpired:
        raise CommandError("Command timed out.") from None
    except OSError:
        raise LabError("Could not execute the lab command.") from None
    if expected_failure is not None:
        code, suffix = expected_failure
        if not result.returncode:
            raise LabError("The negative probe unexpectedly succeeded.")
        if result.returncode != code or not suffix or not result.stderr.rstrip("\r\n").endswith(suffix):
            raise LabError("The negative probe did not return the expected native diagnostic.")
        return ""
    if result.returncode:
        # Only a daemon-confirmed absence is evidence of teardown. Transport
        # errors and permission failures cannot be mistaken for absent objects.
        if missing:
            key = re.escape(missing)
            if re.search(
                rf"(?:^|\n)(?:Error(?: response from daemon)?: )?(?:No such "
                rf"(?:object|container|volume): {key}|get {key}: no such volume|volume {key} not found)\s*\Z",
                result.stderr, re.IGNORECASE,
            ):
                return None
        raise CommandFailed(result.returncode)
    return result.stdout


def env_flags(env):
    flags = []
    for key in env:
        if not re.fullmatch(r"[A-Z][A-Z0-9_]*", key):
            raise LabError("Invalid engine environment variable.")
        flags.extend(("-e", key))
    return flags


class Container:
    def __init__(self, docker, container_id):
        self.docker = docker
        self.id = container_id

    def exec(self, argv, *, stdin=None, env=None, timeout=15):
        args = ["exec"]
        if stdin is not None:
            args.append("-i")
        return self.docker(
            [*args, *env_flags(env or {}), self.id, *argv],
            stdin=stdin, env=env, timeout=timeout,
        )

    def wait(self, argv, *, env=None, timeout=180):
        deadline = time.monotonic() + timeout
        while (remaining := deadline - time.monotonic()) > 0:
            try:
                return self.exec(argv, env=env, timeout=min(10, remaining))
            except CommandError:
                time.sleep(min(1, max(0, deadline - time.monotonic())))
        raise LabError("Engine readiness deadline expired.")


class Lab:
    def __init__(self, tempdir: Path, sqlite_binary: Path | None, allow_sqlserver: bool):
        self.tempdir = tempdir
        self.sqlite_binary = sqlite_binary
        self.allow_sqlserver = allow_sqlserver

    def local(self, argv, *, stdin=None, timeout=15, expected_failure=None):
        return command(argv, cwd=self.tempdir, stdin=stdin, timeout=timeout, expected_failure=expected_failure)

    def _docker_client(self):
        host = os.environ.get("DOCKER_HOST")
        if host:
            local_endpoint(host)
        context_env = inherited(("HOME", "USERPROFILE", "DOCKER_CONTEXT", "DOCKER_CONFIG", "APPDATA", "LOCALAPPDATA"))
        try:
            endpoint = command(
                ["docker", "context", "inspect", "--format", "{{.Endpoints.docker.Host}}"],
                cwd=self.tempdir, env=context_env,
            ).strip()
        except CommandError:
            raise Blocked("Unable to inspect the current Docker context.") from None
        endpoint = local_endpoint(endpoint)
        if host and not context_env.get("DOCKER_CONTEXT"):
            endpoint = host

        # Pin the inspected endpoint for the whole allocation, even if the
        # active context changes. An empty config also prevents proxy settings
        # in the user's Docker config from being injected into the container.
        config_dir = self.tempdir / "docker-config"
        config_dir.mkdir(mode=0o700, exist_ok=True)

        def docker(argv, *, env=None, **kwargs):
            return command(
                ["docker", "--config", str(config_dir), "--host", endpoint, *argv],
                cwd=self.tempdir, env=env, **kwargs,
            )

        try:
            platform = docker(["version", "--format", "{{.Server.Os}}/{{.Server.Arch}}"]).strip()
        except CommandError:
            raise Blocked("The local Docker daemon is unavailable.") from None
        if platform != PLATFORM:
            raise Blocked("The local Docker daemon must run Linux/amd64.")
        return docker

    @contextmanager
    def container(self, image, env):
        if not re.fullmatch(r"[a-z0-9][a-z0-9./_:-]*@sha256:[0-9a-f]{64}", image):
            raise LabError("The engine image requires an immutable SHA-256 digest.")
        docker = self._docker_client()
        image_info = ["image", "inspect", "--format", "{{.Os}}/{{.Architecture}}", image]
        try:
            platform = docker(image_info).strip()
        except CommandError:
            try:
                docker(["pull", "--platform", PLATFORM, image], timeout=600)
            except CommandError:
                raise Blocked("The pinned engine image could not be retrieved.") from None
            platform = docker(image_info).strip()
        if platform != PLATFORM:
            raise Blocked("The pinned engine image must target Linux/amd64.")

        owner = secrets.token_hex(16)
        name = "goef-lab-" + owner
        flags = env_flags(env)
        target = name
        try:
            # Record the unique name/label before create. If create loses its
            # reply, inspect only that exact name to recover the immutable ID.
            created = docker([
                "create", "--pull", "never", "--platform", PLATFORM,
                "--network", "none", "--name", name, "--label", f"{OWNER}={owner}",
                *flags, image,
            ], env=env, timeout=60).strip()
            if not FULL_ID.fullmatch(created):
                raise LabError("Docker returned an invalid container ID.")
            target = created
            docker(["start", target], timeout=90)
            yield Container(docker, target)
        except BaseException as primary:
            try:
                self._cleanup(docker, target, owner)
            except Exception as cleanup:
                raise LabError(f"Smoke failed ({safe_reason(primary)}). Cleanup failed ({safe_reason(cleanup)}).") from None
            raise
        else:
            self._cleanup(docker, target, owner)

    @staticmethod
    def _cleanup(docker, target, owner):
        volumes = None
        for attempt in range(3):
            if attempt:
                time.sleep(1)
            try:
                metadata = docker(["container", "inspect", "--format", METADATA, target], missing=target)
                if metadata is None:
                    if volumes is not None:
                        break
                    # A lost create reply can precede completion on the daemon.
                    continue
                fields = metadata.split()
                if (len(fields) < 2 or not FULL_ID.fullmatch(fields[0]) or fields[1] != owner
                        or (FULL_ID.fullmatch(target) and fields[0] != target)):
                    raise LabError("Container ownership verification failed during cleanup.")
                target, _, *volumes = fields
                if any(not FULL_ID.fullmatch(volume) for volume in volumes):
                    raise LabError("Unexpected named volume on the lab container.")
                docker(["container", "rm", "--force", "--volumes", target], timeout=60)
                if docker(["container", "inspect", "--format", "{{.Id}}", target], missing=target) is None:
                    break
            except CommandError:
                continue
        else:
            raise LabError(f"Could not verify container cleanup for {target}.")
        for volume in volumes:
            if docker(["volume", "inspect", "--format", "{{.Name}}", volume], missing=volume) is not None:
                raise LabError(f"Lab volume survived cleanup: {volume}.")
