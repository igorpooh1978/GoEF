@LINQ-PG-001
Feature: Materialize a typed PostgreSQL equality read
  An explicitly mapped projection is compiled before execution and binds one value per read.

  @LINQ-PG-001-S01
  Scenario: Rebind one compiled settings projection
    Given two settings rows in an isolated PostgreSQL database
    When I execute the same compiled projection with two different text keys
    Then the adapter dispatches one parameterized command per key
    And the typed values preserve null and present revisions and timestamps
    And an unmatched key is distinguishable from a matched row with zero values

  @LINQ-PG-001-S02
  Scenario: Execute a generated scalar projection with the same compiler
    Given a generated equality shape and its Code First mapping
    When I execute the generated scalar projection
    Then the provider returns the selected scalar without running a local projection callback
    And a mismatched mapped field or unsupported model type is rejected before acquisition

  @LINQ-PG-001-S03
  Scenario: Preserve cardinality and cancellation while releasing owned resources
    Given a PostgreSQL projection limited to at most one result
    When two settings rows match or an in-flight read is canceled
    Then the corresponding typed error is observable
    And the caller-owned pool can run subsequent reads

  @LINQ-PG-001-S04
  Scenario: Reuse the connection for a reviewed compiled read
    Given a vetted settings projection and a single available PostgreSQL pool slot
    When a matching and a missing key are fetched with explicit permission to reuse
    Then the results preserve their cardinality and the next checkout has the original backend PID
