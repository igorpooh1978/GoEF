# Production execution adapters

[Issue #13](https://github.com/igorpooh1978/GoEF/issues/13) · [Application scenarios](../../../specs/features/providers/provider-exec-001.feature) · [Driver module](../../../test/conformance/driverlab/README.md) · [Adapter entry points](../../../providers/README.md)

Each provider module implements the frozen execution transport over its pinned native driver. The root Go module shares connection, stream, and type handling through the standard library. The isolated test program uses the same driver factories and engine identity checks as the W002 native characterization. It binds the production adapter to each validated pool.

The application suite checks parameter binding and typed row iteration, unique and foreign-key failures, stream and queued pool cancellation, and an owned transaction with a commit-gated receipt. It checks durable rows through a separate observer or adapter lease. SQL Server code 547 remains unclassified at this boundary because it can represent both foreign-key and check violations. The safe error retains the native code. SQL Server enables savepoint recovery for the successful save path by setting `XACT_ABORT OFF` inside that isolated fixture.

The runner reports coverage for the selected adapter module and the shared transport for each live profile. Each module also has separate API and native error classification tests. The initial executables were characterized by raw Go driver and test-adapter probes. The production suite requires the concrete constructors, and the validator rejects missing, duplicate, or mislabeled results. See the [native driver report](DRIVER_GHERKIN.md) and [coverage policy](../../development/COVERAGE.md).

## Scope

The application suite exercises the SPI v1 command and row transport, using file-backed SQLite. Other native mapping branches may have unit and characterization evidence. A separate feature remains responsible for SQL translation, automatic tracking, and provider-native advanced migration or transaction scenarios. The application receipt exercise coordinates the existing explicit save API. A lost commit response is covered by the [authoritative reconciliation fixtures](COMMIT_RECONCILIATION.md). Process exit during SQLite rebuild is covered by the [migration executor fixture](SQLITE_EXECUTOR_REBUILD.md).
