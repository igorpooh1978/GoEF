@MIG-DRIFT-001
Feature: Reject altered artifacts and mismatched live schemas

  @MIG-DRIFT-001-S01
  Scenario Outline: A change after planning invalidates the plan
    Given a reviewed <provider> migration plan and a previously matching live schema
    When a separate session changes an expected column before apply acquires its lock
    Then reinspection under the lock returns ErrSchemaDrift
    And no additional schema or history change occurs

    Examples:
      | provider  |
      | postgres  |
      | sqlite    |
      | sqlserver |
      | mysql     |

  @MIG-DRIFT-001-S02
  Scenario Outline: Reject a checksum mismatch or divergent predecessor
    Given <provider> history records a released definition with its digest and predecessor
    When the registry presents a changed digest or incompatible predecessor for that ID
    Then apply returns ErrMigrationConflict before executing any operation
    And the existing data and history entries remain unchanged

    Examples:
      | provider  |
      | postgres  |
      | sqlite    |
      | sqlserver |
      | mysql     |
