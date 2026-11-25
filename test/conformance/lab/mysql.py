"""Oracle MySQL engine smoke for W002 issue #21."""

import secrets

from runtime import LabError


IMAGE = "docker.io/library/mysql@sha256:f0c7a4430d773ef8433aae82e08cb02ade4a6ad806a18b1610a48a1728fbdfd5"
VERSION = "8.4.13.1"
DATABASE = "goef_smoke"
VARIANT = "MySQL Community Server - GPL"
SESSION = {
    "default_storage_engine": "InnoDB",
    "transaction_isolation": "READ-COMMITTED",
    "autocommit": "1",
    "sql_mode": "STRICT_TRANS_TABLES,NO_ENGINE_SUBSTITUTION",
    "time_zone": "+00:00",
    "collation_connection": "utf8mb4_bin",
}
INIT_SQL = (
    "SET SESSION default_storage_engine = 'InnoDB', "
    "transaction_isolation = 'READ-COMMITTED', autocommit = 1, "
    "sql_mode = 'STRICT_TRANS_TABLES,NO_ENGINE_SUBSTITUTION', "
    "time_zone = '+00:00', collation_connection = 'utf8mb4_bin'"
)

# The image uses a temporary server during bootstrap. Wait for the final
# exec of mysqld as PID 1 before opening the fixture database.
READY = 'test "$(cat /proc/1/comm)" = mysqld && exec "$@"'

# Check both the failed exit and the native error prefix inside the container.
# Do not forward the server's full diagnostic, even for unexpected failures.
DUPLICATE = r"""
if message=$("$@" 2>&1); then
    exit 1
fi
case "$message" in
    'ERROR 1062 (23000) at line '*|'ERROR 1062 (23000): '*) printf '1062\t23000\n' ;;
    *) exit 1 ;;
esac
"""


def smoke(lab) -> dict[str, str]:
    """Validate the isolated MySQL build and fixture behavior with its CLI."""
    fixture_password = secrets.token_hex(24)
    target_token = secrets.token_hex(32)
    setup = {
        "MYSQL_DATABASE": DATABASE,
        "MYSQL_USER": "goef_smoke",
        "MYSQL_PASSWORD": fixture_password,
        "MYSQL_ROOT_PASSWORD": secrets.token_hex(24),
        "MYSQL_ROOT_HOST": "localhost",
    }
    client_env = {"MYSQL_PWD": fixture_password}
    socket_client = [
        "mysql",
        "--no-defaults",
        "--no-login-paths",
        "--skip-reconnect",
        "--protocol=SOCKET",
        "--socket=/var/run/mysqld/mysqld.sock",
        "--connect-timeout=5",
        "--default-character-set=utf8mb4",
        "--batch",
        "--skip-column-names",
        "--binary-mode",
    ]
    client = [*socket_client, "--user=goef_smoke", f"--database={DATABASE}"]
    select_options = ", ".join(f"@@session.{key}" for key in SESSION)
    with lab.container(IMAGE, setup) as container:
        container.wait(
            ["sh", "-c", READY, "mysql-readiness", *client, "--execute=SELECT 1"],
            env=client_env,
        )
        client.append(f"--init-command={INIT_SQL}")
        identity = container.exec(
            client,
            stdin="SELECT VERSION_WITH_UPDATE(), DATABASE(), @@version_comment;",
            env=client_env,
        )
        if identity.strip() != "\t".join((VERSION, DATABASE, VARIANT)):
            raise LabError("MySQL runtime version, database, or variant did not match the pin")
        settings = container.exec(
            client,
            stdin=f"SELECT {select_options}, @@session.character_set_connection;",
            env=client_env,
        )
        if settings.strip() != "\t".join((*SESSION.values(), "utf8mb4")):
            raise LabError("MySQL session settings did not match the smoke profile")

        # The image grants the fixture login privileges only in its own database.
        # Keep the marker in a separate database and grant table-level reads.
        container.exec(
            [*socket_client, "--user=root"],
            stdin=f"""\
CREATE DATABASE goef_marker;
CREATE TABLE goef_marker.target (
    fixture_database VARCHAR(64) PRIMARY KEY,
    token CHAR(64) NOT NULL
) ENGINE=InnoDB;
INSERT INTO goef_marker.target VALUES ('{DATABASE}', '{target_token}');
GRANT SELECT ON goef_marker.target TO 'goef_smoke'@'%';
""",
            env={"MYSQL_PWD": setup["MYSQL_ROOT_PASSWORD"]},
        )
        target = container.exec(
            client,
            stdin="SELECT DATABASE(), token FROM goef_marker.target WHERE fixture_database = DATABASE();",
            env=client_env,
        )
        if target.strip() != f"{DATABASE}\t{target_token}":
            raise LabError("MySQL target marker did not match this run")

        storage = container.exec(
            client,
            stdin="""\
CREATE TABLE smoke_rows (
    id INT PRIMARY KEY,
    label VARCHAR(30) NOT NULL
) ENGINE=InnoDB DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_bin;
SELECT ENGINE, TABLE_COLLATION FROM INFORMATION_SCHEMA.TABLES
WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'smoke_rows';
""",
            env=client_env,
        )
        if storage.strip() != "InnoDB\tutf8mb4_bin":
            raise LabError("MySQL fixture table storage settings did not match")

        container.exec(
            client,
            stdin="""\
START TRANSACTION;
INSERT INTO smoke_rows VALUES (1, 'seed');
COMMIT;
""",
            env=client_env,
        )
        try:
            native = container.exec(
                ["sh", "-c", DUPLICATE, "mysql-duplicate-key", *client],
                stdin="INSERT INTO smoke_rows VALUES (1, 'duplicate');",
                env=client_env,
            )
        except LabError:
            raise LabError("MySQL did not report the expected duplicate-key failure") from None
        if native.strip() != "1062\t23000":
            raise LabError("MySQL duplicate-key evidence did not match")

        pending = container.exec(
            client,
            stdin="""\
START TRANSACTION;
INSERT INTO smoke_rows VALUES (2, 'rollback');
SELECT id, label FROM smoke_rows ORDER BY id;
ROLLBACK;
""",
            env=client_env,
        )
        if pending.strip() != "1\tseed\n2\trollback":
            raise LabError("MySQL rollback probe did not observe the pending row")
        persisted = container.exec(
            client, stdin="SELECT id, label FROM smoke_rows ORDER BY id;", env=client_env
        )
        if persisted.strip() != "1\tseed":
            raise LabError("MySQL committed seed was not the only row after rollback")

    return {
        "image": IMAGE,
        "server_version": VERSION,
        "variant": VARIANT,
        "database": DATABASE,
        "target_marker": "verified",
        "storage_engine": "InnoDB",
        "table_collation": "utf8mb4_bin",
        "character_set_connection": "utf8mb4",
        **SESSION,
        "duplicate_key": "1062/23000",
        "rollback": "committed seed preserved",
    }
