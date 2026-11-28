# MySQL provider

Current source can [reuse a physical connection after a reviewed read](../../providers/README.md#reuse-after-a-reviewed-read). MySQL user-defined variables persist for one session, and ordinary SQL retains conservative connection retirement. [MySQL 8.4 session variables](https://dev.mysql.com/doc/refman/8.4/en/user-variables.html)

Tracking [#11](https://github.com/igorpooh1978/GoEF/issues/11). W000 research cell: Oracle MySQL 8.4.13.1 LTS with InnoDB and `github.com/go-sql-driver/mysql` v1.10.0. W001 must confirm the exact artifact, digest and settings. Conformance is pending. [Engine release](https://dev.mysql.com/doc/relnotes/mysql/8.4/en/news-8-4-13-1.html) · [Driver README at v1.10.0](https://github.com/go-sql-driver/mysql/blob/v1.10.0/README.md)

## Candidate capabilities

| Area | Research basis and implementation questions |
| --- | --- |
| Dialect | Use backtick quoting and bind values with `?`. Record `lower_case_table_names`, SQL mode, collation and `utf8mb4` settings. Verify identifier lengths, Unicode, NULL order, deterministic pagination and parameter limits on the chosen build. [Identifier names](https://dev.mysql.com/doc/refman/8.4/en/identifiers.html) · [Identifier case](https://dev.mysql.com/doc/refman/8.4/en/identifier-case-sensitivity.html) |
| Mapping | Specify decimal precision, `BINARY(16)` or `CHAR(36)` UUID codec, native JSON, binary, integer-backed boolean and temporal mappings. Prove `DATETIME(6)` precision, location handling, NULL and zero-date handling against driver settings. [Data types](https://dev.mysql.com/doc/refman/8.4/en/data-types.html) · [Driver README](https://github.com/go-sql-driver/mysql/blob/v1.10.0/README.md) |
| Model and introspection | Capture schemas, `AUTO_INCREMENT`, generated columns, checks, default expressions, composite keys, foreign keys and cascades, indexes, views and triggers. Verify information schema visibility under the intended identity. [CREATE TABLE](https://dev.mysql.com/doc/refman/8.4/en/create-table.html) · [INFORMATION_SCHEMA](https://dev.mysql.com/doc/refman/8.4/en/information-schema.html) |
| Transactions and errors | Confirm InnoDB isolation, consistent reads, savepoints, next-key locking and metadata locks where applicable. Test rollback scope after lock wait or deadlock, physical session affinity, cancellation and reconnect behavior with driver typed errors. [InnoDB locking](https://dev.mysql.com/doc/refman/8.4/en/innodb-locking.html) · [Server error reference](https://dev.mysql.com/doc/mysql-errors/8.4/en/server-error-reference.html) |
| DDL and recovery | Many DDL statements commit an active transaction. Atomic DDL applies to supported individual operations. Design explicit migration execution and recovery for sequences of operations, verifying persisted schema and history after a mid-migration failure. [Implicit commits](https://dev.mysql.com/doc/refman/8.4/en/implicit-commit.html) · [Atomic DDL](https://dev.mysql.com/doc/refman/8.4/en/atomic-ddl.html) |
| Provider operations | Evaluate `ON DUPLICATE KEY UPDATE` against conflicting unique keys, generated IDs on the same connection, affected rows and any driver-specific flags, bulk restrictions and prepared-statement behavior. [Upsert](https://dev.mysql.com/doc/refman/8.4/en/insert-on-duplicate.html) · [Driver README](https://github.com/go-sql-driver/mysql/blob/v1.10.0/README.md) |

## Evidence required

Capture `VERSION_WITH_UPDATE()`, version comment, storage engine, `sql_mode`, isolation, autocommit, time zone, collation, `lower_case_table_names` and driver version. Record row, schema and history evidence through failure and restart. Link DDL boundaries to `MIG-MYSQL-001-S01` and isolation to `TX-ISOLATION-001-S01`. [Exact version function](https://dev.mysql.com/doc/refman/8.4/en/information-functions.html#function_version-with-update)

A standalone Firecrawl MCP check returned the exact Oracle 8.4.13.1 release note and the applicable MySQL 8.4 sections on implicit DDL commits and InnoDB isolation. Tagged vendor source independently corroborates the release identity. [Exact-release evidence](../evidence/W000/MCP_EXACT_RELEASES.md) · [Tagged source check](../evidence/W000/MCP_MYSQL_RECHECK.md)
