@LINQ-CONFORMANCE-001
Feature: Compare matched LINQ workloads

  @LINQ-CONFORMANCE-001-S01
  Scenario: Record a pinned local differential fixture
    Given duplicate integer keys with a zero value and an empty sequence
    When equivalent local cases run on GoEF and each available pinned reference
    Then the report records the input, comparator, ordered result and absence or error
    And unavailable references have an explicit reason and zero passing cases

  @LINQ-CONFORMANCE-001-S02
  Scenario Outline: A matched database fixture records provider-specific semantics
    Given nullable join keys and text under an explicit collation on <provider>
    When the corresponding EF and GoEF fixture executes on the same engine version
    Then the report compares row multiplicities, null placement and ordered IDs
    And records bound parameters, command counts, engine settings and errors

    Examples:
      | provider   |
      | postgres   |
      | sqlite     |
      | sqlserver  |
      | mysql      |

  @LINQ-CONFORMANCE-001-S03
  Scenario: A benchmark verifies equivalent work before timing
    Given identical seeded input and a defined early-stop position
    When native Go, GoEF and pinned go-linq complete the fixture
    Then their observed output and consumed prefixes match the fixture
    And each measured run records toolchain, hardware, sample count, time and allocations
