@TX-ISOLATION-001
Feature: Honor the requested transaction options

  @TX-ISOLATION-001-S01
  Scenario Outline: Demand the advertised effective isolation
    Given an isolated <provider> database with a recorded isolation and read-only capability matrix
    When the runner begins each declared mode and coordinates separate physical sessions
    Then the recorded settings or coordinated visibility and locking match the advertised mode
    And a read-only transaction rejects writes for each advertised read-only mode
    And an unsupported option returns ErrUnsupportedFeature before any write
    And the pool is available for a new default transaction after completion

    Examples:
      | provider  |
      | postgres  |
      | sqlite    |
      | sqlserver |
      | mysql     |

  @TX-ISOLATION-001-S02
  Scenario Outline: Unsupported nesting is explicit
    Given an active <provider> transaction with one pending insert
    When the callback begins a nested independent transaction on the same handle
    Then the request returns ErrUnsupportedFeature
    And the owner can roll back its insert without affecting other sessions

    Examples:
      | provider  |
      | postgres  |
      | sqlite    |
      | sqlserver |
      | mysql     |
