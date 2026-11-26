# Changelog

## Unreleased

- Add ordered `SaveChanges` batches for owned and caller-owned transactions. Failed batches roll back to a savepoint when recovery is confirmed, and generated values are accepted after commit ([#2](https://github.com/igorpooh1978/GoEF/issues/2)).
- Add managed savepoints and non-owning participation in caller transactions ([#2](https://github.com/igorpooh1978/GoEF/issues/2)).
- Add owned transactions on one physical connection with explicit begin, commit, rollback, and pool return ([#2](https://github.com/igorpooh1978/GoEF/issues/2)).
- Bind Go booleans, signed integers, strings and byte slices to physical types for all four database families ([#3](https://github.com/igorpooh1978/GoEF/issues/3)).
- Add a terminal `Count` for in-memory iterators and verify complete traversal through `Where` and `Select` ([#4](https://github.com/igorpooh1978/GoEF/issues/4)).
- Resolve Code First table and property mappings with independent column name and length precedence and deterministic metadata ([#3](https://github.com/igorpooh1978/GoEF/issues/3)).
- Add lazy, type-changing `Select` and verify that `Where` followed by `Select` preserves order and stops upstream when the consumer stops ([#4](https://github.com/igorpooh1978/GoEF/issues/4)).
- Adopt the MIT License for GoEF with copyright attributed to Igor Pukhlikov ([#15](https://github.com/igorpooh1978/GoEF/issues/15)).
- Add validation of resolved model declarations with typed invalid-model errors and a lazy in-memory `Where` over `iter.Seq` ([#3](https://github.com/igorpooh1978/GoEF/issues/3), [#4](https://github.com/igorpooh1978/GoEF/issues/4)).
- Add focused local checks for native row cancellation on four PostgreSQL profiles and InnoDB deadlock rollback and recovery on both MySQL profiles. Verify blocked-read cancellation on three additional SQL Server profiles and label SQL Server provisioning failures by stage ([#13](https://github.com/igorpooh1978/GoEF/issues/13), [#26](https://github.com/igorpooh1978/GoEF/issues/26)).
- Verify cancellation in a full connection pool and recovery on every local driver profile ([#13](https://github.com/igorpooh1978/GoEF/issues/13)).
- Record blocked-read cancellation while opening queries on PostgreSQL 18.7 and SQL Server 2022, and during native row iteration on PostgreSQL 18.7 ([#13](https://github.com/igorpooh1978/GoEF/issues/13)).
- Verify already-ended query contexts stop before dispatch and a live query succeeds on the same test lease across the local driver matrix ([#13](https://github.com/igorpooh1978/GoEF/issues/13)).
- Observe blocked writes in the local driver matrix, distinguish caller deadlines from native lock timeouts, and verify recovery and the original rows ([#13](https://github.com/igorpooh1978/GoEF/issues/13)).
- Record a PostgreSQL 18.7 left-join reference run that compares EF Core and native pgx results, parameters, and command counts ([#13](https://github.com/igorpooh1978/GoEF/issues/13)).
- Exercise cancellation and connection release through the provider execution interface on the local driver matrix ([#13](https://github.com/igorpooh1978/GoEF/issues/13)).
- Set SQLite 3.51.3 as the W002 minimum for the pinned Go driver and verify strict generated-column errors during bidirectional file exchange with 3.54.1 ([#31](https://github.com/igorpooh1978/GoEF/issues/31)).
- Add PostgreSQL 19 and SQL Server 2022 at database compatibility level 140 to the local matrix. Check `GENERATE_SERIES` at levels 140 and 160 on the same SQL Server build ([#31](https://github.com/igorpooh1978/GoEF/issues/31)).
- Extend the local compatibility suite with PostgreSQL 17, SQL Server 2017 and 2019, and bidirectional SQLite 3.51.3 and 3.54.1 file checks ([#31](https://github.com/igorpooh1978/GoEF/issues/31)).
- Pin the four W002 Go drivers in an isolated test module and run native Gherkin characterization in local CI. Add PostgreSQL 16 and MySQL 8.0 coverage and record lower-version candidates for SQL Server and SQLite ([#13](https://github.com/igorpooh1978/GoEF/issues/13), [#26](https://github.com/igorpooh1978/GoEF/issues/26), [#31](https://github.com/igorpooh1978/GoEF/issues/31)).

- Freeze the driver-free provider execution SPI v1 and add an independent external Go fixture ([#12](https://github.com/igorpooh1978/GoEF/issues/12)).
- Implement the disposable four-engine CLI smoke lab with strict artifact identity, rollback and constraint checks ([#13](https://github.com/igorpooh1978/GoEF/issues/13), [#19](https://github.com/igorpooh1978/GoEF/issues/19)).

- Describe the W001 module and public contracts and verify typed GoLINQ query compilation in an isolated module. Track the work in [#12](https://github.com/igorpooh1978/GoEF/issues/12) and [#17](https://github.com/igorpooh1978/GoEF/issues/17).
- Recheck PostgreSQL and MySQL documentation sources, audit the published W000 baseline, and design the isolated four-engine lab. Track the work in [#10](https://github.com/igorpooh1978/GoEF/issues/10), [#11](https://github.com/igorpooh1978/GoEF/issues/11), [#18](https://github.com/igorpooh1978/GoEF/issues/18), and [#16](https://github.com/igorpooh1978/GoEF/issues/16).

- Establish the GoEF and GoLINQ bootstrap contracts, proposed guides, source evidence, and acceptance workflow. Track the work in [issue #1](https://github.com/igorpooh1978/GoEF/issues/1).
