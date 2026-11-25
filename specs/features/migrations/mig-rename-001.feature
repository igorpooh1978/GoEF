@MIG-RENAME-001
Feature: Require a reviewed identity for a rename

  @MIG-RENAME-001-S01
  Scenario Outline: Preserve values and dependencies across an explicit rename
    Given a <provider> table with two distinct values and a reviewed column rename hint
    When the pinned migration is applied
    Then the new name yields both original values and existing constraint behavior
    And the old name is absent and history advances once

    Examples:
      | provider  |
      | postgres  |
      | sqlite    |
      | sqlserver |
      | mysql     |

  @MIG-RENAME-001-S02
  Scenario Outline: Block an ambiguous rename
    Given a populated <provider> column disappears and a similarly shaped column appears
    When the author scaffolds a migration without an identity hint
    Then the diff reports the ambiguous removal and addition for review
    And execution without approval returns ErrDestructiveMigration and preserves the original values

    Examples:
      | provider  |
      | postgres  |
      | sqlite    |
      | sqlserver |
      | mysql     |
