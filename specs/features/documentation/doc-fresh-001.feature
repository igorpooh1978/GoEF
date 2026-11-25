@DOC-FRESH-001
Feature: Detect stale documentation

  @DOC-FRESH-001-S01
  Scenario: Reject a changed public contract with stale docs
    Given a change to an accepted public operation and its documented output
    When the docs gate compares the current references with the old expected output
    Then the gate identifies the stale reference or example
    And the change cannot receive documentation acceptance
