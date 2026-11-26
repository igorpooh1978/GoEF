@MODEL-MAP-001
Feature: Stable model mapping and generation

  @MODEL-MAP-001-S01
  Scenario Outline: Generate the same model twice
    Given the <provider> capability target and the same synthetic customer schema and generator settings
    When the design-time model generator runs twice in separate directories
    Then the normalized model snapshot and generated Go files are byte-for-byte identical
    And the generated descriptors preserve the primary key and property order

    Examples:
      | provider  |
      | postgres  |
      | sqlite    |
      | sqlserver |
      | mysql     |

  @MODEL-MAP-001-S02
  Scenario: Resolve Code First table and column settings per facet
    Given customer and audit structs registered by name and a customer with an ordered composite key
    And a customer name with a tagged column and maximum length
    When fluent options override the customer table and name column and the model is mapped in both registration orders
    Then the customer uses the fluent table and name column with the tagged name length
    And the snapshots match byte for byte and preserve the field and key orders
    And callers can modify the returned metadata and snapshot without changing the published mapping

  @MODEL-MAP-001-S03
  Scenario: Reject duplicate effective columns without exposing identifiers
    Given two fields of a customer mapped to the same column by a tag and a fluent option
    When the model is mapped
    Then no mapping is published
    And a portable invalid-model error identifies both field positions and omits the raw column name
