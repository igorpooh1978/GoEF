@QUERY-UNSUPPORTED-001
Feature: Report unsupported SQL translation

  @QUERY-UNSUPPORTED-001-S01
  Scenario Outline: Reject an unregistered local callback on the server
    Given the <provider> capability target and a server query with an unregistered local callback in the predicate
    When the terminal operation requests a database result
    Then the query reports an unsupported translation kind
    And the provider observes no query or implicit client-side scan

    Examples:
      | provider  |
      | postgres  |
      | sqlite    |
      | sqlserver |
      | mysql     |
