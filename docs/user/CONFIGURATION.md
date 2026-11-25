# Configure a provider and a context

Status: Proposed. The example is a planned acceptance case and has no runnable GoEF API yet.

Select one provider and identify who owns the pool, context and active transaction before opening a connection. The planned example cancels a command at its deadline and returns the connection while leaving an externally owned pool open. Connection strings come from runtime configuration.

Feature IDs: TX-AFFINITY-001, ERR-CANCEL-001.

- [Design or source evidence](../architecture/SECURITY_AND_LIFECYCLE.md)
- [Planned examples and evidence requirements](../../examples/README.md)
