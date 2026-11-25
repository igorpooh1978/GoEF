@MODEL-NULL-001
Feature: Distinguish absent and zero values

  @MODEL-NULL-001-S01
  Scenario Outline: Round-trip null, zero and nonzero values
    Given the <provider> capability target and a nullable integer column and three synthetic rows containing NULL, zero and a nonzero value
    When the rows are read through the typed model and written back to a fresh fixture
    Then the NULL remains absent and the zero remains present
    And the nonzero value remains unchanged

    Examples:
      | provider  |
      | postgres  |
      | sqlite    |
      | sqlserver |
      | mysql     |
