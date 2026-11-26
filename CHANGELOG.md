# Changelog

## Unreleased

- Set SQLite 3.51.3 as the W002 minimum for the pinned Go driver and verify strict generated-column errors during bidirectional file exchange with 3.54.1 ([#31](https://github.com/igorpooh1978/GoEF/issues/31)).
- Add PostgreSQL 19 and SQL Server 2022 at database compatibility level 140 to the local matrix. Check `GENERATE_SERIES` at levels 140 and 160 on the same SQL Server build ([#31](https://github.com/igorpooh1978/GoEF/issues/31)).
- Extend the local compatibility suite with PostgreSQL 17, SQL Server 2017 and 2019, and bidirectional SQLite 3.51.3 and 3.54.1 file checks ([#31](https://github.com/igorpooh1978/GoEF/issues/31)).
- Pin the four W002 Go drivers in an isolated test module and run native Gherkin characterization in local CI. Add PostgreSQL 16 and MySQL 8.0 coverage and record lower-version candidates for SQL Server and SQLite ([#13](https://github.com/igorpooh1978/GoEF/issues/13), [#26](https://github.com/igorpooh1978/GoEF/issues/26), [#31](https://github.com/igorpooh1978/GoEF/issues/31)).

- Freeze the driver-free provider execution SPI v1 and add an independent external Go fixture ([#12](https://github.com/igorpooh1978/GoEF/issues/12)).
- Implement the disposable four-engine CLI smoke lab with strict artifact identity, rollback and constraint checks ([#13](https://github.com/igorpooh1978/GoEF/issues/13), [#19](https://github.com/igorpooh1978/GoEF/issues/19)).

- Describe the W001 module and public contracts and verify typed GoLINQ query compilation in an isolated module. Track the work in [#12](https://github.com/igorpooh1978/GoEF/issues/12) and [#17](https://github.com/igorpooh1978/GoEF/issues/17).
- Recheck PostgreSQL and MySQL documentation sources, audit the published W000 baseline, and design the isolated four-engine lab. Track the work in [#10](https://github.com/igorpooh1978/GoEF/issues/10), [#11](https://github.com/igorpooh1978/GoEF/issues/11), [#18](https://github.com/igorpooh1978/GoEF/issues/18), and [#16](https://github.com/igorpooh1978/GoEF/issues/16).

- Establish the GoEF and GoLINQ bootstrap contracts, proposed guides, source evidence, and acceptance workflow. Track the work in [issue #1](https://github.com/igorpooh1978/GoEF/issues/1).
