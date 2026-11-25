@ERR-CONTRACT-001
Feature: Keep portable error identities stable and diagnostics safe

  @ERR-CONTRACT-001-S01
  Scenario: Round-trip the public classification for every introduced kind
    Given a reviewed registry entry with a planned producing test
    When its documented trigger is produced and the returned error is wrapped
    Then errors.Is matches the registered sentinel and errors.As exposes its typed detail
    And the stable code, operation, phase, outcome and retry class agree with the registry
    And the error, structured event and CLI JSON omit credentials, row values and raw vendor text

  @ERR-CONTRACT-001-S02
  Scenario Outline: Leave an unrecognized native code unclassified
    Given a <provider> classifier has no reviewed rule matching the native fields and phase
    When the adapter receives the error before any commit attempt
    Then it returns ErrUnknownDriver with the safe native metadata and retry class never
    And it reports neither not-found nor a constraint violation from text alone

    Examples:
      | provider  |
      | postgres  |
      | sqlite    |
      | sqlserver |
      | mysql     |

  @ERR-CONTRACT-001-S03
  Scenario: Demand real evidence for a native mapping
    Given a proposed native mapping backed only by manufacturer documentation
    When the mapping is reviewed for verified status
    Then the gate requires the exact engine and driver version, actual observed native fields, test and integrated SHA
    And an unobserved match or missing negative control keeps the mapping unverified
