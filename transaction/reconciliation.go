package transaction

import (
	"context"
	"errors"

	"github.com/igorpooh1978/GoEF/goeferr"
)

// CommitVerification describes the authoritative result of checking an
// ambiguous commit.
type CommitVerification uint8

const (
	// CommitUnverified leaves the outcome unknown.
	CommitUnverified CommitVerification = iota
	// CommitConfirmed requires evidence that the exact unit committed.
	CommitConfirmed
	// RollbackConfirmed requires evidence that the prior unit ended without
	// committing. Absence of a marker alone does not establish that outcome.
	RollbackConfirmed
)

// CommitVerifier checks the exact unit using a fresh connection to an
// authoritative primary. For example, a matching operation marker and
// payload, written atomically with the unit, can confirm a commit. A negative
// read can confirm rollback only after the prior unit is known to be final.
// The verifier owns its resources and must honor its context. The old native
// transaction and its affected connection have already been released.
type CommitVerifier func(context.Context) (CommitVerification, error)

// ReconcileCommit checks an owned transaction after Commit reported an unknown
// outcome and discarded its connection. A confirmed result updates [Tx.State] and
// returns nil. A confirmed commit allows explicit acceptance of valid saved
// changes. A confirmed rollback keeps all staged values unaccepted.
//
// A missing or inconclusive verifier, a verification failure, or cancellation
// leaves StateOutcomeUnknown and returns [goeferr.ErrCommitOutcomeUnknown].
// The caller can try verification again using a fresh context. Verification
// runs once per call at most, with no native completion or replay.
func (tx *Tx) ReconcileCommit(ctx context.Context, verify CommitVerifier) error {
	if ctx == nil {
		return goeferr.NewInvalidState()
	}
	owned, err := tx.lockState(StateOutcomeUnknown)
	if err != nil {
		return err
	}
	defer owned.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return errors.Join(goeferr.NewCommitOutcomeUnknown(), contextCause(err))
	}
	if verify == nil {
		return goeferr.NewCommitOutcomeUnknown()
	}
	result, err := verify(ctx)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return errors.Join(goeferr.NewCommitOutcomeUnknown(), contextCause(ctxErr))
	}
	if err != nil {
		return errors.Join(goeferr.NewCommitOutcomeUnknown(), contextCause(err))
	}
	switch result {
	case CommitConfirmed:
		owned.state.Store(StateCommitted)
	case RollbackConfirmed:
		owned.state.Store(StateRolledBack)
	default:
		return goeferr.NewCommitOutcomeUnknown()
	}
	return nil
}
