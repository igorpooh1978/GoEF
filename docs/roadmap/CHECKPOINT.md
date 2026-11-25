# Current checkpoint

The [published W000 baseline](https://github.com/igorpooh1978/GoEF/commit/c789602e46c7063e6c0f1ddbfec68bc22c2dbb75) remains partially accepted. W001 architecture review approved the module identity and froze provider execution SPI v1. W002 now has native Go-driver Gherkin characterization and local-only CI. See the [driver run](../evidence/W002/DRIVER_GHERKIN.md) and [version compatibility](../parity/ENGINE_COMPATIBILITY.md). GoEF adapter and full application conformance remain separate. [W001 review](../evidence/W001/SPI_FREEZE.md) · [W002 smoke evidence](../evidence/W002/LAB_SMOKE.md) · [Wave dependencies](WAVES.md)

Confirm origin and HEAD, open the live tracker, read the [source lock](../upstream/lock.json), and load the relevant [skills](../development/SKILLS.md). GitHub is authoritative for current issue state.

| Issue | Handoff |
| --- | --- |
| [#12](https://github.com/igorpooh1978/GoEF/issues/12) W001 architecture | The module graph, external compile fixture and execution SPI are accepted. Future compiler, model, transaction and migration extensions retain their own implementation gates. |
| [#24](https://github.com/igorpooh1978/GoEF/issues/24) Generated bindings and release resolution | Reproducibility and stale binding checks require the generator. A release integration check depends on the approved release revision and license. |
| [#13](https://github.com/igorpooh1978/GoEF/issues/13) W002 conformance | Continue from the [runner](../../test/conformance/lab/README.md) and [lab design](../development/CONFORMANCE_LAB.md). The four driver pins, first native Gherkin suite and local CI command are in place. Continue with provider integration and application Gherkin. |
| [#26](https://github.com/igorpooh1978/GoEF/issues/26) Local CI | Run the [local test command](../../test/conformance/driverlab/README.md) on the workstation. No hosted CI runner is configured. |
| [#31](https://github.com/igorpooh1978/GoEF/issues/31) Engine version compatibility | See the [expanded local matrix and run results](../evidence/W002/LEGACY_COMPATIBILITY.md) for PostgreSQL 17, SQL Server 14.x/15.x and bidirectional SQLite file checks. SQL Server 2022 with compatibility level 140, later engine versions and the SQLite runtime floor retain their own evidence gates. |
| [#19](https://github.com/igorpooh1978/GoEF/issues/19)–[#23](https://github.com/igorpooh1978/GoEF/issues/23) Lab implementation | Owned test resources, redacted reporting, engine identity, and isolated rollback/constraint smoke checks were verified. |
| [#10](https://github.com/igorpooh1978/GoEF/issues/10) PostgreSQL documentation | [Exact source recheck](../evidence/W000/MCP_POSTGRES_RECHECK.md). The dedicated versioned documentation route is still blocked. |
| [#11](https://github.com/igorpooh1978/GoEF/issues/11) MySQL documentation | [Exact release recheck](../evidence/W000/MCP_MYSQL_RECHECK.md). The dedicated versioned documentation route is still blocked. |
| [#1](https://github.com/igorpooh1978/GoEF/issues/1) W000 and [#15](https://github.com/igorpooh1978/GoEF/issues/15) license | [Published audit](../evidence/W000/PUBLISHED_REVIEW.md) complete. The maintainer owns the new project's license and attribution decision. |
