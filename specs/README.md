# GoEF behavior specifications

The features directory contains English Gherkin. Feature tags identify registry rows. Scenario and Scenario Outline tags identify independent behavior. An outline has one scenario ID and can create several engine-specific test rows. [Traceability](traceability.json) connects each scenario to an issue, a feature, and documentation.

Bootstrap scenarios are SPECIFIED. They become executable when a named test validates the observable outcome. Record RED or characterization before changing behavior, GREEN after the change, and independently reviewed evidence on the integrated commit. No scenario earns PASS from this document, a stub step, or a SQL string alone.

The separate [W002 driver Gherkin feature](../test/conformance/driverlab/features/driver.feature) characterizes native driver behavior with real engines. Its cancellation case exercises the provider interface through a test adapter. GoEF application scenarios retain their own acceptance gates. See the [native driver evidence](../docs/evidence/W002/DRIVER_GHERKIN.md) and the [cancellation evidence](../docs/evidence/W002/SPI_CANCELLATION.md).

Run the [bootstrap validator](../test/bootstrap/README.md) after changing a spec, registry, or guide.
