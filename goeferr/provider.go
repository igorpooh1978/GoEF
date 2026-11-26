package goeferr

import "errors"

const (
	UniqueViolationID       ID = "GOEF-CONS-0001"
	ForeignKeyViolationID   ID = "GOEF-CONS-0002"
	NotNullViolationID      ID = "GOEF-CONS-0003"
	CheckViolationID        ID = "GOEF-CONS-0004"
	DeadlockID              ID = "GOEF-CONC-0002"
	SerializationFailureID  ID = "GOEF-CONC-0003"
	LockUnavailableID       ID = "GOEF-CONC-0004"
	ConnectionUnavailableID ID = "GOEF-IO-0001"
	TimeoutID               ID = "GOEF-IO-0002"
	MaterializationID       ID = "GOEF-QUERY-0005"
)

const (
	PhaseRead        Phase = "read"
	PhaseWrite       Phase = "write"
	PhaseRelease     Phase = "release"
	PhaseMaterialize Phase = "materialize"
)

// RetryTransactionSafe permits consideration of replay only after a confirmed
// full rollback, a fresh transaction and tracker, and an explicit replay contract.
const RetryTransactionSafe RetryClass = "transaction_safe"

var (
	// ErrUniqueViolation identifies a violation of a primary or unique key.
	ErrUniqueViolation = errors.New("A uniqueness constraint was violated.")
	// ErrForeignKeyViolation identifies a referential constraint violation.
	ErrForeignKeyViolation = errors.New("A foreign key constraint was violated.")
	// ErrNotNullViolation identifies a required column that received NULL.
	ErrNotNullViolation = errors.New("A required column received NULL.")
	// ErrCheckViolation identifies a failed check constraint.
	ErrCheckViolation = errors.New("A check constraint was violated.")
	// ErrDeadlock identifies the operation selected as a deadlock victim.
	ErrDeadlock = errors.New("The transaction encountered a deadlock.")
	// ErrSerializationFailure identifies an engine-reported serialization conflict.
	ErrSerializationFailure = errors.New("The transaction could not be serialized.")
	// ErrLockUnavailable identifies an unavailable database lock.
	ErrLockUnavailable = errors.New("A required database lock is unavailable.")
	// ErrConnectionUnavailable identifies a failed connection or transport.
	ErrConnectionUnavailable = errors.New("The database connection is unavailable.")
	// ErrTimeout identifies a database time budget independent of a caller deadline.
	ErrTimeout = errors.New("The database operation timed out.")
	// ErrMaterialization identifies a result incompatible with its declared layout.
	ErrMaterialization = errors.New("A result could not be materialized.")
)

var providerKinds = map[error]ID{
	ErrUniqueViolation:       UniqueViolationID,
	ErrForeignKeyViolation:   ForeignKeyViolationID,
	ErrNotNullViolation:      NotNullViolationID,
	ErrCheckViolation:        CheckViolationID,
	ErrDeadlock:              DeadlockID,
	ErrSerializationFailure:  SerializationFailureID,
	ErrLockUnavailable:       LockUnavailableID,
	ErrConnectionUnavailable: ConnectionUnavailableID,
	ErrTimeout:               TimeoutID,
	ErrMaterialization:       MaterializationID,
	ErrCanceled:              CanceledID,
	ErrDeadlineExceeded:      DeadlineExceededID,
}

// NewProviderFailure constructs a registered, redacted provider error. Pass only
// the registry sentinel for kind. Native codes may be SQLSTATEs or decimal codes
// extracted from structured driver fields. Unknown kinds use ErrUnknownDriver.
func NewProviderFailure(kind error, provider string, phase Phase, code string) *ErrorDetail {
	// Compare sentinels directly. A caller-supplied error may be non-comparable.
	id := UnknownDriverID
	knownKind := ErrUnknownDriver
	for candidate, candidateID := range providerKinds {
		if kind == candidate {
			id, knownKind = candidateID, candidate
			break
		}
	}

	retry, outcome := RetryNever, OutcomeInspectTransaction
	switch knownKind {
	case ErrDeadlock, ErrSerializationFailure:
		retry = RetryTransactionSafe
	case ErrLockUnavailable, ErrConnectionUnavailable:
		outcome = OutcomeNoCommitAttempted
	}
	switch phase {
	case PhaseAcquire, PhaseRelease, PhaseRead, PhaseWrite, PhaseMaterialize:
	default:
		phase = PhaseClassify
	}
	detail := newDetail(id, phase, retry, outcome, knownKind)
	switch provider {
	case "postgres", "sqlite", "mysql", "sqlserver":
		detail.data.provider = provider
	}
	if len(code) > 0 && len(code) <= 10 && validNativeCode(code) {
		detail.data.nativeCode = code
	}
	return detail
}

func validNativeCode(code string) bool {
	for i := range len(code) {
		c := code[i]
		if c >= '0' && c <= '9' {
			continue
		}
		if len(code) != 5 || c < 'A' || c > 'Z' {
			return false
		}
	}
	return true
}

// Provider returns the recognized provider identifier when present.
func (e *ErrorDetail) Provider() string { return e.fields().provider }

// NativeCode returns the structured SQLSTATE or native number when present.
func (e *ErrorDetail) NativeCode() string { return e.fields().nativeCode }
