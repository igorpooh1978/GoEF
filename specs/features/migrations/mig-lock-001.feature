@MIG-LOCK-001
Feature: Serialize migration execution and release locks safely

  @MIG-LOCK-001-S01
  Scenario Outline: Two independent processes compete for the same migration
    Given two migrators targeting one <provider> history namespace and conflicting schema objects
    When a barrier holds the first migrator after lock acquisition and the second requests apply
    Then the second cannot enter execution until the first relinquishes ownership
    And after revalidation the second observes the applied migration
    And there is one history entry and one data effect

    Examples:
      | provider  |
      | postgres  |
      | sqlite    |
      | sqlserver |
      | mysql     |

  @MIG-LOCK-001-S02
  Scenario Outline: Cancel a waiting migrator
    Given an owner holds the <provider> migration lock
    When a second process cancels its pending acquisition
    Then errors.Is identifies context.Canceled and the owner retains its lock
    And the waiter writes no migration history and releases its connection

    Examples:
      | provider  |
      | postgres  |
      | sqlite    |
      | sqlserver |
      | mysql     |
