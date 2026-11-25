@LINQ-EXPRESSION-001
Feature: Compose typed expression trees

  @LINQ-EXPRESSION-001-S01
  Scenario: A field from a different alias fails validation
    Given two typed aliases bound to different query scopes
    When I compare a field from the second scope in the first scope
    Then binding reports the foreign field and its source span
    And the provider receives zero commands

  @LINQ-EXPRESSION-001-S02
  Scenario: A reusable expression binds the current argument
    Given one plan with an integer minimum parameter
    When it is invoked with minima 11 and 19
    Then the same plan shape has distinct bindings of 11 and 19
    And neither invocation modifies the other binding

  @LINQ-EXPRESSION-001-S03
  Scenario: A changed declaration invalidates generated code
    Given a generated entrypoint for a marked source declaration
    When the declaration changes without regeneration
    Then the freshness check identifies the changed declaration
    And an unsupported expression is diagnosed at its source span on regeneration
