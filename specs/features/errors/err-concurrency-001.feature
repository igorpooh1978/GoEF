@ERR-CONCURRENCY-001
Feature: Report optimistic concurrency conflicts independently of engine failures

  @ERR-CONCURRENCY-001-S01
  Scenario Outline: Reject a stale update without losing a committed change
    Given two independent contexts read version 1 of the same <provider> row
    When the first saves and the second updates using its stale version
    Then errors.Is identifies ErrConcurrencyConflict and typed detail records the write phase
    And the first value and advanced version remain persisted
    And the stale context retains its original version and proposed values without automatic replay

    Examples:
      | provider  |
      | postgres  |
      | sqlite    |
      | sqlserver |
      | mysql     |
