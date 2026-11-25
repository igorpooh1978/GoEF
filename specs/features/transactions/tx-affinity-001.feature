@TX-AFFINITY-001
Feature: Bind every command and stream to the enlisted connection

  @TX-AFFINITY-001-S01
  Scenario Outline: A bound graph query and writes use the same connection
    Given a <provider> pool with one connection and an open transaction
    And a second independent observer using isolation that excludes dirty reads
    When raw SQL inserts a parent and a bulk command inserts its child inside the transaction
    Then a bound split query including the child reads both pending rows
    And a bound follow-up load reads the same child
    And the observer sees neither pending row or waits until rollback
    And after rollback the observer sees neither row and the pool is reusable

    Examples:
      | provider  |
      | postgres  |
      | sqlite    |
      | sqlserver |
      | mysql     |

  @TX-AFFINITY-001-S02
  Scenario Outline: Concurrent use and cross-connection enlistment fail before dispatch
    Given an active stream holding a <provider> context and transaction
    When another operation uses that handle or another connection tries to enlist
    Then concurrent use returns ErrContextBusy and the incompatible enlistment returns ErrInvalidState
    And closing the stream permits the owner to roll back
    And using the closed context returns ErrContextClosed before database I/O

    Examples:
      | provider  |
      | postgres  |
      | sqlite    |
      | sqlserver |
      | mysql     |
