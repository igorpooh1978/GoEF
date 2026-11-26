# Model metadata

Status: Proposed. This reference describes the intended contract. Its usage examples are planned.

The intended tags name storage metadata such as columns and keys. Their syntax and precedence require an accepted Go fixture. Missing or contradictory keys fail before I/O. Model validation distinguishes nullability, precision, identity and conversion; the provider derives physical type names from a reviewed capability.

## Decimal precision and scale

Place `model.DecimalPrecision(field, precision, scale)` among the fluent options passed to `model.Struct`. The field can be `model.Decimal` or `*model.Decimal`. `model.Map` records the declared precision and scale, including zero scale. `model.BindTypes` supplies the provider declaration and a codec accessible through `model.BoundProperty.DecimalBinding`. See the [Code First example and validation rules](../user/MODELS_CODE_FIRST.md#bind-a-decimal-struct-field).

Feature IDs: MODEL-MAP-001, MODEL-NULL-001, MODEL-TYPE-001.

- [Canonical design or registry](../architecture/MODEL_ENGINE.md)
- [Example acceptance criteria](../../examples/README.md)
