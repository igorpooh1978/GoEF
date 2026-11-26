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
