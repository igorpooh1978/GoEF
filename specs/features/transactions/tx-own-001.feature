@TX-OWN-001
Feature: Honor transaction and connection ownership

  @TX-OWN-001-S01
  Scenario Outline: Only the external owner completes enlisted work
    Given a caller begins a transaction on an isolated <provider> connection
    And two compatible contexts explicitly enlist on that connection
    When one context saves a row and both contexts close
    Then the owner can read the pending row through the transaction
    And owner rollback leaves no row on a fresh connection
    And the caller can still use and close its connection

    Examples:
      | provider  |
      | postgres  |
      | sqlite    |
      | sqlserver |
      | mysql     |

  @TX-OWN-001-S02
  Scenario Outline: Commit and rollback have observable terminal states
    Given an isolated <provider> pool with capacity for one physical connection
    When one library-owned transaction inserts A and commits and the next inserts B and rolls back
    Then each transaction uses the same physical connection for its commands
    And a queued acquisition can reuse that physical connection after each completion
    And a fresh physical connection finds A only
    And both handles reject further commands with ErrInvalidState
    And the pool returns to its initial in-use count

    Examples:
      | provider  |
      | postgres  |
      | sqlite    |
      | sqlserver |
      | mysql     |

  @TX-OWN-001-S03
  Scenario Outline: An atomic save rolls back a partially executed graph
    Given <provider> can save a parent and its children atomically
    And the second child violates a unique constraint after the parent obtains a generated key
    When SaveChanges returns an error in an owned transaction
    Then no parent or child exists on a fresh connection
    And the tracker restores original versions and temporary keys
    And all entries remain pending for correction

    Examples:
      | provider  |
      | postgres  |
      | sqlite    |
      | sqlserver |
      | mysql     |
