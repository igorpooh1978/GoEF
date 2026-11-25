# Example catalog

All GoEF examples are planned. The isolated Go language feasibility probe demonstrates compiler behavior only.

| Planned runnable example | Scope | Required acceptance |
| --- | --- | --- |
| Code First customer model | [Quickstart](../docs/user/QUICKSTART.md), MODEL-MAP-001 | Compiles and preserves model and rows on all four engines |
| Deferred in-memory order selection | [GoLINQ Objects](../docs/user/LINQ_OBJECTS.md), LINQ-OBJECTS-001 | Compiles without database configuration and enumerates expected values |
| Database query then explicit client computation | [Client/server boundary](../docs/user/LINQ_CLIENT_SERVER_BOUNDARY.md), LINQ-BOUNDARY-001 | Records bounded server result and explicit client transition |
| Caller-owned transaction and rollback | [Transactions](../docs/user/TRANSACTIONS.md), TX-OWN-001 | Persists or rolls back rows exactly as the caller requests |
| Reviewed migration in Go | [Migrations in Go](../docs/user/MIGRATIONS_IN_GO.md), MIG-API-001 | Preserves existing data and applies the migration exactly once |
| Safe handling of portable errors | [Error handling](../docs/user/ERROR_HANDLING.md), ERR-CONTRACT-001 | Shows known and unknown errors without leaking inputs |

A runnable example needs canonical Go source, imports, commands, expected output, isolated test data, and recorded execution. See the [documentation policy](../docs/development/DOCUMENTATION_POLICY.md).
