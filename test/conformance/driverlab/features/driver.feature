@LAB-DRIVER-001
Feature: Observe transaction outcomes, constraints and reader cancellation with Go drivers

  @LAB-DRIVER-001-S01
  Scenario Outline: An independent connection sees the committed row and the rollback preserves it
    Given isolated parent and child tables on <provider> with a separate observer connection
    When the writer commits a transaction inserting a parent
    Then the observer reads the committed parent
    When the writer rolls back a transaction inserting a second parent
    Then the observer still reads only the committed parent

    Examples:
      | provider  |
      | postgres  |
      | sqlite    |
      | sqlserver |
      | mysql     |

  @LAB-DRIVER-001-S02
  Scenario Outline: A duplicate code produces the driver's native unique constraint error
    Given isolated parent and child tables on <provider> with a separate observer connection
    And a parent with a unique code exists
    When the writer inserts another parent with the same code
    Then the driver reports the native unique constraint code
    And the observer reads only the original parent

    Examples:
      | provider  |
      | postgres  |
      | sqlite    |
      | sqlserver |
      | mysql     |

  @LAB-DRIVER-001-S03
  Scenario Outline: A missing parent produces the driver's native foreign key error
    Given isolated parent and child tables on <provider> with a separate observer connection
    And a parent and its child exist
    When the writer inserts another child referencing a missing parent
    Then the driver reports the native foreign key code
    And the observer reads only the original parent and child

    Examples:
      | provider  |
      | postgres  |
      | sqlite    |
      | sqlserver |
      | mysql     |

  @LAB-DRIVER-001-S04
  Scenario Outline: Cancel a provider SPI reader after the first row
    Given a single-connection provider SPI fixture on <provider>
    When the reader reads the first row and the caller cancels
    Then the reader reports caller cancellation and releases its connection
    And a fresh reader reads both rows and releases its connection

    Examples:
      | provider  |
      | postgres  |
      | sqlite    |
      | sqlserver |
      | mysql     |

  @LAB-DRIVER-001-S05
  Scenario Outline: Distinguish a caller deadline from an engine lock timeout
    Given isolated parent and child tables on <provider> with a separate observer connection
    When the contender executes both lock waits while the writer holds the write lock
    Then the observed waits distinguish the caller deadline and native engine timeout
    And the observer reads exactly the rows that were seeded for the lock waits

    Examples:
      | provider  |
      | postgres  |
      | sqlite    |
      | sqlserver |
      | mysql     |
