# Error codes

Status: Proposed. This reference describes the intended contract. Its usage examples are planned.

Stable IDs belong to the canonical error registry. A Go sentinel or typed error exposes the behavior independently of human-readable messages. Native mapping evidence needs version, phase, disambiguation and safe metadata. Unknown commit and classification are separate concerns.

Feature IDs: ERR-CONTRACT-001, TX-OUTCOME-001.

- [Canonical design or registry](../errors/registry.json)
- [Example acceptance criteria](../../examples/README.md)

## Proposed error IDs

| ID | Go sentinel | Category | Retry classification |
| --- | --- | --- | --- |
| GOEF-CAP-0001 | ErrUnsupportedFeature | capability | never |
| GOEF-CONC-0001 | ErrConcurrencyConflict | concurrency | never |
| GOEF-CONC-0002 | ErrDeadlock | concurrency | transaction_safe |
| GOEF-CONC-0003 | ErrSerializationFailure | concurrency | transaction_safe |
| GOEF-CONC-0004 | ErrLockUnavailable | concurrency | never |
| GOEF-CONS-0001 | ErrUniqueViolation | constraint | never |
| GOEF-CONS-0002 | ErrForeignKeyViolation | constraint | never |
| GOEF-CONS-0003 | ErrNotNullViolation | constraint | never |
| GOEF-CONS-0004 | ErrCheckViolation | constraint | never |
| GOEF-CONS-0005 | ErrValueOutOfRange | constraint | never |
| GOEF-INTERNAL-0001 | ErrInternalInvariant | invariant | never |
| GOEF-IO-0001 | ErrConnectionUnavailable | connection | never |
| GOEF-IO-0002 | ErrTimeout | connection | never |
| GOEF-IO-0003 | ErrCanceled | connection | never |
| GOEF-IO-0004 | ErrDeadlineExceeded | connection | never |
| GOEF-IO-0005 | ErrPermissionDenied | connection | never |
| GOEF-IO-0006 | ErrAuthenticationFailed | connection | never |
| GOEF-IO-0007 | ErrUnknownDriver | connection | never |
| GOEF-MIG-0001 | ErrPendingModelChanges | migration | never |
| GOEF-MIG-0002 | ErrMigrationConflict | migration | never |
| GOEF-MIG-0003 | ErrMigrationLocked | migration | never |
| GOEF-MIG-0004 | ErrSchemaDrift | migration | never |
| GOEF-MIG-0005 | ErrDestructiveMigration | migration | never |
| GOEF-MIG-0006 | ErrMigrationRecoveryRequired | migration | resolve_outcome |
| GOEF-MIG-0007 | ErrIrreversibleMigration | migration | never |
| GOEF-MODEL-0001 | ErrInvalidModel | model | never |
| GOEF-MODEL-0002 | ErrValidation | model | never |
| GOEF-QUERY-0001 | ErrNotFound | query | never |
| GOEF-QUERY-0002 | ErrMultipleResults | query | never |
| GOEF-QUERY-0003 | ErrQueryTranslation | query | never |
| GOEF-QUERY-0004 | ErrInvalidQuery | query | never |
| GOEF-QUERY-0005 | ErrMaterialization | query | never |
| GOEF-STATE-0001 | ErrInvalidState | lifetime | never |
| GOEF-STATE-0002 | ErrContextBusy | lifetime | never |
| GOEF-STATE-0003 | ErrContextClosed | lifetime | never |
| GOEF-TOOL-0001 | ErrDesignTime | tooling | never |
| GOEF-TOOL-0002 | ErrGeneration | tooling | never |
| GOEF-TOOL-0003 | ErrCLIUsage | tooling | never |
| GOEF-TX-0001 | ErrCommitOutcomeUnknown | transaction | resolve_outcome |
| GOEF-TX-0002 | ErrSavepointFailed | transaction | never |
| GOEF-TX-0003 | ErrRollbackFailed | transaction | resolve_outcome |
