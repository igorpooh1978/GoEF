package goeferr_test

import (
	"errors"
	"testing"

	"github.com/igorpooh1978/GoEF/goeferr"
)

func TestQueryDetails(t *testing.T) {
	for _, tc := range []struct {
		detail  *goeferr.ErrorDetail
		kind    error
		id      goeferr.ID
		phase   goeferr.Phase
		outcome goeferr.Outcome
	}{
		{goeferr.NewMultipleResults(), goeferr.ErrMultipleResults, "GOEF-QUERY-0002", "read-cardinality", "inspect_transaction"},
		{goeferr.NewQueryTranslation(), goeferr.ErrQueryTranslation, "GOEF-QUERY-0003", "translate", "no_commit_attempted"},
	} {
		t.Run(string(tc.id), func(t *testing.T) {
			if !errors.Is(tc.detail, tc.kind) || tc.detail.ID() != tc.id || tc.detail.Phase() != tc.phase ||
				tc.detail.Outcome() != tc.outcome || tc.detail.RetryClass() != goeferr.RetryNever {
				t.Fatal("query error no longer matches the registered policy")
			}
		})
	}
}
