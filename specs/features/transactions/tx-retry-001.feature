@TX-RETRY-001
Feature: Replay only a complete, explicitly replay-safe unit

  @TX-RETRY-001-S01
  Scenario Outline: Repeat using a fresh transaction after a confirmed rollback
    Given an isolated <provider> database with a replay-safe callback and an allowed retry budget
    And a deterministic fault makes the first attempt transient and proves its rollback
    When the execution strategy invokes the callback again
    Then it creates a fresh transaction and tracker from the original input
    And a fresh connection observes exactly one logical effect and its unique operation marker

    Examples:
      | provider  |
      | postgres  |
      | sqlite    |
      | sqlserver |
      | mysql     |

  @TX-RETRY-001-S02
  Scenario Outline: Respect the retry budget and cancellation
    Given a replay-safe transaction on <provider> repeatedly encounters a classified rollback
    When the configured attempt limit or context deadline is reached
    Then no additional callback starts
    And attempts, elapsed budget and the last safe cause are available
    And rollback leaves the fixture and pool reusable

    Examples:
      | provider  |
      | postgres  |
      | sqlite    |
      | sqlserver |
      | mysql     |

  @TX-RETRY-001-S03
  Scenario Outline: Never retry constraints or irreversible external work
    Given a <provider> callback with a constraint failure or no replay contract
    When the callback fails
    Then the caller observes the first failure and a single callback invocation
    And no later attempt changes the recorded rows

    Examples:
      | provider  |
      | postgres  |
      | sqlite    |
      | sqlserver |
      | mysql     |
