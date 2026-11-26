@LAB-SQLITE-WAL-001
Feature: Recover a committed WAL across the pinned SQLite runtimes
  Each direction uses a disposable local database with schema shared by both runtimes.

  @LAB-SQLITE-WAL-001-S01
  Scenario: SQLite 3.54.1 checkpoints a WAL written by SQLite 3.51.3
    Given SQLite 3.51.3 has created and closed a synthetic database in WAL mode
    When SQLite 3.51.3 commits an additive migration with automatic checkpointing disabled
    And the writer process stops before SQLite closes the connection
    Then the main file is unchanged and committed frames remain in the WAL
    When SQLite 3.54.1 opens the file and checkpoints the WAL with TRUNCATE
    Then it sees the committed migration and its resulting rows and passes integrity checks
    And SQLite 3.51.3 reopens the checkpointed file with no frames left in the WAL

  @LAB-SQLITE-WAL-001-S02
  Scenario: SQLite 3.51.3 checkpoints a WAL written by SQLite 3.54.1
    Given SQLite 3.54.1 has created and closed a synthetic database in WAL mode
    When SQLite 3.54.1 commits an additive migration with automatic checkpointing disabled
    And the writer process stops before SQLite closes the connection
    Then the main file is unchanged and committed frames remain in the WAL
    When SQLite 3.51.3 opens the file and checkpoints the WAL with TRUNCATE
    Then it sees the committed migration and its resulting rows and passes integrity checks
    And SQLite 3.54.1 reopens the checkpointed file with no frames left in the WAL
