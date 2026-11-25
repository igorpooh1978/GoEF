---
name: goef-model-engine
description: Design or change the GoEF logical and physical model, Code First tags or Fluent APIs, Database First scaffolding, keys, relationships, and deterministic model snapshots.
---

# GoEF Model Engine

1. Read the [model contract](../../../docs/architecture/MODEL_ENGINE.md) and verify upstream precedence rules, model validation and round trips. Identify Code First, Database First and model metadata effects.
2. Trace each input through normalized logical metadata, validated keys/relationships, provider-specific type binding and a canonical snapshot. Define deterministic ordering, ownership and validation failures. Keep physical provider types and facets out of portable metadata APIs.
3. Prove conventions, tag and explicit overrides with order-independent snapshot tests and meaningful contradiction tests. Exercise composite/alternate/foreign keys, shadow or owned values, nullability and generated defaults as relevant.
4. Record diagnostics and fixture round trips, linked scenarios, snapshot schema version and compatibility effects. Change source definitions/templates and regenerate approved outputs with freshness proof.

If tags and explicit relationship options disagree, consult the validated precedence contract. Exercise contradictory keys and prove repeat runs produce the same snapshot.
