package goeferr_test

import (
	"errors"
	"testing"

	"github.com/igorpooh1978/GoEF/goeferr"
)

type privateError []string

func (privateError) Error() string { return "private driver diagnostic" }

func TestProviderFailureMetadata(t *testing.T) {
	for _, tc := range []struct {
		name         string
		input, kind  error
		id           goeferr.ID
		provider     string
		phase        goeferr.Phase
		code         string
		retry        goeferr.RetryClass
		outcome      goeferr.Outcome
		wantProvider string
		wantPhase    goeferr.Phase
		wantCode     string
	}{
		{"unique", goeferr.ErrUniqueViolation, goeferr.ErrUniqueViolation, goeferr.UniqueViolationID, "postgres", goeferr.PhaseWrite, "23505", goeferr.RetryNever, goeferr.OutcomeInspectTransaction, "postgres", goeferr.PhaseWrite, "23505"},
		{"deadlock", goeferr.ErrDeadlock, goeferr.ErrDeadlock, goeferr.DeadlockID, "postgres", goeferr.PhaseRead, "40P01", goeferr.RetryTransactionSafe, goeferr.OutcomeInspectTransaction, "postgres", goeferr.PhaseRead, "40P01"},
		{"locking", goeferr.ErrLockUnavailable, goeferr.ErrLockUnavailable, goeferr.LockUnavailableID, "sqlite", goeferr.PhaseRead, "6", goeferr.RetryNever, goeferr.OutcomeNoCommitAttempted, "sqlite", goeferr.PhaseRead, "6"},
		{"unknown", privateError{}, goeferr.ErrUnknownDriver, goeferr.UnknownDriverID, "secret-name", "private message", "bad code", goeferr.RetryNever, goeferr.OutcomeInspectTransaction, "", goeferr.PhaseClassify, ""},
		{"long code", nil, goeferr.ErrUnknownDriver, goeferr.UnknownDriverID, "mysql", goeferr.PhaseAcquire, "01234567890", goeferr.RetryNever, goeferr.OutcomeInspectTransaction, "mysql", goeferr.PhaseAcquire, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := goeferr.NewProviderFailure(tc.input, tc.provider, tc.phase, tc.code)
			if !errors.Is(got, tc.kind) || got.ID() != tc.id || got.RetryClass() != tc.retry ||
				got.Outcome() != tc.outcome || got.Provider() != tc.wantProvider ||
				got.Phase() != tc.wantPhase || got.NativeCode() != tc.wantCode ||
				got.Error() != tc.kind.Error() {
				t.Fatalf("unexpected provider failure: %+v", got)
			}
		})
	}
}
