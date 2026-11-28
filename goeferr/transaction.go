package goeferr

import "errors"

const (
	InvalidStateID         ID = "GOEF-STATE-0001"
	ContextBusyID          ID = "GOEF-STATE-0002"
	CanceledID             ID = "GOEF-IO-0003"
	DeadlineExceededID     ID = "GOEF-IO-0004"
	UnknownDriverID        ID = "GOEF-IO-0007"
	CommitOutcomeUnknownID ID = "GOEF-TX-0001"
	RollbackFailedID       ID = "GOEF-TX-0003"
	CommitRolledBackID     ID = "GOEF-TX-0005"
)

const (
	PhasePreflight Phase = "preflight"
	PhaseAcquire   Phase = "acquire"
	PhaseExecute   Phase = "execute"
	PhaseClassify  Phase = "classify"
	PhaseCommit    Phase = "commit"
	PhaseRollback  Phase = "rollback"
)

const (
	OutcomeInspectTransaction  Outcome = "inspect_transaction"
	OutcomeUnknown             Outcome = "unknown"
	OutcomeRollbackUnconfirmed Outcome = "rollback_unconfirmed"
	// OutcomeRolledBack identifies a confirmed rollback during commit.
	OutcomeRolledBack Outcome = "rolled_back"
)

// RetryResolveOutcome requires reconciliation before considering replay.
const RetryResolveOutcome RetryClass = "resolve_outcome"

var (
	// ErrInvalidState identifies an operation on an invalid or completed handle.
	ErrInvalidState = errors.New("The operation is invalid in the current state.")
	// ErrContextBusy identifies conflicting use of an active handle.
	ErrContextBusy = errors.New("The context is already in use.")
	// ErrCanceled identifies a causally canceled operation.
	ErrCanceled = errors.New("The operation was canceled.")
	// ErrDeadlineExceeded identifies a causal caller deadline.
	ErrDeadlineExceeded = errors.New("The caller deadline expired.")
	// ErrUnknownDriver identifies a driver failure without a safe classification.
	ErrUnknownDriver = errors.New("An unclassified database error occurred.")
	// ErrCommitOutcomeUnknown identifies an unconfirmed commit result.
	ErrCommitOutcomeUnknown = errors.New("The commit outcome is unknown.")
	// ErrCommitRolledBack identifies an explicitly confirmed rollback at commit.
	ErrCommitRolledBack = errors.New("The transaction rolled back during commit.")
	// ErrRollbackFailed identifies an unconfirmed rollback.
	ErrRollbackFailed = errors.New("Transaction rollback could not be confirmed.")
)

// NewInvalidState returns a safe detail for an invalid operation or handle.
func NewInvalidState() *ErrorDetail {
	return newDetail(InvalidStateID, PhasePreflight, RetryNever, OutcomeNoCommitAttempted, ErrInvalidState)
}

// NewContextBusy returns a safe detail for concurrent transaction use.
func NewContextBusy() *ErrorDetail {
	return newDetail(ContextBusyID, PhaseAcquire, RetryNever, OutcomeNoCommitAttempted, ErrContextBusy)
}

// NewCanceled returns a safe detail for causal cancellation.
func NewCanceled() *ErrorDetail {
	return newDetail(CanceledID, PhaseExecute, RetryNever, OutcomeInspectTransaction, ErrCanceled)
}

// NewDeadlineExceeded returns a safe detail for a causal caller deadline.
func NewDeadlineExceeded() *ErrorDetail {
	return newDetail(DeadlineExceededID, PhaseExecute, RetryNever, OutcomeInspectTransaction, ErrDeadlineExceeded)
}

// NewUnknownDriver returns a safe detail for an unclassified driver failure.
func NewUnknownDriver() *ErrorDetail {
	return newDetail(UnknownDriverID, PhaseClassify, RetryNever, OutcomeInspectTransaction, ErrUnknownDriver)
}

// NewCommitOutcomeUnknown returns a safe detail requiring outcome reconciliation.
func NewCommitOutcomeUnknown() *ErrorDetail {
	return newDetail(CommitOutcomeUnknownID, PhaseCommit, RetryResolveOutcome, OutcomeUnknown, ErrCommitOutcomeUnknown)
}

// NewCommitRolledBack reports that commit explicitly confirmed a rollback.
func NewCommitRolledBack() *ErrorDetail {
	return newDetail(CommitRolledBackID, PhaseCommit, RetryNever, OutcomeRolledBack, ErrCommitRolledBack)
}

// NewRollbackFailed returns a safe detail for an unconfirmed rollback.
func NewRollbackFailed() *ErrorDetail {
	return newDetail(RollbackFailedID, PhaseRollback, RetryResolveOutcome, OutcomeRollbackUnconfirmed, ErrRollbackFailed)
}

func newDetail(id ID, phase Phase, retry RetryClass, outcome Outcome, kind error) *ErrorDetail {
	return &ErrorDetail{data: errorData{
		id: id, phase: phase, retry: retry, outcome: outcome, message: kind.Error(), kind: kind,
	}}
}
