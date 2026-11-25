# PostgreSQL provider

Tracking [#10](https://github.com/igorpooh1978/GoEF/issues/10). W000 research cell: PostgreSQL 18.7 with `github.com/jackc/pgx/v5` v5.13.0. Select the adapter mode, CI image digest and server settings in W001. Conformance is pending. [Engine release](https://www.postgresql.org/docs/release/18.7/) · [Driver README at v5.13.0](https://github.com/jackc/pgx/blob/v5.13.0/README.md)

## Candidate capabilities

| Area | Research basis and implementation questions |
| --- | --- |
| Dialect | Quoted identifiers retain case, unquoted names fold to lower case, and identifiers have a 63-byte limit with the standard build. Bind values with ordinal placeholders such as `$1`. Qualify schemas deliberately and test search path, collation, Unicode, deterministic pagination and explicit NULL order. [Lexical structure](https://www.postgresql.org/docs/18/sql-syntax-lexical.html#SQL-SYNTAX-IDENTIFIERS) · [Sorting](https://www.postgresql.org/docs/18/queries-order.html) |
| Mapping | Research `uuid`, `numeric`, `boolean`, `bytea`, `jsonb`, arrays and `timestamptz` codecs, including offsets on input, precision, NULL and custom type OIDs. Pin the mapping under native pgx or its `database/sql` adapter. [Data types](https://www.postgresql.org/docs/18/datatype.html) · [Driver source](https://github.com/jackc/pgx/tree/v5.13.0/pgtype) |
| Model and introspection | Capture schema-qualified names, identity vs explicit sequence/default, generated columns, composite keys, FK actions, indexes, views and triggers. Check visibility under the intended low-privilege identity. [Information schema](https://www.postgresql.org/docs/18/information-schema.html) · [System catalogs](https://www.postgresql.org/docs/18/catalogs.html) |
| Transactions and errors | Test READ COMMITTED, REPEATABLE READ and SERIALIZABLE, savepoints, aborted transaction cleanup and the connection affinity of advisory locks. Classify serialization failure and deadlock by SQLSTATE `40001` and `40P01`, respectively. Extract native detail from the driver's typed error and test cleanup after cancellation. [Isolation](https://www.postgresql.org/docs/18/transaction-iso.html) · [Advisory locks](https://www.postgresql.org/docs/18/explicit-locking.html#ADVISORY-LOCKS) · [Error codes](https://www.postgresql.org/docs/18/errcodes-appendix.html) |
| DDL and recovery | The vendor requires `CREATE INDEX CONCURRENTLY` outside a transaction block. Model it as an explicit execution boundary and test recovery after interruption, including invalid indexes and migration history. Check statement-level atomicity and compare observed schema and rows. [Building indexes concurrently](https://www.postgresql.org/docs/18/sql-createindex.html#SQL-CREATEINDEX-CONCURRENTLY) |
| Provider operations | Evaluate `RETURNING`, `ON CONFLICT`, native bulk copy, bind limits and prepared statement behavior. Prove pool/proxy mode compatibility before listing any such mode as supported. [INSERT](https://www.postgresql.org/docs/18/sql-insert.html) · [COPY](https://www.postgresql.org/docs/18/sql-copy.html) · [Driver README](https://github.com/jackc/pgx/blob/v5.13.0/README.md) |

## Evidence required

Record the actual server version, driver version, runner, image digest, schema/search path, collation, time zone and connection mode. Verify concurrent DDL failure/recovery, generated values, and transaction behavior on the same physical connection. Run the shared transaction isolation and retry scenarios `TX-ISOLATION-001-S01` and `TX-RETRY-001-S01`.

Context7 returned PostgreSQL 18 documentation for concurrent indexes. Its release index was stale for the 18.7 candidate. Use exact official release notes alongside the versioned manual and keep [#10](https://github.com/igorpooh1978/GoEF/issues/10) open for exact MCP provenance. [Probe evidence](../evidence/W000/MCP_REPORT.md#postgresql)
