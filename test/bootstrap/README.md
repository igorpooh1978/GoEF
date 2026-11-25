# Bootstrap checks

From the repository root run:

    python test/bootstrap/verify.py
    python -B -m unittest discover -s test/bootstrap -p test_verify.py

Use Python 3.11 or above with approved jsonschema 4.x and PyYAML 6.x already available. W000 uses the versions recorded in the [report](../../docs/evidence/W000/BOOTSTRAP_REPORT.md). The checker is local and has no network calls. It checks relative Markdown destinations, issue-form shape, JSON syntax and schemas, cross-references between research registries and Gherkin, and proof requirements. It does not execute Gherkin, validate database behavior, or verify external links. W000 rejects runtime acceptance claims until W002 provides an executable evidence harness.

When updating the bootstrap inventory before any executable scenarios are linked, refresh the machine index with:

    python test/bootstrap/verify.py --write-bootstrap-traceability
    python test/bootstrap/verify.py

The refresh aborts if it would discard scenario execution or evidence. Update execution evidence explicitly once the runtime harness exists. Test matrix and version requirements are documented in [test environments](../../docs/development/TEST_ENVIRONMENT.md).
