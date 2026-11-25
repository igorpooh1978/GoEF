@MODEL-MAP-001
Feature: Stable model generation

  @MODEL-MAP-001-S01
  Scenario Outline: Generate the same model twice
    Given the <provider> capability target and the same synthetic customer schema and generator settings
    When the design-time model generator runs twice in separate directories
    Then the normalized model snapshot and generated Go files are byte-for-byte identical
    And the generated descriptors preserve the primary key and property order

    Examples:
      | provider  |
      | postgres  |
      | sqlite    |
      | sqlserver |
      | mysql     |
