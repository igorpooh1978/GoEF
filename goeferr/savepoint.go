package goeferr

import "errors"

const (
	// UnsupportedFeatureID identifies an unavailable capability.
	UnsupportedFeatureID ID = "GOEF-CAP-0001"
	// SavepointFailedID identifies a failed savepoint operation.
	SavepointFailedID ID = "GOEF-TX-0002"
	// PhaseSavepoint identifies native savepoint execution.
	PhaseSavepoint Phase = "savepoint"
)

var (
	// ErrUnsupportedFeature identifies an unavailable capability.
	ErrUnsupportedFeature = errors.New("The requested capability is unavailable.")
	// ErrSavepointFailed identifies failed native savepoint execution.
	ErrSavepointFailed = errors.New("The savepoint operation failed.")
)

// NewUnsupportedFeature returns safe detail for an unavailable capability.
func NewUnsupportedFeature() *ErrorDetail {
	return newDetail(UnsupportedFeatureID, PhasePreflight, RetryNever, OutcomeNoCommitAttempted, ErrUnsupportedFeature)
}

// NewSavepointFailed returns safe detail for a failed native savepoint operation.
func NewSavepointFailed() *ErrorDetail {
	return newDetail(SavepointFailedID, PhaseSavepoint, RetryNever, OutcomeInspectTransaction, ErrSavepointFailed)
}
