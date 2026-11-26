# Model engine

Proposed contract for the first model fixtures. Code First consumes explicit registrations, struct tags and fluent mappings. Database First interprets inspected columns, constraints, indexes and provider annotations. Model First consumes declarations. Each produces one logical model and a separate physical mapping for a particular provider. A scaffold writes generated files deterministically and keeps user-owned files separate. [W001 contract fixtures](https://github.com/igorpooh1978/GoEF/issues/12)

| Feature | Input and rule | First acceptance example |
| --- | --- | --- |
| MODEL-MAP-001 | Resolve each facet independently. Fluent configuration overrides tags, which override conventions. Conflicting declarations at equal precedence report both source locations. | A fluent column name changes only that facet while a tag-supplied length survives. Duplicate mapped columns fail unless a supported shared-table mapping is declared. |
| MODEL-NULL-001 | Track store nullability, Go representation, defaults and generated values independently. Nullable pointers or an explicit optional codec preserve presence. | Round-trip NULL, zero and empty string as distinct inputs when the chosen provider and codec support those distinctions. Reject a required foreign key whose read codec admits NULL without an explicit policy. |
| MODEL-REL-001 | Declare principal/dependent, ordered key members, cardinality, optionality, inverse navigation and delete behavior. Check alternate/composite key counts, order and codec compatibility. | Resolve a two-column foreign key to the declared composite key. Reject an ambiguous inverse or a type mismatch before opening a connection. |
| MODEL-TYPE-001 | Map logical scalars and value objects through explicit converters, comparers, snapshot strategy and provider codecs. Configure precision, scale, time policy, collation, UUID layout and generation per property/model. | Decimal and timestamp values must satisfy a documented round-trip policy. Reject an unrepresentable mapping or require an explicit conversion with documented loss. |
| MODEL-VALID-001 | Validate graph, unique names and key identities, generated values, concurrency tokens, mappings and capabilities before publishing immutable metadata. | Reordered registrations produce identical stable IDs, serialized bytes and logical hash. Invalid input yields a source-located error without a partial cached model. |

## Available Code First mapping

`model.Map` resolves the table name and direct exported fields of explicitly registered Go structs. Entity and field names supply defaults. Property tags and fluent options override individual facets. Matching is case-sensitive. Duplicate effective tables or columns and contradictory settings at one precedence yield a typed error with both declaration positions.

The immutable result records ordered primary keys and direct pointer nullability. The version 1 mapping snapshot sorts entities by exact name and preserves field and key-member order. This snapshot describes resolved mapping metadata for the Code First slice. Provider-specific types use the physical binding below. Quoting, shared-table mapping, schema inspection and source generation have separate acceptance criteria. [Code First guide](../user/MODELS_CODE_FIRST.md) · [Mapping evidence](../evidence/W003/MODEL_MAPPING.md)

## Available physical scalar binding

`model.BindTypes` binds the Code First model to one explicit PostgreSQL, SQLite, SQL Server or MySQL dialect. The initial binding covers built-in Boolean, signed integer, text and binary fields and their direct nullable pointer forms. It adds the native type declaration, logical scalar kind and original length to each physical property. The physical snapshot has its own format version and dialect identity. Code First mapping snapshot v1 keeps its naming and key format. [Binding guide](../user/MODELS_CODE_FIRST.md#bind-the-mapped-model-to-a-provider) · [Binding evidence](../evidence/W003/PROVIDER_TYPE_BINDING.md)

Value conversion, driver selection, physical key/index validation, table layout, capability negotiation, collations and source generation continue in their respective stages. Unsupported types fail before I/O with a typed error whose coordinates reflect canonical entity and mapped property order. Exact decimal retains the dedicated S01 scenario.



## Freeze and resolve

1. Collect declarations with provenance. Conventions resolve only remaining facets. Preserve provider extension annotations in a versioned namespace.
2. Validate the logical graph. Self-references are legal relationships. Diagnose cycles that violate a specific rule, such as an unresolved required insertion dependency. Keyless read models cannot enter a tracked identity map.
3. Assign stable identifiers by canonical model identity and ordered member paths. Specify a versioned encoding with sorted unordered lists and explicit ordering for composite keys. Hash the canonical serialization. Read-only published metadata must be independent of caller-owned slices and maps.
4. Resolve schema, store type, length limits, quoting policy, identity strategy, converters and constraints against provider capabilities. Cache the resolved mapping by logical hash, provider identity, capabilities and model-affecting options. Unknown namespaced annotations yield a diagnostic before execution.
5. Derive typed field/navigation descriptors, query materialization and migration snapshots from these same identities. The physical snapshot additionally records provider identity, mapping format and the resolved physical facets.

Many-to-many joins, complex/owned types, JSON, views, inheritance strategies and other EF extensions need separate fixtures. For inheritance, specify composition, discriminator and permitted concrete mappings. Preserve existing keys, including historical UUIDs. Generation strategy belongs to the model.

EF’s `FinalizeModel` and `ModelValidator.Validate` provide distinct freeze and validation landmarks. The test `Detects_shadow_key_referenced_by_foreign_key_by_convention` captures one relationship validation case to adapt. [Model finalization](https://github.com/dotnet/efcore/blob/b34c0e9d7e768aa808ded44f19e7f871bea6869f/src/EFCore/Metadata/Internal/Model.cs) · [Model validation](https://github.com/dotnet/efcore/blob/b34c0e9d7e768aa808ded44f19e7f871bea6869f/src/EFCore/Infrastructure/ModelValidator.cs) · [Pinned validation tests](https://github.com/dotnet/efcore/blob/b34c0e9d7e768aa808ded44f19e7f871bea6869f/test/EFCore.Tests/Infrastructure/ModelValidatorTest.cs)
