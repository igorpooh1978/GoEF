@ERR-CONSTRAINT-001
Feature: Classify constraints using native fields and operation context

  @ERR-CONSTRAINT-001-S01
  Scenario Outline: Identify an actual uniqueness violation
    Given a <provider> table with a named unique constraint and a committed row
    When a second insert duplicates its key
    Then errors.Is identifies ErrUniqueViolation through wrapping
    And errors.As exposes a redacted detail with the provider and observed native code
    And there is one persisted row and no automatic replay

    Examples:
      | provider  |
      | postgres  |
      | sqlite    |
      | sqlserver |
      | mysql     |

  @ERR-CONSTRAINT-001-S02
  Scenario Outline: Classify a foreign-key failure using its context
    Given a <provider> table with an enforced foreign key
    When an insert references a missing parent
    Then errors.Is identifies ErrForeignKeyViolation through wrapping
    And the detailed native evidence and constraint type distinguish other constraint failures
    And no invalid child exists and the error contains no inserted values

    Examples:
      | provider  |
      | postgres  |
      | sqlite    |
      | sqlserver |
      | mysql     |
