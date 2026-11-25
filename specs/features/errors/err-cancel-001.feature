@ERR-CANCEL-001
Feature: Preserve caller cancellation, deadlines and independent cleanup errors

  @ERR-CANCEL-001-S01
  Scenario Outline: Cancel a live stream
    Given an active <provider> stream owns a connection and has yielded one row
    When the caller cancels its request and closes the stream
    Then errors.Is on the terminal result identifies context.Canceled
    And an independent operation can acquire the released connection within its deadline

    Examples:
      | provider  |
      | postgres  |
      | sqlite    |
      | sqlserver |
      | mysql     |

  @ERR-CANCEL-001-S02
  Scenario Outline: Distinguish a causal deadline from an engine timeout
    Given a coordinated lock wait on <provider>
    When a caller deadline expires while waiting
    Then errors.Is identifies context.DeadlineExceeded with a redacted phase
    And a separate run with a live context and only a server timeout never identifies a caller deadline

    Examples:
      | provider  |
      | postgres  |
      | sqlite    |
      | sqlserver |
      | mysql     |

  @ERR-CANCEL-001-S03
  Scenario Outline: Retain an independent cleanup failure
    Given a canceled <provider> operation whose resource cleanup also fails at a controlled seam
    When the bounded cleanup ends
    Then errors.Is still reaches context.Canceled and errors.As can inspect the cleanup branch
    And the uncertain connection is discarded and subsequent pool work completes

    Examples:
      | provider  |
      | postgres  |
      | sqlite    |
      | sqlserver |
      | mysql     |
