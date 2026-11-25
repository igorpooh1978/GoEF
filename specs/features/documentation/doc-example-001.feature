@DOC-EXAMPLE-001
Feature: Verify generated reference examples

  @DOC-EXAMPLE-001-S01
  Scenario: Compare regenerated examples with the runnable source
    Given an accepted API fixture and its published example output
    When the docs gate regenerates references twice and executes the canonical example
    Then both reference generations are identical
    And the runnable example produces the documented result
