"""PostgreSQL 18.7 smoke for the isolated conformance lab (#20)."""

from secrets import token_hex

from runtime import Blocked, LabError


IMAGE = "docker.io/library/postgres@sha256:32f5fdbf4d27ebe0d4552313126a593400f40b21bff207e43617b2305838bce3"
VERSION_NUM = "180007"
DATABASE = "goef_smoke"
TARGET = ["--host=127.0.0.1", "--port=5432", "--username=postgres", f"--dbname={DATABASE}"]
PSQL = [
    "psql", "--no-psqlrc", "--no-password", "--quiet", "--no-align",
    "--tuples-only", "--set=ON_ERROR_STOP=1", *TARGET, "--file=-",
]

PROVISION_MARKER_SQL = r"""
\getenv run_token GOEF_LAB_RUN_TOKEN
BEGIN;
CREATE SCHEMA goef_lab_identity;
CREATE TABLE goef_lab_identity.target (run_token text NOT NULL);
INSERT INTO goef_lab_identity.target (run_token) VALUES ($1)
\bind :run_token
\g
COMMIT;
"""

VERIFY_MARKER_SQL = r"""
\getenv run_token GOEF_LAB_RUN_TOKEN
SELECT run_token = $1 FROM goef_lab_identity.target
\bind :run_token
\g
"""

SEED_SQL = """
BEGIN;
CREATE TABLE public.goef_smoke (
    id integer CONSTRAINT goef_smoke_pkey PRIMARY KEY,
    marker text NOT NULL
);
INSERT INTO public.goef_smoke (id, marker) VALUES (1, 'committed');
COMMIT;
"""

READ_ROWS_SQL = "SELECT id, marker FROM public.goef_smoke ORDER BY id;"

ROLLBACK_SQL = """
BEGIN;
INSERT INTO public.goef_smoke (id, marker) VALUES (2, 'transient');
DO $probe$
DECLARE
    rejected_constraint text;
BEGIN
    INSERT INTO public.goef_smoke (id, marker) VALUES (1, 'duplicate');
    RAISE EXCEPTION 'duplicate primary key was accepted';
EXCEPTION
    WHEN SQLSTATE '23505' THEN
        GET STACKED DIAGNOSTICS rejected_constraint = CONSTRAINT_NAME;
        IF rejected_constraint IS DISTINCT FROM 'goef_smoke_pkey' THEN
            RAISE EXCEPTION 'unexpected unique constraint';
        END IF;
END;
$probe$;
SELECT id, marker FROM public.goef_smoke ORDER BY id;
ROLLBACK;
"""


def smoke(lab) -> dict[str, str]:
    """Check the engine identity, native constraint rejection and rollback."""
    password = token_hex(24)
    server_env = {"POSTGRES_DB": DATABASE, "POSTGRES_PASSWORD": password}
    client_env = {"PGPASSWORD": password, "PGCONNECT_TIMEOUT": "5"}
    with lab.container(IMAGE, server_env) as container:
        # The image's initialization server accepts only Unix-socket connections.
        container.wait(["pg_isready", "--quiet", "--timeout=3", *TARGET])
        identity = container.exec(
            PSQL,
            stdin="SELECT current_setting('server_version_num'), current_database();",
            env=client_env,
        )
        if identity.splitlines() != [f"{VERSION_NUM}|{DATABASE}"]:
            raise Blocked("PostgreSQL server version or database did not match the pinned target")

        marker_env = {**client_env, "GOEF_LAB_RUN_TOKEN": token_hex(24)}
        container.exec(PSQL, stdin=PROVISION_MARKER_SQL, env=marker_env)
        marker = container.exec(PSQL, stdin=VERIFY_MARKER_SQL, env=marker_env)
        if marker.splitlines() != ["t"]:
            raise Blocked("PostgreSQL target marker did not match the current run")

        container.exec(PSQL, stdin=SEED_SQL, env=client_env)
        before = container.exec(PSQL, stdin=READ_ROWS_SQL, env=client_env)
        if before.splitlines() != ["1|committed"]:
            raise LabError("PostgreSQL committed seed did not match the fixture")

        in_transaction = container.exec(PSQL, stdin=ROLLBACK_SQL, env=client_env)
        if in_transaction.splitlines() != ["1|committed", "2|transient"]:
            raise LabError("PostgreSQL transaction did not contain the expected rows")

        after = container.exec(PSQL, stdin=READ_ROWS_SQL, env=client_env)
        if after.splitlines() != ["1|committed"]:
            raise LabError("PostgreSQL rows after rollback did not match the committed seed")

    return {
        "image": IMAGE,
        "version": "18.7",
        "version_num": VERSION_NUM,
        "database": DATABASE,
        "target_marker": "verified",
        "constraint_sqlstate": "23505",
        "constraint": "goef_smoke_pkey",
        "rollback": "committed seed preserved",
    }
