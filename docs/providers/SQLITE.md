# SQLite provider

Current source can [reuse a physical connection after a reviewed read](../../providers/README.md#reuse-after-a-reviewed-read). SQLite keeps state such as temporary objects and some pragmas per connection. Review those effects before opting in. Ordinary SQL retains conservative connection retirement. [SQLite pragmas](https://www.sqlite.org/pragma.html#pragma_cache_size)

Tracking [#8](https://github.com/igorpooh1978/GoEF/issues/8). W000 research cell: SQLite 3.54.1 with `modernc.org/sqlite` v1.63.0. The driver's platform table documents the embedded engine version. Confirm the actual source ID and compile options in W001 before pinning the runtime. Conformance is pending. [Engine release](https://www.sqlite.org/releaselog/3_54_1.html) · [Tagged driver docs](https://pkg.go.dev/modernc.org/sqlite@v1.63.0)

## Candidate capabilities

| Area | Research basis and implementation questions |
| --- | --- |
| Dialect | Research quoted names, schema boundaries across `main` and `temp`, per-connection settings, bind markers, prepared statements, collation/Unicode, and explicit sorting and pagination behavior. Capture runtime variable and SQL size limits. [SQL language](https://www.sqlite.org/lang.html) · [Limits](https://www.sqlite.org/limits.html) |
| Mapping | Type affinity permits conversions and a declaration such as `VARCHAR(255)` does not enforce a length limit. Specify exact round trips for decimals, UUID, binary, integer booleans, JSON text or SQLite JSONB, temporal values and nullable fields, with explicit STRICT-table behavior. [Datatypes](https://www.sqlite.org/datatype3.html) · [STRICT tables](https://www.sqlite.org/stricttables.html) · [JSON](https://www.sqlite.org/json1.html) |
| Model and introspection | Record `INTEGER PRIMARY KEY`, rowid and AUTOINCREMENT constraints, generated columns, defaults, composite keys and FKs. Read actual secondary indexes, triggers and dependent views on the migration connection. Explicitly initialize FK enforcement per connection and check actual value. [AUTOINCREMENT](https://www.sqlite.org/autoinc.html) · [PRAGMAs](https://www.sqlite.org/pragma.html) · [Schema table](https://www.sqlite.org/schematab.html) |
| Transactions and errors | Distinguish DEFERRED, IMMEDIATE and EXCLUSIVE transaction modes and journaling options. Verify that contention fixtures share one file across connections. Capture extended constraints, `SQLITE_BUSY`, `SQLITE_LOCKED`, cancellation and pool reuse. [Transactions](https://www.sqlite.org/lang_transaction.html) · [WAL](https://www.sqlite.org/wal.html) · [Result codes](https://www.sqlite.org/rescode.html) |
| DDL and recovery | General schema changes can require a rebuild. Preserve indexes, triggers and views, copy data and check FK integrity before commit. Foreign-key enforcement changes take effect outside any transaction or savepoint. Restore the original setting and confirm connection cleanup after success and failure. [Generalized ALTER TABLE](https://www.sqlite.org/lang_altertable.html#making_other_kinds_of_table_schema_changes) · [Foreign-key PRAGMA](https://www.sqlite.org/pragma.html#pragma_foreign_keys) |
| Provider operations | Verify upsert and `RETURNING` semantics, actual inserted IDs, row counts, bulk strategy and parameter limits against the embedded driver build. [UPSERT](https://www.sqlite.org/lang_upsert.html) · [RETURNING](https://www.sqlite.org/lang_returning.html) |

## Evidence required

Capture `sqlite_version()`, `sqlite_source_id()`, compile options, driver version, journal mode, busy timeout and actual FK enforcement. Use a file-backed fixture for contention and reopen it to verify persisted schema/data after failure. Test in-memory connection lifetime separately. Link the rebuild to `MIG-SQLITE-001-S01` and transaction behavior to `TX-ISOLATION-001-S01`.

Context7 returned the official SQLite 3.54.1 version constants and source ID, plus relevant PRAGMA semantics. The first rebuild excerpt described an alternate schema-edit path. The documented GoEF research basis is the vendor's generalized rebuild procedure linked above. [Probe evidence](../evidence/W000/MCP_REPORT.md#sqlite)

The W002 SQLite minimum is 3.51.3 with `modernc.org/sqlite` v1.47.0. The local file contract exercises both directions with 3.54.1, on schemas shared by the two versions. It verifies additive migration, rollback, strict generated-column type rejection, and preserved data and metadata in `DELETE` and `WAL` modes. The library-level rebuild and recovery scenarios above retain their own acceptance gates. [Minimum version decision](../evidence/W002/LEGACY_COMPATIBILITY.md#sqlite-minimum-runtime)
