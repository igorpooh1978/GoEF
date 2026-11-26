@MODEL-TYPE-001
Feature: Explicit provider type mapping

  @MODEL-TYPE-001-S01
  Scenario Outline: Resolve the requested exact decimal explicitly
    Given the <provider> capability target and an explicitly declared decimal precision and scale
    When the binding supplies the storage declaration and encodes signed values
    Then the engine catalog shows the expected type and facets
    And a fresh physical connection reads exact boundary values at their declared scale
    And an unsupported declaration or lossy value is rejected before the relevant database operation

    Examples:
      | provider  |
      | postgres  |
      | sqlite    |
      | sqlserver |
      | mysql     |

  @MODEL-TYPE-001-S02
  Scenario Outline: Store bound Code First scalar values on each provider
    Given the <provider> target and a Code First model with signed integers, Boolean, bounded and unbounded nullable text and binary fields
    When the scalar bindings define columns in a disposable table and the pinned driver writes and reads representative values
    Then the provider catalog shows the bound type and nullability of every column in model order
    And signed boundary values, Boolean values, Unicode text and binary bytes round-trip exactly
    And null and present empty text remain distinct
    And the text length is declared where the provider supports it and retained as model metadata for SQLite

    Examples:
      | provider  |
      | postgres  |
      | sqlite    |
      | sqlserver |
      | mysql     |

  @MODEL-TYPE-001-S03
  Scenario: Reject an unsupported scalar before any database operation
    Given a mapped Code First entity with a uint64 property
    When its types are bound for all four targets
    Then each target produces a portable invalid-model error and no physical model
    And the error identifies the provider and the field position without revealing model names

  @MODEL-TYPE-001-S04
  Scenario: Preserve arbitrary exact decimal digits in Go text
    Given plain decimal text with a scale and digits beyond binary floating-point precision
    When I parse and encode the Go decimal value
    Then the canonical text round-trips every significant digit and preserves the original scale
    And invalid syntax and out-of-bound precision leave a previously decoded value unchanged


  @MODEL-TYPE-001-S05
  Scenario Outline: Bind required and nullable Code First decimals
    Given the <provider> target and required Decimal and optional pointer fields with explicit precision and scale
    When I bind the mapped fields and use their codecs to write and read exact decimals in a disposable table
    Then the catalog declares the bound storage types and nullability
    And a separate connection reads SQL NULL and exact signed boundary values at their declared scales
    And the version 1 model snapshots retain the decimal facets and preserve bytes for non-decimal fields

    Examples:
      | provider  |
      | postgres  |
      | sqlite    |
      | sqlserver |
      | mysql     |

  @MODEL-TYPE-001-S06
  Scenario: Reject invalid Code First decimal declarations and values
    Given missing, conflicting, invalid or dialect-incompatible decimal facets
    When I map and bind the declared fields
    Then no invalid model is returned and a typed invalid-model error gives positions without revealing raw identifiers
    And a valid field codec rejects values that would be rounded or overflow before database dispatch
