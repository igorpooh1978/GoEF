@TX-PG-NATIVE-001
Feature: Coordinate GoEF transactions directly on a caller-owned pgx pool
  # Issue: https://github.com/igorpooh1978/GoEF/issues/37

  @TX-PG-NATIVE-001-S01
  Scenario: Cancellation bounds connection acquisition and native begin
    Given a one-connection pgx pool on an isolated PostgreSQL server
    When a caller cancels while queued for the pool or while the driver is entering BEGIN
    Then no active transaction is returned and the error preserves the context cause
    And a later transaction can use the pool

  @TX-PG-NATIVE-001-S02
  Scenario: Owned commands stay on one backend and cancellation rolls back pending work
    Given one owned transaction has reserved the only connection and has inserted a row
    When a caller cancels the lifetime or its deadline expires during an observed advisory lock wait
    Then the command distinguishes cancellation from a deadline and the transaction ends
    And no row persists and a fresh operation can acquire the pool slot

  @TX-PG-NATIVE-001-S03
  Scenario: An error while committing returns a typed unknown outcome without replay
    Given an owned transaction has pending work on one backend
    When the driver returns a context error while the server awaits a deferred trigger at commit
    Then GoEF records the outcome as unknown and discards the connection
    And no statement or commit attempt is replayed

  @TX-PG-NATIVE-001-S04
  Scenario: A GoEF participant shares a pgx transaction without taking ownership
    Given an application owns an isolated transaction on a pgx pool
    When a participant inserts a row, reads it and rewinds a managed savepoint
    And the coordinator detaches
    Then only the native owner controls completion and the pool checkout
    And an explicit owner rollback leaves no rows visible on a fresh connection

  @TX-PG-NATIVE-001-S05
  Scenario: An explicit server rollback during COMMIT has a confirmed outcome
    Given a statement error puts the owned PostgreSQL transaction in an aborted state
    When the server acknowledges rollback in response to COMMIT
    Then GoEF reports ErrCommitRolledBack and StateRolledBack
    And the caller can reuse the same idle physical connection
    And the independent observer finds no row from the rolled back transaction

  @TX-PG-NATIVE-001-S06
  Scenario: A deferred foreign key rejection confirms rollback at COMMIT
    Given a native transaction has executed valid writes and an insert that violates a deferred foreign key
    When PostgreSQL rejects COMMIT with the foreign key violation and the session returns to idle
    Then GoEF reports ErrCommitRolledBack and StateRolledBack
    And the independent observer finds no writes from the transaction
    And the caller can reuse the same idle physical connection
