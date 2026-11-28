@TOOL-VERSION-001
Feature: Identify the LINQ binding generator build

  @TOOL-VERSION-001-S01
  Scenario: An installed generator reports the selected module version
    Given the generator installed from a versioned Go module
    When I run the generator with the version flag
    Then standard output contains the command name and selected module version on one line
    And the command exits successfully without generating bindings

  @TOOL-VERSION-001-S02
  Scenario: A source build reports available Git provenance
    Given binaries built from clean and modified Git checkouts
    When I run each binary with the version flag
    Then their version lines identify the Git revision and dirty state
    And an unstamped build with no embedded module version reports development
