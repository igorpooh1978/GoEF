# Apply reviewed migrations to PostgreSQL

Use [migrations.Apply](../../providers/postgres/migration/apply.go) from
`github.com/igorpooh1978/GoEF/providers/postgres/migration` with a
caller-owned pgx pool dedicated to migrations. The executor requires no CLI.
The [runnable fixture](../../test/conformance/driverlab/postgres_migrations_test.go)
creates a schema object, adds one parameterized seed row and verifies the
ordered history and durable data. A complete reapplication returns zero after
verifying the target. [Issue #35](https://github.com/igorpooh1978/GoEF/issues/35)

```bash
python -B test/conformance/lab/postgres_migrations.py
```

The runner provisions a disposable PostgreSQL 18.7 container, checks its
identity and runs Go 1.27.4 with pgx v5.13.0. It also verifies failures, lock
contention and recovery. [Evidence](../evidence/W006/POSTGRES_MIGRATIONS.md)

## Define the plan

- Set the expected database name, the existing schema for history and a
  history table name reserved for GoEF. Use credentials that can create that
  table and apply the reviewed changes. The executor checks the database
  name and quotes the history identifiers. Include any additional target
  identity or schema restrictions in the checks. Use 63 bytes or fewer per
  identifier and omit NUL characters from both identifiers and artifact IDs.
- Assign a stable, nonzero advisory lock key. Use the same key within one
  database for all migrators that share the history or touch conflicting
  objects. Set a positive lock timeout and give the call a context deadline.
- Register the complete ordered lineage. Give each artifact an immutable ID,
  the immediately preceding ID and the SHA-256 digest of its reviewed
  operation and check definitions. The executor compares history as an exact
  prefix of the registry. Supply case-sensitive IDs and append a new artifact
  when correcting a released migration.
- Supply a read-only source check, a transactional run callback and a
  read-only target check for every artifact. Use the supplied session
  synchronously, close every row stream, and propagate query and execution
  errors. Leave transaction control, connection settings and history to the
  executor. Qualify application object names and bind runtime values.

The executor takes an advisory transaction lock on one connection. It then
reads and validates history, checks each pending artifact and persists the
entire suffix and its history in one READ COMMITTED transaction. The next
lock owner reads history after the first transaction ends. The caller owns
the pool and may close it after Apply returns. [PostgreSQL advisory locks](https://www.postgresql.org/docs/18/explicit-locking.html#ADVISORY-LOCKS)

## Handle failures and existing history

| Error matched with errors.Is | Meaning and response |
| --- | --- |
| ErrMigrationConflict | The definitions or the stored lineage differ. Review the immutable artifacts and durable history before continuing. |
| ErrMigrationRecoveryRequired | The identity or an observed source or target check failed. Inspect the live schema and data before writing further migrations. |
| ErrMigrationLocked | The configured migration lock wait expired. Inspect the competing migration and apply again only as an explicit decision. |
| context.Canceled or context.DeadlineExceeded | The caller canceled the operation or exhausted its request budget. Check joined errors, including an unconfirmed rollback or commit. |
| ErrCommitOutcomeUnknown | The server outcome could not be established from the commit response. Inspect the exact history, schema and data through a fresh connection before deciding how to proceed. |

The executor leaves legacy tables in place. Establish a reviewed mapping
from every legacy entry to its immutable artifact and verify the live schema
and data before adopting GoEF history. A matching ID alone supplies no
digest evidence. Entries with missing, altered or unverifiable digests
require a separately reviewed baseline. [Recovery design](MIGRATIONS_UPGRADE_AND_RECOVERY.md)

Use this executor for transactional forward DDL and DML. Independent
recovery for nontransactional DDL such as CREATE INDEX CONCURRENTLY,
automatic baseline import, downgrade execution and PgBouncer verification
remain separate work. [PostgreSQL concurrent index restrictions](https://www.postgresql.org/docs/18/sql-createindex.html#SQL-CREATEINDEX-CONCURRENTLY)
