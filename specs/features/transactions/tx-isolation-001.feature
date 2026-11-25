@TX-ISOLATION-001
Feature: Honor the requested transaction options

  @TX-ISOLATION-001-S01
  Scenario Outline: Demand the advertised effective isolation
    Given an isolated <provider> database with a recorded isolation and read-only capability matrix
    When the runner begins each declared mode with a separate writer and observer
    Then the server settings and coordinated reads or locks match the advertised mode
    And a read-only transaction rejects writes for each advertised read-only mode
    And an unsupported request returns ErrUnsupportedFeature before the callback or any write

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
