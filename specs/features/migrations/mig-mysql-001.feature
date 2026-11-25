@MIG-MYSQL-001
Feature: Resolve partial migration steps across implicit MySQL commits

  @MIG-MYSQL-001-S01
  Scenario: Recover a completed DDL step across a lost checkpoint
    Given an InnoDB migration has two implicit-commit DDL steps and a durable intent for the first
    When the session is lost after the first ALTER succeeds and before its completion checkpoint
    Then a new lock owner inspects the exact column definition and existing row values
    And only the remaining validated step executes
    And one history row records the fully verified target

  @MIG-MYSQL-001-S02
  Scenario: Stop on conflicting postconditions
    Given an InnoDB journal indicates a DDL intent and the live column matches neither planned definition
    When the executor verifies the interrupted migration
    Then it returns ErrMigrationRecoveryRequired with the migration and operation identifiers
    And it issues no further DDL and creates no applied history row
