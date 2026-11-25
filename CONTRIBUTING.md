# Contributing to GoEF

GoEF is in W000 bootstrap. The current deliverables are design contracts, research, specifications and acceptance evidence. The [W000 tracking issue](https://github.com/igorpooh1978/GoEF/issues/1) and [checkpoint](docs/roadmap/CHECKPOINT.md) describe current scope and blockers.

1. Search the [GoEF issues](https://github.com/igorpooh1978/GoEF/issues) and use one of the [issue forms](https://github.com/igorpooh1978/GoEF/issues/new/choose) when a new issue is needed. Write repository artifacts and GitHub discussion in English. Use the [security policy](SECURITY.md) for vulnerability reports.
2. Read [AGENTS.md](AGENTS.md), the [agent workflow](docs/development/AGENT_WORKFLOW.md) and the [skills index](docs/development/SKILLS.md). Confirm the issue, baseline, scope, allowed paths and acceptance criteria before editing.
3. Follow the [issue workflow](docs/development/ISSUE_WORKFLOW.md). Anchor behavior to pinned upstream sources, link the feature and any error IDs, write Gherkin scenarios and executable tests, then update examples and documentation with the change.
4. Use [coding standards](docs/development/CODING_STANDARDS.md), [testing](docs/development/TESTING.md), and the [documentation policy](docs/development/DOCUMENTATION_POLICY.md). Run applicable checks with synthetic fixtures and isolated services. Record skipped checks and blockers.
5. Review the staged diff and include the issue ID in the local commit. Hand off the integrated SHA for independent acceptance under the [release gates](docs/development/RELEASE_GATES.md). Publishing, release, deployment and changes to real databases require separate authorization.

Documentation maps and proposed usage are in the [documentation index](docs/README.md).
