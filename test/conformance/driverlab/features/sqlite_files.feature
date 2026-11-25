@LAB-SQLITE-FILE-001
Feature: SQLite file compatibility across linked runtimes
  Both pinned runtimes can reopen and change a file whose schema uses features they share.

  @LAB-SQLITE-FILE-001-S01
  Scenario Outline: SQLite 3.51.3 reopens a file migrated by SQLite 3.54.1
    Given SQLite 3.51.3 has created a synthetic database in <journal_mode> mode
    And the file contains related rows, an audit trigger and one migration history entry
    When SQLite 3.54.1 reads the rows and rolls back a migration after a unique violation
    Then the existing schema, rows, audit records and migration history are preserved
    When SQLite 3.54.1 commits the compatible additive migration and closes the file
    Then SQLite 3.51.3 reopens the same file and reads the new schema and committed rows
    And the audit records and migration history include the committed changes only
    And integrity and foreign-key checks succeed

    Examples:
      | journal_mode |
      | delete       |
      | wal          |

  @LAB-SQLITE-FILE-001-S02
  Scenario Outline: SQLite 3.54.1 reopens a file migrated by SQLite 3.51.3
    Given SQLite 3.54.1 has created a synthetic database in <journal_mode> mode
    And the file contains related rows, an audit trigger and one migration history entry
    When SQLite 3.51.3 reads the rows and rolls back a migration after a unique violation
    Then the existing schema, rows, audit records and migration history are preserved
    When SQLite 3.51.3 commits the compatible additive migration and closes the file
    Then SQLite 3.54.1 reopens the same file and reads the new schema and committed rows
    And the audit records and migration history include the committed changes only
    And integrity and foreign-key checks succeed

    Examples:
      | journal_mode |
      | delete       |
      | wal          |
