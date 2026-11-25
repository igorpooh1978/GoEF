# Test environment

Status: Proposed for [issue #13](https://github.com/igorpooh1978/GoEF/issues/13). See [provider candidates](../parity/PROVIDER_MATRIX.md) for engine and driver status.

- Capture the image digest or official engine build, observed server version, driver version, OS and architecture, database settings, fixture identifier and commit. Select versions before claiming coverage and exercise every advertised combination.
- PostgreSQL uses an isolated database or schema and explicit isolation settings. MySQL uses isolated InnoDB instances or schemas with observed SQL mode and timezone. SQL Server runs on a supported host with documented isolation and MARS settings. SQLite uses the selected actual engine and driver, a file in the test directory and separate connections for contention.
- Use per-run names, disposable storage, bounded startup and teardown, explicit resource owner and cleanup, barrier-based lock coordination, and an isolated fault proxy where needed. Capture rows, native codes, lock or pool counters and migration history.
- Keep DSNs in runtime configuration. Prevent test scripts from accepting production targets. Seed with synthetic rows and redact server messages/schema details from uploaded logs. Bind endpoints to an isolated network and record supported startup requirements.
- Keep live integration runs separate from source or compiler checks. Use only approved tools and image/driver pins. Set independent deadlines for server readiness, connect, command and process teardown. Missing access is visible in each affected matrix cell.
