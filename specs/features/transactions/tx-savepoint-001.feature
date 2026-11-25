@TX-SAVEPOINT-001
Feature: Bound the effects of failed saves with savepoints

  @TX-SAVEPOINT-001-S01
  Scenario Outline: Retain the outer write while reverting a failed save
    Given a <provider> configuration with usable savepoints and a caller-owned transaction
    And the owner inserted A before SaveChanges
    When a two-command save fails after the first command
    Then rollback to the savepoint removes both save effects and restores pending tracker entries
    And the owner can commit A alone, verified on a new connection

    Examples:
      | provider  |
      | postgres  |
      | sqlite    |
      | sqlserver |
      | mysql     |

  @TX-SAVEPOINT-001-S02
  Scenario Outline: Reject an unsafe caller-owned save without changing the transaction
    Given a <provider> configuration declared unable to guarantee savepoint recovery
    When a caller-owned multi-command save requests atomic recovery
    Then it returns ErrUnsupportedFeature before dispatch
    And the owner can still commit its previously written row

    Examples:
      | provider  |
      | postgres  |
      | sqlite    |
      | sqlserver |
      | mysql     |
