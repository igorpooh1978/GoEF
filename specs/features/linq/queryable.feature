@LINQ-QUERYABLE-001
Feature: Execute immutable provider-backed plans

  @LINQ-QUERYABLE-001-S01
  Scenario: Query composition preserves independent branches
    Given two amount filters derived from one model-bound root
    When I compose one projection on each branch
    Then zero commands have executed
    And each branch contains only its own amount filter

  @LINQ-QUERYABLE-001-S02
  Scenario Outline: A bound filter executes in the existing transaction
    Given a <provider> transaction containing orders for two tenants
    When the current tenant requests orders above a bound amount
    Then the command executes on the transaction connection
    And only matching current-tenant orders are decoded
    And the amount is a typed parameter

    Examples:
      | provider   |
      | postgres   |
      | sqlite     |
      | sqlserver  |
      | mysql      |

  @LINQ-QUERYABLE-001-S03
  Scenario: Pagination distinguishes tied sort keys
    Given ordered orders with duplicate timestamps and unique IDs
    When I request two consecutive pages using timestamp and ID order
    Then their ID sets are disjoint
    And concatenating their IDs yields the same prefix as the unpaged query
