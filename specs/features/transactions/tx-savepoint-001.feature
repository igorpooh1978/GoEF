@TX-SAVEPOINT-001
Feature: Bound the effects of failed saves with savepoints

  @TX-SAVEPOINT-001-S01
  Scenario Outline: Retain the outer write while reverting a failed save
    Given a <provider> configuration with usable savepoints and a caller-owned transaction
    And the owner inserted A before SaveChanges
    When a two-command save fails after the first command
    Then rollback to the savepoint removes the batch's writes and leaves staged values unpublished
    And the owner can commit A alone, verified on a new connection
    When a later caller-owned transaction saves B and the coordinator detaches
    Then staged values become visible to the application only after the owner commits and accepts them
    And a fresh connection reads A and B

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
    Then it returns ErrUnsupportedFeature before calling the batch or dispatching any writes
    And the owner can still commit its previously written row

    Examples:
      | provider  |
      | postgres  |
      | sqlite    |
      | sqlserver |
      | mysql     |

  @TX-SAVEPOINT-001-S03
  Scenario Outline: Roll back nested explicit savepoints without ending the transaction
    Given a GoEF-owned <provider> transaction has inserted A
    When the caller creates nested savepoints around later writes on the same connection
    Then a point with a younger active point cannot be released
    When the caller rolls back to the older point
    Then the younger point is invalid and the older point can be used again
    When the caller rolls back to the older point again, releases it, and commits
    Then the released point is invalid and a fresh physical connection sees only A

    Examples:
      | provider  |
      | postgres  |
      | sqlite    |
      | sqlserver |
      | mysql     |
