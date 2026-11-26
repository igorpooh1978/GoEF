package goeferr_test

import (
	"errors"
	"testing"

	"github.com/igorpooh1978/GoEF/goeferr"
)

func TestSavepointDetailsMatchTheErrorRegistry(t *testing.T) {
	for _, tc := range []struct {
		id      goeferr.ID
		kind    error
		detail  *goeferr.ErrorDetail
		phase   goeferr.Phase
		outcome goeferr.Outcome
		message string
	}{
		{"GOEF-CAP-0001", goeferr.ErrUnsupportedFeature, goeferr.NewUnsupportedFeature(), "preflight", "no_commit_attempted", "The requested capability is unavailable."},
		{"GOEF-TX-0002", goeferr.ErrSavepointFailed, goeferr.NewSavepointFailed(), "savepoint", "inspect_transaction", "The savepoint operation failed."},
	} {
		t.Run(string(tc.id), func(t *testing.T) {
			err := tc.detail
			if !errors.Is(err, tc.kind) || err.ID() != tc.id || err.Phase() != tc.phase ||
				err.Outcome() != tc.outcome || err.RetryClass() != "never" || err.Error() != tc.message {
				t.Fatalf("detail for %s = %v", tc.id, err)
			}
		})
	}
}
