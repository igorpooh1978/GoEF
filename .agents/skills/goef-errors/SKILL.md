---
name: goef-errors
description: Work on GoEF's canonical error registry, stable IDs, typed errors, dialect native-code mapping, redaction, transaction outcome and retry classification.
---

# GoEF Errors

1. Read the [error contract](../../../docs/architecture/ERROR_CONTRACT.md), [registry](../../../docs/errors/registry.json) and [mapping policy](../../../docs/errors/README.md). Locate existing IDs before allocating an ID with the owner.
2. Map documented native code and operation context at the provider boundary. Verify engine/driver versions, wrapped errors and fields with an isolated reproduction. Keep stable errors usable with `errors.Is` and `errors.As`.
3. Distinguish cancellation, transient failures, concurrency conflicts and unknown transaction outcomes. Preserve context causes and safe metadata where the public contract allows. Redact credentials, SQL values, DSNs and raw server excerpts from public text and evidence.
4. Cover unknown codes and mapping collisions, including same numeric code with different states or phases. Link registry entry and mappings to issue, Gherkin, the isolated native-error reproduction, docs and exact evidence SHA. Include the issue and reproduction link in each mapping's evidence record.

If only a localized message changes, use stable code/fields for the mapping and leave unknown codes visibly classified without a guessed retry action.
