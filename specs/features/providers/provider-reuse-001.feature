@PROVIDER-REUSE-001
Feature: PostgreSQL connection reuse after a vetted read
  Issue #39.

  @PROVIDER-REUSE-001-S01
  Scenario: Keep the physical session after verified completion
    Given a one-connection PostgreSQL pool and a caller-vetted session-neutral query
    When the adapter binds a value and the caller reads to EOF, closes the rows and requests reuse
    Then the next checkout has the same backend PID and the bound value is unchanged

  @PROVIDER-REUSE-001-S02
  Scenario: Retire a session after ambiguous or unreviewed work
    Given a one-connection PostgreSQL pool with an adapter lease
    When a command is unreviewed or the stream fails, is canceled, or is left incomplete
    Then the adapter retires the physical session even if the caller requests reuse
    And a later successful vetted read on the same lease cannot clear an earlier discard requirement

  @PROVIDER-REUSE-001-S03
  Scenario: Honor explicit discard and preserve pool ownership
    Given a PostgreSQL adapter over a caller-owned native pool
    When the caller explicitly discards a successfully drained vetted query
    Then the adapter retires its physical session
    And the native pool remains available after the database/sql wrapper closes
