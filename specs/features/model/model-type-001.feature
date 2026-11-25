@MODEL-TYPE-001
Feature: Explicit provider type mapping

  @MODEL-TYPE-001-S01
  Scenario Outline: Resolve the requested exact decimal explicitly
    Given the <provider> capability target and a requested exact decimal precision and scale
    When the model resolves the property type
    Then a supported mapping defines an exact codec and the provider storage contract
    And a synthetic boundary value round-trips without conversion through floating point
    And an unsupported mapping produces a typed rejection before any schema operation

    Examples:
      | provider  |
      | postgres  |
      | sqlite    |
      | sqlserver |
      | mysql     |
