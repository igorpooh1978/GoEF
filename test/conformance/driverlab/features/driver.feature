@LAB-DRIVER-001
Feature: Observe transaction outcomes and constraint errors through native Go drivers

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
