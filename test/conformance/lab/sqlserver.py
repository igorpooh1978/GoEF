"""SQL Server 2022 Developer engine smoke for GoEF issue #22."""

from secrets import token_hex

from runtime import Blocked, LabError


IMAGE = (
    "mcr.microsoft.com/mssql/server"
    "@sha256:4525e3efac9f3e241cb003bb2ff2a82818b502f37e12049d788b1a9fa7e13b7f"
)
DATABASE = "goef_smoke"
SQLCMD = [
    "/opt/mssql-tools18/bin/sqlcmd",
    "-S", "tcp:127.0.0.1,1433",
    "-U", "sa",
    "-C",  # Trust the ephemeral certificate on the private loopback only.
    "-l", "5",
    "-b",
    "-r1",
    "-h", "-1",
    "-W",
    "-s", "|",
]

SERVER_IDENTITY = """
SET NOCOUNT ON;
SELECT CONVERT(varchar(20), SERVERPROPERTY('ProductVersion')),
       CONVERT(varchar(10), SERVERPROPERTY('ProductUpdateLevel')),
       CONVERT(varchar(40), SERVERPROPERTY('Edition'));
"""

DATABASE_GUARD = """
SET NOCOUNT ON;
IF DB_NAME() <> N'goef_smoke' OR DB_ID() <= 4
    THROW 51000, 'Unexpected smoke database', 1;
"""

MARKER_GUARD = DATABASE_GUARD + """
IF NOT EXISTS (
    SELECT 1
    FROM dbo.goef_lab_identity AS marker
    JOIN sys.database_recovery_status AS recovery
      ON recovery.database_guid = marker.database_guid
    WHERE recovery.database_id = DB_ID()
      AND marker.run_token = 0x{run_token}
)
    THROW 51001, 'Smoke database identity does not match', 1;
"""

PROVISION_MARKER = DATABASE_GUARD + """
CREATE TABLE dbo.goef_lab_identity (
    database_guid uniqueidentifier NOT NULL PRIMARY KEY,
    run_token binary(16) NOT NULL
);
INSERT dbo.goef_lab_identity (database_guid, run_token)
SELECT database_guid, 0x{run_token}
FROM sys.database_recovery_status
WHERE database_id = DB_ID();
"""

DATABASE_SETTINGS = MARKER_GUARD + """
SELECT compatibility_level, collation_name,
       snapshot_isolation_state, is_read_committed_snapshot_on
FROM sys.databases
WHERE database_id = DB_ID();
"""

SMOKE_FIXTURE = MARKER_GUARD + """
SET TRANSACTION ISOLATION LEVEL READ COMMITTED;
SET XACT_ABORT ON;

CREATE TABLE dbo.lab_parent (
    id int NOT NULL PRIMARY KEY,
    marker int NOT NULL UNIQUE
);
CREATE TABLE dbo.lab_child (
    id int NOT NULL PRIMARY KEY,
    parent_id int NOT NULL REFERENCES dbo.lab_parent (id)
);
INSERT dbo.lab_parent (id, marker) VALUES (1, 101);
INSERT dbo.lab_child (id, parent_id) VALUES (1, 1);

DECLARE @unique_error int, @foreign_key_error int;
BEGIN TRY
    INSERT dbo.lab_parent (id, marker) VALUES (2, 101);
END TRY
BEGIN CATCH
    IF ERROR_NUMBER() <> 2627
        THROW;
    SET @unique_error = ERROR_NUMBER();
END CATCH;

BEGIN TRY
    INSERT dbo.lab_child (id, parent_id) VALUES (2, 999);
END TRY
BEGIN CATCH
    IF ERROR_NUMBER() <> 547
        THROW;
    SET @foreign_key_error = ERROR_NUMBER();
END CATCH;

IF @unique_error IS NULL OR @foreign_key_error IS NULL
    THROW 51002, 'Fixture constraint accepted an invalid row', 1;

BEGIN TRANSACTION;
INSERT dbo.lab_parent (id, marker) VALUES (3, 103);
INSERT dbo.lab_child (id, parent_id) VALUES (3, 3);
IF XACT_STATE() <> 1
   OR NOT EXISTS (SELECT 1 FROM dbo.lab_parent WHERE id = 3 AND marker = 103)
   OR NOT EXISTS (SELECT 1 FROM dbo.lab_child WHERE id = 3 AND parent_id = 3)
    THROW 51003, 'Transactional fixture rows are missing', 1;
ROLLBACK TRANSACTION;

SELECT @unique_error, @foreign_key_error;
"""

VERIFY_ROLLBACK = MARKER_GUARD + """
IF (SELECT COUNT(*) FROM dbo.lab_parent) <> 1
   OR NOT EXISTS (SELECT 1 FROM dbo.lab_parent WHERE id = 1 AND marker = 101)
   OR (SELECT COUNT(*) FROM dbo.lab_child) <> 1
   OR NOT EXISTS (SELECT 1 FROM dbo.lab_child WHERE id = 1 AND parent_id = 1)
    THROW 51004, 'Persisted fixture rows differ from the committed baseline', 1;
SELECT 'baseline-restored';
"""


def smoke(lab) -> dict[str, str]:
    if not lab.allow_sqlserver:
        raise Blocked("SQL Server Developer EULA acceptance is required for this run")

    password = f"Lab9!{token_hex(24)}"
    # Only internally generated hexadecimal digits enter the SQL binary literal.
    run_token = token_hex(16)
    server_env = {
        "ACCEPT_EULA": "Y",
        "MSSQL_PID": "Developer",
        "MSSQL_SA_PASSWORD": password,
    }
    client_env = {"SQLCMDPASSWORD": password}
    with lab.container(IMAGE, server_env) as container:
        def query(database: str, sql: str) -> str:
            return container.exec(
                [*SQLCMD, "-d", database, "-Q", sql], env=client_env
            ).strip()

        container.wait(
            [*SQLCMD, "-d", "master", "-Q", "SELECT 1;"], env=client_env
        )
        server = query("master", SERVER_IDENTITY)
        if server != "16.0.4325.4|CU28|Developer Edition (64-bit)":
            raise LabError("SQL Server build or Developer edition differs from the pin")
        version, update_level, edition = server.split("|")

        query("master", "CREATE DATABASE [goef_smoke] COLLATE Latin1_General_100_BIN2;")
        query(DATABASE, PROVISION_MARKER.format(run_token=run_token))
        settings = query(DATABASE, DATABASE_SETTINGS.format(run_token=run_token))
        if settings != "160|Latin1_General_100_BIN2|0|0":
            raise LabError("SQL Server smoke database settings differ from the profile")
        compatibility, collation, snapshot, read_committed_snapshot = settings.split("|")

        constraint_codes = query(DATABASE, SMOKE_FIXTURE.format(run_token=run_token))
        if constraint_codes != "2627|547":
            raise LabError("SQL Server fixture returned unexpected constraint codes")

        rollback = query(DATABASE, VERIFY_ROLLBACK.format(run_token=run_token))
        if rollback != "baseline-restored":
            raise LabError("SQL Server rollback verification returned an unexpected result")

    return {
        "image": IMAGE,
        "version": version,
        "product_update_level": update_level,
        "edition": edition,
        "target": DATABASE,
        "compatibility_level": compatibility,
        "collation": collation,
        "snapshot_isolation": snapshot,
        "read_committed_snapshot": read_committed_snapshot,
        "constraint_codes": constraint_codes.replace("|", ","),
        "rollback": rollback,
    }
