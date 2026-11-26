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

  @TX-OUTCOME-001-S03
  Scenario Outline: A real commit with a withheld acknowledgement requires reconciliation
    Given a GoEF-owned transaction on <provider> has written a unique operation marker and aggregate
    When a driver fixture commits the transaction and substitutes a synthetic transport error for success
    Then Commit returns GOEF-TX-0001 and the transaction enters the unknown outcome state
    And the staged value stays unaccepted and the batch runs only once
    And the worker discards its connection and can obtain a fresh one
    And an independent primary connection sees exactly one matching marker and aggregate

    Examples:
      | provider  |
      | postgres  |
      | sqlite    |
      | sqlserver |
      | mysql     |

  @TX-OUTCOME-001-S04
  Scenario Outline: A failure before native commit leaves no durable writes
    Given a GoEF-owned transaction on <provider> has written a unique operation marker and aggregate
    When a driver fixture rolls back before native commit and returns the same synthetic transport error
    Then Commit returns GOEF-TX-0001 and the transaction enters the unknown outcome state
    And the staged value stays unaccepted and the batch runs only once
    And the worker discards its connection and can obtain a fresh one
    And an independent primary connection finds neither the marker nor the aggregate

    Examples:
      | provider  |
      | postgres  |
      | sqlite    |
      | sqlserver |
      | mysql     |

  @TX-OUTCOME-001-S05
  Scenario Outline: An authoritative negative after final rollback retains pending edits
    Given a GoEF-owned transaction on <provider> has staged changes and its native rollback completed before commit
    When the lost-response fixture reports an unknown commit outcome
    And the verifier finds neither the marker nor the aggregate without knowing whether the prior unit is final
    Then the outcome remains unknown and the receipt remains unaccepted
    When a fresh primary read again finds neither write and the verifier has proof of the prior native rollback
    Then reconciliation confirms rollback and the receipt cannot publish the staged values
    And the original snapshot and pending edits stay intact without replay

    Examples:
      | provider  |
      | postgres  |
      | sqlite    |
      | sqlserver |
      | mysql     |
