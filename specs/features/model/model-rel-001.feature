@MODEL-REL-001
Feature: Preserve relationship identity

  @MODEL-REL-001-S01
  Scenario Outline: Load a tracked parent and multiple children
    Given the <provider> capability target and one tracked customer and two distinct related orders
    When a supported relationship load materializes both orders and their principal
    Then the identity map contains one customer instance with both related children

    Examples:
      | provider  |
      | postgres  |
      | sqlite    |
      | sqlserver |
      | mysql     |
