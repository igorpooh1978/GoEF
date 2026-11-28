# Microsoft SQL Server provider

Current source can [reuse a physical connection after a reviewed read](../../providers/README.md#reuse-after-a-reviewed-read). Consider connection-scoped effects such as session context when reviewing a command. Ordinary SQL retains conservative connection retirement. [SQL Server session context](https://learn.microsoft.com/en-us/sql/relational-databases/system-stored-procedures/sp-set-session-context-transact-sql?view=sql-server-ver16)

Tracking [#6](https://github.com/igorpooh1978/GoEF/issues/6). W000 research cell: SQL Server 2022 CU28 (16.0.4325.4) with `github.com/microsoft/go-mssqldb` v1.11.0. W001 must confirm an eligible runner, edition, compatibility level, installation or image, and driver configuration. Conformance is pending. [SQL Server builds](https://learn.microsoft.com/en-us/troubleshoot/sql/releases/sqlserver-2022/build-versions) · [Driver README at v1.11.0](https://github.com/microsoft/go-mssqldb/blob/v1.11.0/README.md)

## Candidate capabilities

| Area | Research basis and implementation questions |
| --- | --- |
| Dialect | Use delimited identifiers and schema qualification, respect identifier length and database collation, and research ordered `OFFSET/FETCH` and NULL sorting. Bind through the driver with named parameters such as `@p1`. [Identifiers](https://learn.microsoft.com/en-us/sql/relational-databases/databases/database-identifiers?view=sql-server-ver16) · [ORDER BY](https://learn.microsoft.com/en-us/sql/t-sql/queries/select-order-by-clause-transact-sql?view=sql-server-ver16) |
| Mapping | Research `uniqueidentifier` and byte order, `bit`, `decimal`, `datetime2`, `datetimeoffset`, `varbinary`, `rowversion`, and JSON text and function capabilities for 16.x. Test nullable and Unicode fields, precision, offset handling and unsupported array/mapping requests. [Data types](https://learn.microsoft.com/en-us/sql/t-sql/data-types/data-types-transact-sql?view=sql-server-ver16) |
| Model and introspection | Capture identities, computed columns, default and check constraints, composite FKs, indexes, views, triggers and metadata visibility. Check `OUTPUT` and generated value behavior for tables with triggers. [Catalog views](https://learn.microsoft.com/en-us/sql/relational-databases/system-catalog-views/catalog-views-transact-sql?view=sql-server-ver16) · [OUTPUT](https://learn.microsoft.com/en-us/sql/t-sql/queries/output-clause-transact-sql?view=sql-server-ver16) |
| Transactions and errors | Test configured isolation including row versioning when enabled, lock timeout and deadlock victim behavior, `XACT_ABORT` and `XACT_STATE()`. Local savepoints have case-sensitive names up to 32 characters. Distributed transactions lack `SAVE TRANSACTION` support. Gate MARS interactions on tested behavior. [SAVE TRANSACTION](https://learn.microsoft.com/en-us/sql/t-sql/language-elements/save-transaction-transact-sql?view=sql-server-ver16) · [XACT_STATE](https://learn.microsoft.com/en-us/sql/t-sql/functions/xact-state-transact-sql?view=sql-server-ver16) |
| DDL and recovery | Test DDL atomicity per operation. Prove dependency ordering for default constraints, identity/computed column changes, data copy, migration history and locks on the correct session. [ALTER TABLE](https://learn.microsoft.com/en-us/sql/t-sql/statements/alter-table-transact-sql?view=sql-server-ver16) |
| Provider operations | Verify actual parameter ceilings, prepared statements, row counts, bulk copy, generated keys, and concurrency semantics for any upsert translation. [Capacity specifications](https://learn.microsoft.com/en-us/sql/sql-server/maximum-capacity-specifications-for-sql-server?view=sql-server-ver16) · [Driver README](https://github.com/microsoft/go-mssqldb/blob/v1.11.0/README.md) |

## Evidence required

Record `SERVERPROPERTY('ProductVersion')`, `ProductUpdateLevel`, edition, database compatibility, collation, isolation settings, runner architecture and driver version. Exercise savepoint rollback, failure with an uncommittable transaction, timeout/cancel cleanup, migration failure after schema changes, and shared scenarios `TX-ISOLATION-001-S01` and `TX-RETRY-001-S01`.

Microsoft Learn MCP returned the SQL Server 2022 build table and the SQL Server 16.x savepoint reference. The smoke confirms the released build identity and applicable T-SQL documentation. [Probe evidence](../evidence/W000/MCP_REPORT.md#sql-server)

The W002 local driver suite passed commit and rollback and the uniqueness and foreign-key cases on Developer builds 14.0.3570.2 at compatibility level 140, 15.0.4510.2 at level 150 and 16.0.4325.4 at level 160. Each used go-mssqldb v1.11.0. The remaining provider and application cases above require their own evidence. [Full local run](../evidence/W002/LEGACY_COMPATIBILITY.md#complete-run-on-2026-11-26)

The same SQL Server 2022 build passed the driver scenarios at level 140. The fixture observed native error `208` for `GENERATE_SERIES` at level 140 and verified the returned values at level 160. [Compatibility evidence](../evidence/W002/LEGACY_COMPATIBILITY.md#postgresql-19-and-sql-server-2022-at-level-140)
