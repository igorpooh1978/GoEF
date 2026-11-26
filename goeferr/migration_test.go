package goeferr_test

import (
	"errors"
	"testing"

	"github.com/igorpooh1978/GoEF/goeferr"
)

func TestMigrationExecutorDetailsMatchTheErrorRegistry(t *testing.T) {
	for _, tc := range []struct {
		id      goeferr.ID
		kind    error
		new     func() *goeferr.ErrorDetail
		phase   goeferr.Phase
		outcome goeferr.Outcome
		retry   goeferr.RetryClass
		message string
	}{
		{"GOEF-MIG-0003", goeferr.ErrMigrationLocked, goeferr.NewMigrationLocked, "lock", "no_commit_attempted", "never", "The migration lock could not be acquired."},
		{"GOEF-MIG-0006", goeferr.ErrMigrationRecoveryRequired, goeferr.NewMigrationRecoveryRequired, "recover", "inspect_durable_effects", "resolve_outcome", "Migration recovery is required."},
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
