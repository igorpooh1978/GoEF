@PROVIDER-REUSE-001
Feature: Physical connection reuse after a vetted read
  Issue #39.

  @PROVIDER-REUSE-001-S01
  Scenario Outline: Keep the physical session after verified completion
    Given a one-connection <provider> pool and a caller-vetted session-neutral query
    When the adapter binds a value and the caller reads to EOF, closes the rows and requests reuse
    Then the next checkout uses the same physical connection and the bound value is unchanged

    Examples:
      | provider   |
      | PostgreSQL |
      | SQLite     |
      | SQL Server |
      | MySQL      |

  @PROVIDER-REUSE-001-S02
  Scenario Outline: Retire a session after ambiguous or unreviewed work
    Given a one-connection <provider> pool with an adapter lease
    When a command is unreviewed or the stream fails, is canceled, or is left incomplete
    Then the adapter retires the physical session even if the caller requests reuse
    And a later successful vetted read on the same lease cannot clear an earlier discard requirement
    And session state created by an ordinary command is absent on the next checkout

    Examples:
      | provider   |
      | PostgreSQL |
      | SQLite     |
      | SQL Server |
      | MySQL      |

  @PROVIDER-REUSE-001-S03
  Scenario Outline: Honor explicit discard and preserve pool ownership
    Given a <provider> adapter over a caller-owned pool
    When the caller explicitly discards a successfully drained vetted query
    Then the adapter retires its physical session
    And the caller-owned pool remains available

    Examples:
      | provider   |
      | PostgreSQL |
      | SQLite     |
      | SQL Server |
      | MySQL      |
