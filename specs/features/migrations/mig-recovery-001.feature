@MIG-RECOVERY-001
Feature: Verify durable effects before resuming an interrupted migration

  @MIG-RECOVERY-001-S01
  Scenario Outline: Reconcile interruption before publishing history
    Given a <provider> migration with persisted preconditions and a known starting row
    When the executor is interrupted between an operation and acknowledgement of its result
    Then a new executor reacquires the lock and checks schema, row values and history
    And it resumes only a command proven unapplied and replay-safe
    And any ambiguous effect returns ErrMigrationRecoveryRequired without a new history entry

    Examples:
      | provider  |
      | postgres  |
      | sqlite    |
      | sqlserver |
      | mysql     |

  @MIG-RECOVERY-001-S02
  Scenario Outline: Do not finalize a partially applied data backfill
    Given <provider> has a committed checkpoint in a predicate-scoped multi-batch backfill
    When the process stops before the next checkpoint
    Then the next migrator inspects the persisted batch boundary and unique keys
    And a verified resume backfills each intended row once and validates the final constraint
    And final history appears only after all postconditions hold

    Examples:
      | provider  |
      | postgres  |
      | sqlite    |
      | sqlserver |
      | mysql     |
