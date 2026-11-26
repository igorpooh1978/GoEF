// Package goeferr exposes portable error classifications and safe details.
package goeferr

import "errors"

// ID identifies a portable error in the GoEF error registry.
type ID string

// InvalidModelID identifies the invalid-model classification.
const InvalidModelID ID = "GOEF-MODEL-0001"

// Phase describes where an error arose.
type Phase string

// PhaseValidateModel denotes validation of a logical model.
const PhaseValidateModel Phase = "validate-model"

// RetryClass describes the classification's retry policy.
type RetryClass string

// RetryNever denotes a classification that prohibits automatic replay.
const RetryNever RetryClass = "never"

// Outcome describes the known commit outcome.
type Outcome string

// OutcomeNoCommitAttempted denotes an operation that never attempted a commit.
const OutcomeNoCommitAttempted Outcome = "no_commit_attempted"

const invalidModelMessage = "Model validation failed."

// ErrInvalidModel identifies incompatible model declarations.
var ErrInvalidModel = errors.New(invalidModelMessage)

// ErrorDetail exposes a portable classification and its safe metadata.
// An uninitialized detail has no classification or metadata.
type ErrorDetail struct {
	data errorData
}

type errorData struct {
	id         ID
	phase      Phase
	retry      RetryClass
	outcome    Outcome
	message    string
	kind       error
	provider   string
	nativeCode string
}

// NewInvalidModel returns the portable error for a failed logical model validation.
func NewInvalidModel() *ErrorDetail {
	return &ErrorDetail{data: errorData{
		id:      InvalidModelID,
		phase:   PhaseValidateModel,
		retry:   RetryNever,
		outcome: OutcomeNoCommitAttempted,
		message: invalidModelMessage,
		kind:    ErrInvalidModel,
	}}
}

func (e *ErrorDetail) fields() errorData {
	if e == nil {
		return errorData{}
	}
	return e.data
}

// Error returns the safe public summary for the classification.
func (e *ErrorDetail) Error() string {
	if message := e.fields().message; message != "" {
		return message
	}
	return "goeferr: uninitialized error detail"
}

// Unwrap exposes the portable sentinel for errors.Is.
func (e *ErrorDetail) Unwrap() error { return e.fields().kind }

// ID returns the registry identifier of the classification.
func (e *ErrorDetail) ID() ID { return e.fields().id }

// Phase returns the operation phase where the error arose.
func (e *ErrorDetail) Phase() Phase { return e.fields().phase }

// RetryClass returns the classification's retry policy.
func (e *ErrorDetail) RetryClass() RetryClass { return e.fields().retry }

// Outcome returns the known commit outcome.
func (e *ErrorDetail) Outcome() Outcome { return e.fields().outcome }
