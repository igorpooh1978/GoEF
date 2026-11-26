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

  @MIG-SQLITE-001-S03
  Scenario Outline: Resume a reviewed transactional rebuild after process exit
    Given a file-backed SQLite <engine> database in <journal_mode> mode with a verified applied lineage
    When a process exits after the GoEF executor checks the rebuilt table and before it publishes history
    Then another executor acquires the writer lock and checks the original schema, values and history
    And the executor rejects ambiguous state and commits a proven unapplied rebuild with its history exactly once
    And a separate reader verifies the committed values, history and foreign-key integrity

    Examples:
      | engine | journal_mode |
      | 3.51.3 | delete       |
      | 3.51.3 | wal          |
      | 3.54.1 | delete       |
      | 3.54.1 | wal          |
