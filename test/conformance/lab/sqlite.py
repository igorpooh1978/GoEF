"""On-disk SQLite CLI smoke for GoEF issue #23."""

from runtime import Blocked, LabError

VERSION = "3.54.1"
SOURCE_ID = (
    "2026-10-16 14:45:20 "
    "555e07ef33f69a0957d24d8e555b702464a131d6f1607f4c3c1e4551e8e6ca76"
)

SEED = """
PRAGMA foreign_keys = ON;
PRAGMA foreign_keys;
BEGIN IMMEDIATE;
CREATE TABLE parent (id INTEGER PRIMARY KEY);
CREATE TABLE child (
    id INTEGER PRIMARY KEY,
    parent_id INTEGER NOT NULL REFERENCES parent (id)
);
INSERT INTO parent (id) VALUES (1);
INSERT INTO child (id, parent_id) VALUES (1, 1);
COMMIT;
"""

BEGIN_WRITE = """
PRAGMA foreign_keys = ON;
PRAGMA foreign_keys;
BEGIN IMMEDIATE;
INSERT INTO child (id, parent_id) VALUES (2, 1);
"""

VERIFY = """
PRAGMA foreign_keys = ON;
PRAGMA foreign_keys;
PRAGMA journal_mode;
SELECT id FROM parent ORDER BY id;
SELECT id, parent_id FROM child ORDER BY id;
"""


def smoke(lab) -> dict[str, str]:
    """Verify the pinned CLI and exercise fresh files in both journal modes."""
    if lab.sqlite_binary is None:
        raise Blocked("SQLite requires an explicit trusted 3.54.1 CLI binary")

    cli = [str(lab.sqlite_binary), "-batch", "-bail", "-noinit", "-safe", "-list", "-noheader"]
    identity = lab.local(
        [*cli, "--", ":memory:"],
        stdin="SELECT sqlite_version(), sqlite_source_id();\n",
    ).strip()
    if identity != f"{VERSION}|{SOURCE_ID}":
        raise LabError("SQLite CLI version or source ID differs from the lab pin")

    for journal in ("delete", "wal"):
        directory = lab.tempdir / f"sqlite-{journal}"
        directory.mkdir()
        _check_file(lab, cli, directory / "fixture.db", journal)

    return {
        "version": VERSION,
        "source_id": SOURCE_ID,
        "journal_modes": "delete, wal",
        "foreign_keys": "valid child accepted, orphan rejected by foreign key constraint",
        "rollback": "explicit rollback and failed transaction verified against persisted seed",
    }


def _check_file(lab, cli, database, journal):
    # The caller selects the journal name from the two fixed profiles above.
    setup = lab.local(
        [*cli, "--", str(database)],
        stdin=f"PRAGMA journal_mode = {journal};\n{SEED}",
    ).splitlines()
    if setup != [journal, "1"]:
        raise LabError(f"SQLite {journal} journal mode or foreign keys did not take effect")

    writer = [*cli, "-ifexists", "--", str(database)]
    control = lab.local(
        writer,
        stdin=BEGIN_WRITE + """
INSERT INTO child (id, parent_id) VALUES (3, 1);
SELECT count(*) FROM child;
ROLLBACK;
SELECT count(*) FROM child;
""",
    ).splitlines()
    if control != ["1", "3", "1"]:
        raise LabError(f"SQLite {journal} explicit rollback did not restore the seed")

    # Repeat the write with an orphan as the second insert. Bail exits before
    # COMMIT. The previous successful insert must roll back when the CLI closes.
    lab.local(
        writer,
        stdin=BEGIN_WRITE + """
INSERT INTO child (id, parent_id) VALUES (3, 999);
COMMIT;
""",
        expected_failure=(1, ": FOREIGN KEY constraint failed"),
    )

    stored = lab.local([*cli, "-readonly", "--", str(database)], stdin=VERIFY).splitlines()
    if stored != ["1", journal, "1", "1|1"]:
        raise LabError(f"SQLite {journal} reopened database did not match the seed and settings")
