@MODEL-VALID-001
Feature: Fail invalid models before database access

  @MODEL-VALID-001-S01
  Scenario Outline: Reject incompatible key declarations
    Given the <provider> capability target and a model with an optional property declared as part of a required primary key
    When model validation runs for that model
    Then the returned error identifies the invalid model kind
    And the provider observes no database command

    Examples:
      | provider  |
      | postgres  |
      | sqlite    |
      | sqlserver |
      | mysql     |
