@QUERY-PARAM-001
Feature: Bind captured query values

  @QUERY-PARAM-001-S01
  Scenario Outline: Preserve query structure for an input containing SQL punctuation
    Given the <provider> capability target and a synthetic customer lookup and a captured string containing a quote
    When the query is compiled and executed for the captured value
    Then the SQL command retains one value placeholder and has no captured input in its text
    And the provider receives the exact value as a parameter
    And the result contains only the matching synthetic row

    Examples:
      | provider  |
      | postgres  |
      | sqlite    |
      | sqlserver |
      | mysql     |
