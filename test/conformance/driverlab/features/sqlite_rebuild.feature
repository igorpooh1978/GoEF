@LAB-SQLITE-REBUILD-001
Feature: Recover a SQLite table rebuild interrupted before commit

  @LAB-SQLITE-REBUILD-001-S01
  Scenario Outline: An interrupted rebuild preserves the original schema, rows and history
    Given SQLite <engine> has seeded a synthetic database in <journal_mode> mode
    When a worker copies the writable columns, swaps the table, recreates its index and trigger, and writes within its transaction
    And the worker verifies the rebuilt table and exits before recording history or committing
    Then a new process verifies the original schema, rows, sequence, dependencies and history
    When that process commits the rebuild and history in one transaction
    Then an independent process reads the rebuilt table, preserved rows and exactly one new history entry

    Examples:
      | engine | journal_mode |
      | 3.51.3 | delete       |
      | 3.51.3 | wal          |
      | 3.54.1 | delete       |
      | 3.54.1 | wal          |
