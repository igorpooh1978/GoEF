package goeferr

import "errors"

// SaveChangesFailedID identifies a save batch callback failure.
const SaveChangesFailedID ID = "GOEF-TX-0004"

// ErrSaveChangesFailed identifies a callback that could not complete a save.
var ErrSaveChangesFailed = errors.New("The save batch could not complete.")

// NewSaveChangesFailed returns safe details about a save callback failure.
func NewSaveChangesFailed() *ErrorDetail {
	return newDetail(SaveChangesFailedID, PhaseExecute, RetryNever, OutcomeInspectTransaction, ErrSaveChangesFailed)
}
