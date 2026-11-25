# Current checkpoint

The [published W000 baseline](https://github.com/igorpooh1978/GoEF/commit/c789602e46c7063e6c0f1ddbfec68bc22c2dbb75) was audited after publication. The current scope covers W001 architecture design and compiler fixtures, provider documentation rechecks, and the W002 test lab design. Start with the [published W000 audit](../evidence/W000/PUBLISHED_REVIEW.md), [bootstrap evidence](../evidence/W000/BOOTSTRAP_REPORT.md), and [wave dependencies](WAVES.md).

Confirm origin and HEAD, open the live tracker, read the [source lock](../upstream/lock.json), and load the relevant [skills](../development/SKILLS.md). All listed issues are assigned to igorpooh1978. GitHub remains the current source for issue state.

| Issue | Handoff | Evidence or next proof |
| --- | --- | --- |
| [#1](https://github.com/igorpooh1978/GoEF/issues/1) W000 | Partial | Published and audited. The dedicated exact-version documentation routes and project license are still tracked. |
| [#10](https://github.com/igorpooh1978/GoEF/issues/10) PostgreSQL documentation | Dedicated route blocked | [Recheck](../evidence/W000/MCP_POSTGRES_RECHECK.md). Exact 18.7 source was retrieved through GitHub MCP. Context7 still has older content. |
| [#11](https://github.com/igorpooh1978/GoEF/issues/11) MySQL documentation | Exact MCP route blocked | [Recheck](../evidence/W000/MCP_MYSQL_RECHECK.md). Oracle's exact release note and the tag's version file agree. The dedicated MCP corpus returned the 8.4 family. |
| [#12](https://github.com/igorpooh1978/GoEF/issues/12) W001 architecture | Design delivered for review | [Module graph](../architecture/MODULE_GRAPH.md) and [public API proposal](../architecture/PUBLIC_API.md). Stabilize the API before authoring provider modules. |
| [#17](https://github.com/igorpooh1978/GoEF/issues/17) W001 LINQ compilation | Isolated fixture accepted | [Compiler evidence](../evidence/W001/LINQ_COMPILER.md). Model-generated descriptors and real SQL behavior still require their own proof. |
| [#16](https://github.com/igorpooh1978/GoEF/issues/16) W002 test lab design | Design delivered | [Lab design](../development/CONFORMANCE_LAB.md). Implement the lab once the provider contracts and versions are accepted. |
| [#13](https://github.com/igorpooh1978/GoEF/issues/13) W002 conformance implementation | Blocked | Requires approved W001 contracts, pinned engines and drivers, and the executable isolated suite. |
| [#18](https://github.com/igorpooh1978/GoEF/issues/18) W000 publication audit | Audit delivered | [Published baseline audit](../evidence/W000/PUBLISHED_REVIEW.md). The master plan now distinguishes probes from installed endpoint configuration. |
| [#15](https://github.com/igorpooh1978/GoEF/issues/15) GoEF project license | Blocked | The maintainer owns the project license and attribution decision. |

The published W000 documentation issues [#2](https://github.com/igorpooh1978/GoEF/issues/2), [#3](https://github.com/igorpooh1978/GoEF/issues/3), [#4](https://github.com/igorpooh1978/GoEF/issues/4), [#5](https://github.com/igorpooh1978/GoEF/issues/5), [#6](https://github.com/igorpooh1978/GoEF/issues/6), [#7](https://github.com/igorpooh1978/GoEF/issues/7), [#8](https://github.com/igorpooh1978/GoEF/issues/8), [#9](https://github.com/igorpooh1978/GoEF/issues/9), and [#14](https://github.com/igorpooh1978/GoEF/issues/14) are open for scoped acceptance or follow-up.

The language and typed-query probes are isolated test modules. Production API examples, native errors on real providers, and full four-engine Gherkin execution remain future work.
