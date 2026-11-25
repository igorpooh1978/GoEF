@MIG-API-001
Feature: Apply versioned Go definitions through the public library

  @MIG-API-001-S01
  Scenario Outline: Add an optional column without an installed CLI
    Given a <provider> database at release 001 containing one customer
    And a registered manual Go migration 002 adds a nullable address column
    When the application plans and applies 002 through the library
    Then the column exists, the customer values are unchanged, and history records 002 exactly once
    And repeating apply changes neither schema, rows nor history

    Examples:
      | provider  |
      | postgres  |
      | sqlite    |
      | sqlserver |
      | mysql     |

  @MIG-API-001-S02
  Scenario Outline: Scaffold and preview are deterministic
    Given a released snapshot and desired model for <provider>
    When the same inputs are scaffolded twice and the resulting plan is previewed
    Then both operation manifests, generated definitions and target snapshots are byte-identical
    And preview leaves schema, rows, locks and migration history as it found them
    And diffing the desired model against its own snapshot produces zero operations

    Examples:
      | provider  |
      | postgres  |
      | sqlite    |
      | sqlserver |
      | mysql     |

  @MIG-API-001-S03
  Scenario Outline: Check downgrade and baseline safety
    Given a <provider> database matching a candidate legacy baseline
    When the caller explicitly verifies and records the baseline and applies a reversible change
    Then downgrade and re-upgrade reproduce the expected schema and surviving row values
    And a separate forward-only change rejects downgrade with ErrIrreversibleMigration before mutation

    Examples:
      | provider  |
      | postgres  |
      | sqlite    |
      | sqlserver |
      | mysql     |
