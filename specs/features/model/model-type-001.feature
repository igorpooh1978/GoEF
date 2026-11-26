@MODEL-TYPE-001
Feature: Explicit provider type mapping

  @MODEL-TYPE-001-S01
  Scenario Outline: Resolve the requested exact decimal explicitly
    Given the <provider> capability target and a requested exact decimal precision and scale
    When the model resolves the property type
    Then a supported mapping defines an exact codec and the provider storage contract
    And a synthetic boundary value round-trips without conversion through floating point
    And an unsupported mapping produces a typed rejection before any schema operation

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
