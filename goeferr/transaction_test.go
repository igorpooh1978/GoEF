package goeferr_test

import (
	"errors"
	"testing"

	"github.com/igorpooh1978/GoEF/goeferr"
)

func TestTransactionDetailsMatchTheErrorRegistry(t *testing.T) {
	for _, tc := range []struct {
		id      goeferr.ID
		kind    error
		new     func() *goeferr.ErrorDetail
		phase   goeferr.Phase
		outcome goeferr.Outcome
		retry   goeferr.RetryClass
		message string
	}{
		{"GOEF-STATE-0001", goeferr.ErrInvalidState, goeferr.NewInvalidState, "preflight", "no_commit_attempted", "never", "The operation is invalid in the current state."},
		{"GOEF-STATE-0002", goeferr.ErrContextBusy, goeferr.NewContextBusy, "acquire", "no_commit_attempted", "never", "The context is already in use."},
		{"GOEF-IO-0003", goeferr.ErrCanceled, goeferr.NewCanceled, "execute", "inspect_transaction", "never", "The operation was canceled."},
		{"GOEF-IO-0004", goeferr.ErrDeadlineExceeded, goeferr.NewDeadlineExceeded, "execute", "inspect_transaction", "never", "The caller deadline expired."},
		{"GOEF-IO-0007", goeferr.ErrUnknownDriver, goeferr.NewUnknownDriver, "classify", "inspect_transaction", "never", "An unclassified database error occurred."},
		{"GOEF-TX-0001", goeferr.ErrCommitOutcomeUnknown, goeferr.NewCommitOutcomeUnknown, "commit", "unknown", "resolve_outcome", "The commit outcome is unknown."},
		{"GOEF-TX-0003", goeferr.ErrRollbackFailed, goeferr.NewRollbackFailed, "rollback", "rollback_unconfirmed", "resolve_outcome", "Transaction rollback could not be confirmed."},
	} {
		t.Run(string(tc.id), func(t *testing.T) {
			err := tc.new()
			if !errors.Is(err, tc.kind) || err.ID() != tc.id || err.Phase() != tc.phase ||
				err.Outcome() != tc.outcome || err.RetryClass() != tc.retry || err.Error() != tc.message {
				t.Fatalf("detail for %s = %v", tc.id, err)
			}
		})
	}
}
