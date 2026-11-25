@MIG-SQLITE-001
Feature: Preserve dependencies and values during SQLite table rebuilds

  @MIG-SQLITE-001-S01
  Scenario: Rebuild with an explicit writable column projection
    Given a file-backed SQLite table with generated values, foreign keys, sequence state, a trigger and an index
    When a migration rebuilds the table and restores connection settings
    Then the writable values, generated results and next identity satisfy the predeclared expectations
    And foreign_key_check is empty, index and trigger behavior survive, and history advances once

  @MIG-SQLITE-001-S02
  Scenario: Reject an unrecognized dependent object
    Given a SQLite rebuild plan encounters a dependent object with unsupported reconstruction
    When execution validates its dependency inventory
    Then it returns ErrUnsupportedFeature before copying or dropping the old table
    And the original rows, objects and history remain intact after reopening the file
