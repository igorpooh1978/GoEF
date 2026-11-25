# Track edits and save changes

Status: Proposed. The example is a planned acceptance case and has no runnable GoEF API yet.

Load an entity with an original version, edit it, and inspect its state after a successful save. A second planned context tries to save a stale version and receives a portable conflict. Verify stored rows and the failed context's retry state.

Feature IDs: ERR-CONCURRENCY-001, TX-OWN-001.

- [Design or source evidence](../architecture/CHANGE_TRACKER.md)
- [Planned examples and evidence requirements](../../examples/README.md)
