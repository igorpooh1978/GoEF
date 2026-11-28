package goeferr

import "errors"

const (
	// MultipleResultsID identifies a second match for a single-result query.
	MultipleResultsID ID = "GOEF-QUERY-0002"
	// QueryTranslationID identifies a rejected query translation.
	QueryTranslationID ID = "GOEF-QUERY-0003"
	// PhaseReadCardinality identifies single-result validation.
	PhaseReadCardinality Phase = "read-cardinality"
	// PhaseTranslate identifies translation before command dispatch.
	PhaseTranslate Phase = "translate"
)

var (
	// ErrMultipleResults identifies a single-result read with more than one match.
	ErrMultipleResults = errors.New("More than one result matched.")
	// ErrQueryTranslation identifies a query that cannot be translated.
	ErrQueryTranslation = errors.New("The expression cannot be translated.")
)

// NewMultipleResults returns a redacted single-result cardinality error.
func NewMultipleResults() *ErrorDetail {
	return newDetail(MultipleResultsID, PhaseReadCardinality, RetryNever, OutcomeInspectTransaction, ErrMultipleResults)
}

// NewQueryTranslation returns a redacted error for a rejected query.
func NewQueryTranslation() *ErrorDetail {
	return newDetail(QueryTranslationID, PhaseTranslate, RetryNever, OutcomeNoCommitAttempted, ErrQueryTranslation)
}
