@PROVIDER-EXEC-001 @issue-13
Feature: Execute application commands through a production adapter

  @PROVIDER-EXEC-001-S01
  Scenario Outline: Validate bindings and read the committed typed row
    Given an isolated <provider> database and its configured production adapter
    When the application submits a wrong binding kind and then a correctly bound write
    Then the rejected binding leaves the table unchanged
    And the adapter returns the requested row with its original text and a clean end of stream
    And an independent observer sees the write and the adapter releases its lease

    Examples:
      | provider   |
      | PostgreSQL |
      | SQLite     |
      | SQL Server |
      | MySQL      |

  @PROVIDER-EXEC-001-S02
  Scenario Outline: Preserve native constraint codes without private diagnostics
    Given an isolated <provider> database with unique and foreign-key constraints
    When the production adapter executes duplicate and orphan writes
    Then the errors expose their safe native codes and supported portable classifications
    And a native code shared by distinct constraint classes remains unclassified
    And the adapter exposes neither the native error object nor the statement or bound values
    And an independent observer confirms the failed writes left no additional rows

    Examples:
      | provider   |
      | PostgreSQL |
      | SQLite     |
      | SQL Server |
      | MySQL      |

  @PROVIDER-EXEC-001-S03
  Scenario Outline: Recover after stream and pool acquisition cancellation
    Given the production adapter owns a lease on a one-connection <provider> pool
    When the application cancels a stream after its first row
    Then iteration reports caller cancellation and cleanup discards the connection
    When a queued acquisition is canceled while another lease holds the pool slot
    Then it reports caller cancellation without granting a lease
    And a fresh lease reads both control rows with no connection left in use

    Examples:
      | provider   |
      | PostgreSQL |
      | SQLite     |
      | SQL Server |
      | MySQL      |

  @PROVIDER-EXEC-001-S04
  Scenario Outline: Compose the adapter with an owned transaction
    Given an isolated <provider> database and its configured production adapter
    When GoEF saves a tracked value through an owned transaction on the adapter pool
    Then the staged value stays unpublished until the committed receipt is accepted
    And a separate adapter read sees the committed row and releases its connection

    Examples:
      | provider   |
      | PostgreSQL |
      | SQLite     |
      | SQL Server |
      | MySQL      |
