# GoEF

GoEF is a planned Go data access library that uses Entity Framework Core behavior as a reference. Its scope includes modeling, typed queries, tracking, migrations, and separate providers for PostgreSQL, SQLite, SQL Server, and MySQL. GoLINQ also covers independent in-memory sequences and explicit database-backed queries.

The core module contains the frozen provider execution contract, Code First mapping and scalar type binding for four database families, and the in-memory `Where`, `Select` and `Count` operators. The four-engine smoke lab checks isolated engines. The GoEF query runtime, concrete adapters and supported database releases await later work. [Code First mapping](docs/user/MODELS_CODE_FIRST.md) · [In-memory sequence operators](docs/user/LINQ_OBJECTS.md)

- [Documentation](docs/README.md)
- [Master plan](docs/roadmap/MASTER_PLAN.md)
- [Project checkpoint](docs/roadmap/CHECKPOINT.md)
- [Contributing](CONTRIBUTING.md)
- [MIT License](LICENSE)
- [Bootstrap tracking issue](https://github.com/igorpooh1978/GoEF/issues/1)
