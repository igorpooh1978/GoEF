package goeferr

import "errors"

// MigrationConflictID identifies a migration lineage conflict.
const MigrationConflictID ID = "GOEF-MIG-0002"

// PhaseVerifyHistory denotes validation of migration history and definitions.
const PhaseVerifyHistory Phase = "verify-history"

// ErrMigrationConflict identifies malformed or divergent migration history or definitions.
var ErrMigrationConflict = errors.New("Migration history conflicts with the registered set.")

// NewMigrationConflict returns a safe detail for a migration lineage conflict.
func NewMigrationConflict() *ErrorDetail {
	return newDetail(MigrationConflictID, PhaseVerifyHistory, RetryNever, OutcomeNoCommitAttempted, ErrMigrationConflict)
}

const (
	// MigrationLockedID identifies a migration lock acquisition failure.
	MigrationLockedID ID = "GOEF-MIG-0003"
	// MigrationRecoveryRequiredID identifies an unproven durable migration state.
	MigrationRecoveryRequiredID ID = "GOEF-MIG-0006"
	// PhaseLock denotes acquisition of a migration lock.
	PhaseLock Phase = "lock"
	// PhaseRecover denotes verification of the durable migration state.
	PhaseRecover Phase = "recover"
	// OutcomeInspectDurableEffects requires inspection of durable migration effects.
	OutcomeInspectDurableEffects Outcome = "inspect_durable_effects"
)

// ErrMigrationLocked identifies a migration lock that could not be acquired.
var ErrMigrationLocked = errors.New("The migration lock could not be acquired.")

// ErrMigrationRecoveryRequired identifies an ambiguous durable migration state.
var ErrMigrationRecoveryRequired = errors.New("Migration recovery is required.")

// NewMigrationLocked returns a safe detail for a migration lock failure.
func NewMigrationLocked() *ErrorDetail {
	return newDetail(MigrationLockedID, PhaseLock, RetryNever, OutcomeNoCommitAttempted, ErrMigrationLocked)
}

// NewMigrationRecoveryRequired returns a safe detail for an unproven migration state.
func NewMigrationRecoveryRequired() *ErrorDetail {
	return newDetail(MigrationRecoveryRequiredID, PhaseRecover, RetryResolveOutcome, OutcomeInspectDurableEffects, ErrMigrationRecoveryRequired)
}
