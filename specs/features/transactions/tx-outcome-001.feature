@TX-OUTCOME-001
Feature: Reconcile lost commit acknowledgements

  @TX-OUTCOME-001-S01
  Scenario Outline: Losing the acknowledgement preserves an unknown outcome
    Given a transaction on <provider> writing a unique operation marker and aggregate
    When the harness allows commit and hides its successful response from the caller
    Then errors.Is identifies ErrCommitOutcomeUnknown and the tracker is frozen for reconciliation
    And no automatic retry occurs
    And a fresh primary connection finds the marker and aggregate exactly once
    And explicit reconciliation can accept the confirmed changes

    Examples:
      | provider  |
      | postgres  |
      | sqlite    |
      | sqlserver |
      | mysql     |

  @TX-OUTCOME-001-S02
  Scenario Outline: An unavailable verifier leaves the result unresolved
    Given a lost commit response on <provider> and a verifier unable to read authoritative durable state
    When the caller requests reconciliation
    Then the outcome remains unknown and provisional values stay unaccepted
    And the callback invocation count stays one

    Examples:
      | provider  |
      | postgres  |
      | sqlite    |
      | sqlserver |
      | mysql     |
