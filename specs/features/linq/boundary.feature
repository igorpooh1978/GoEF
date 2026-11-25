@LINQ-BOUNDARY-001
Feature: Cross the client boundary explicitly

  @LINQ-BOUNDARY-001-S01
  Scenario Outline: An untranslatable predicate fails before retrieval
    Given a database-backed query on <provider>
    When I request results using an unregistered local-only predicate
    Then the result is a registered translation or unsupported-feature error
    And zero commands execute and zero local callbacks run

    Examples:
      | provider   |
      | postgres   |
      | sqlite     |
      | sqlserver  |
      | mysql      |

  @LINQ-BOUNDARY-001-S02
  Scenario: Local callbacks see only explicitly materialized results
    Given a server-filtered query that returns two of five rows
    When I successfully materialize the two rows and convert them to Objects
    Then the local projection receives exactly those two rows
    And enumerating the local values after context disposal sends zero commands

  @LINQ-BOUNDARY-001-S03
  Scenario: A late decoding error remains observable
    Given a caller-owned transaction and a row stream that fails decoding its third row
    When I consume the stream to termination
    Then two successful items precede exactly one terminal error
    And the stream closes its rows and leaves the transaction owned by the caller

  @LINQ-BOUNDARY-001-S04
  Scenario: Early stop releases streaming resources
    Given an open error-aware stream with at least two rows
    When the consumer stops after the first row
    Then the stream closes the reader and releases its borrowed resources
    And the producer performs no further reads
