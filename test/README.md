# GoEF test assets

- The [W000 Go language probe](../docs/evidence/W000/GO_FEASIBILITY.md) and [W001 typed query fixtures](../docs/evidence/W001/LINQ_COMPILER.md) are separate isolated modules.
- [Provider environments](../docs/development/TEST_ENVIRONMENT.md) define fixture and engine requirements. The [W002 lab design](../docs/development/CONFORMANCE_LAB.md) describes setup and acceptance for the planned suite.
- [Transaction conformance](conformance/transactions/README.md) and [LINQ parity](../docs/upstream/LINQ_RESEARCH.md) define planned behavioral tests.
- The [external provider fixture](architecture/provider-spi/go.mod) checks the frozen SPI from a distinct Go module. The [four-engine smoke lab](conformance/lab/README.md) checks pinned engines with native clients.
- [Bootstrap validation](bootstrap/README.md) checks repository artifacts. Executable GoEF Gherkin scenarios require the GoEF runtime and provider adapters.

Use synthetic data and isolated resources. Test provider behavior against the real engine and driver version recorded in the evidence.
