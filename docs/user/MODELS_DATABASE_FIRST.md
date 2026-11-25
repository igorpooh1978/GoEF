# Generate a Go model from a database

Status: Proposed. The example is a planned acceptance case and has no runnable GoEF API yet.

Inspect a disposable schema with an input configuration and emit reviewable Go and snapshot metadata. The planned round trip compares keys, foreign keys and storage metadata, reports an unmapped type, and detects a hand-edited generated file before overwrite.

Feature IDs: MODEL-MAP-001, MODEL-TYPE-001, MODEL-REL-001.

- [Design or source evidence](../architecture/MODEL_ENGINE.md)
- [Planned examples and evidence requirements](../../examples/README.md)
