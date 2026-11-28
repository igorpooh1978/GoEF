@MIG-POSTGRES-001
Feature: Apply reviewed PostgreSQL migrations in Go

  @MIG-POSTGRES-001-S01
  Scenario: Apply an ordered suffix and verify an already applied target
    Given an empty PostgreSQL history and two registered artifacts with reviewed checks
    When a caller applies the first artifact and then applies the complete plan
    Then each call commits only the pending artifact and its exact history
    And reapplying the complete plan checks the target and reports zero applied artifacts

  @MIG-POSTGRES-001-S02
  Scenario: Refuse divergent history, legacy history and invalid targets
    Given PostgreSQL migration history with a changed or missing digest
    When the executor inspects the history and its expected target
    Then it reports a typed conflict without executing a migration
    And a failed target check reports typed recovery with history intact

  @MIG-POSTGRES-001-S03
  Scenario: A failed migration rolls back earlier work in the pending suffix
    Given a pending pair of PostgreSQL artifacts
    When the second artifact fails after executing DDL and DML
    Then neither artifact nor its history is committed
    And a PostgreSQL deferred constraint rejection also rolls back both artifacts
    And a later caller can explicitly apply the reviewed plan after verifying the state

  @MIG-POSTGRES-001-S04
  Scenario: Competing migrators recheck history after the transaction lock
    Given an owner holds the migration lock using a transaction in PostgreSQL
    When another migrator requests the same plan during the first apply
    Then it waits on a distinct PostgreSQL connection
    And after the owner commits the waiter sees the applied history and reports zero

  @MIG-POSTGRES-001-S05
  Scenario: A migrator can cancel or time out while waiting
    Given another PostgreSQL transaction holds the migration lock
    When a caller cancels its wait or its migration lock budget expires
    Then the Go error distinguishes cancellation from lock timeout
    And the owner retains the lock and its history remains untouched

  @MIG-POSTGRES-001-S06
  Scenario: A lost commit acknowledgement requires reconciliation
    Given an isolated PostgreSQL target receiving the migration transaction
    When the connection drops before the caller can confirm commit
    Then the executor returns a typed unknown commit outcome with no automatic retry
    And inspection through a fresh connection can establish the durable result
