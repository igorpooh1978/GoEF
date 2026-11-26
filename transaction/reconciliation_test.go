package transaction_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/igorpooh1978/GoEF/goeferr"
	"github.com/igorpooh1978/GoEF/transaction"
)

func TestReconcileCommitRequiresUnknownOutcome(t *testing.T) {
	p := new(probe)
	db := openProbe(t, p)
	verifier := func(context.Context) (transaction.CommitVerification, error) {
		t.Error("verification ran without an ambiguous commit")
		return transaction.CommitConfirmed, nil
	}
	for _, tx := range []*transaction.Tx{nil, {}, begin(t, t.Context(), db)} {
		if err := tx.ReconcileCommit(t.Context(), verifier); !errors.Is(err, goeferr.ErrInvalidState) {
			t.Fatalf("reconciliation without ambiguity: %v", err)
		}
	}
}

func TestReconcileCommitConfirmsOneOutcome(t *testing.T) {
	for _, tc := range []struct {
		name         string
		verification transaction.CommitVerification
		state        transaction.State
	}{
		{"committed", transaction.CommitConfirmed, transaction.StateCommitted},
		{"rolled back", transaction.RollbackConfirmed, transaction.StateRolledBack},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &probe{commitErr: privateError}
			db := openProbe(t, p)
			lifetime, cancel := context.WithCancel(t.Context())
			defer cancel()
			tx := begin(t, lifetime, db)

			var published int
			save := func(value int) *transaction.SavedChanges {
				t.Helper()
				saved, err := tx.SaveChanges(t.Context(), saveInPostgres, func(w *transaction.SaveWriter) error {
					return transaction.StageValue(w, &published, value)
				})
				if err != nil {
					t.Fatal(err)
				}
				return saved
			}

			intact := save(7)
			point := newSavepoint(t, tx, transaction.DialectPostgreSQL)
			rewound := save(21)
			if err := point.Rollback(t.Context()); err != nil {
				t.Fatal(err)
			}
			if err := tx.Commit(); !errors.Is(err, goeferr.ErrCommitOutcomeUnknown) {
				t.Fatal(err)
			}
			cancel()

			calls := 0
			verifier := func(ctx context.Context) (transaction.CommitVerification, error) {
				calls++
				if ctx.Err() != nil || tx.State() != transaction.StateOutcomeUnknown ||
					p.close.Load() != 1 || db.Stats().InUse != 0 {
					t.Error("verifier must run after discard with its own live context")
				}
				if err := intact.Accept(); !errors.Is(err, goeferr.ErrInvalidState) {
					t.Errorf("accepted while verification was in progress: %v", err)
				}
				return tc.verification, nil
			}

			copied := *tx
			if err := copied.ReconcileCommit(t.Context(), verifier); err != nil {
				t.Fatal(err)
			}
			checkState(t, tx, db, tc.state, 0)
			if published != 0 {
				t.Fatal("reconciliation applied staged values without explicit acceptance")
			}
			if err := rewound.Accept(); !errors.Is(err, goeferr.ErrInvalidState) {
				t.Fatalf("accepted rewound receipt after reconciliation: %v", err)
			}
			switch err := intact.Accept(); tc.state {
			case transaction.StateCommitted:
				if err != nil || published != 7 {
					t.Fatalf("confirmed commit acceptance = %v, published = %d", err, published)
				}
			case transaction.StateRolledBack:
				if !errors.Is(err, goeferr.ErrInvalidState) || published != 0 {
					t.Fatalf("confirmed rollback acceptance = %v, published = %d", err, published)
				}
			}
			for _, err := range []error{intact.Accept(), tx.ReconcileCommit(t.Context(), verifier), tx.Commit(), tx.Rollback()} {
				if !errors.Is(err, goeferr.ErrInvalidState) {
					t.Fatalf("terminal operation was accepted: %v", err)
				}
			}
			if calls != 1 || p.commit.Load() != 1 || p.rollback.Load() != 0 || p.open.Load() != 1 {
				t.Fatal("reconciliation replayed native completion or acquired a connection")
			}
		})
	}
}

func TestReconcileCommitKeepsUnverifiedState(t *testing.T) {
	p := &probe{commitErr: privateError}
	db := openProbe(t, p)
	tx := begin(t, t.Context(), db)
	if err := tx.Commit(); !errors.Is(err, goeferr.ErrCommitOutcomeUnknown) {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	deadline, stop := context.WithTimeout(t.Context(), 0)
	defer stop()

	for _, tc := range []struct {
		name       string
		ctx        context.Context
		verifier   transaction.CommitVerifier
		want       error
		additional error
	}{
		{"nil context", nil, func(context.Context) (transaction.CommitVerification, error) {
			t.Error("nil context did not prevent verification")
			return transaction.CommitConfirmed, nil
		}, goeferr.ErrInvalidState, nil},
		{"missing verifier", t.Context(), nil, goeferr.ErrCommitOutcomeUnknown, nil},
		{"canceled before reading", cancelled, func(context.Context) (transaction.CommitVerification, error) {
			t.Error("canceled context did not prevent verification")
			return transaction.CommitConfirmed, nil
		}, goeferr.ErrCommitOutcomeUnknown, context.Canceled},
		{"deadline before reading", deadline, func(context.Context) (transaction.CommitVerification, error) {
			t.Error("expired context did not prevent verification")
			return transaction.CommitConfirmed, nil
		}, goeferr.ErrCommitOutcomeUnknown, context.DeadlineExceeded},
		{"missing authority", t.Context(), func(context.Context) (transaction.CommitVerification, error) {
			return transaction.CommitUnverified, nil
		}, goeferr.ErrCommitOutcomeUnknown, nil},
		{"private verifier error", t.Context(), func(context.Context) (transaction.CommitVerification, error) {
			return transaction.CommitConfirmed, privateError
		}, goeferr.ErrCommitOutcomeUnknown, nil},
		{"verifier cancellation", t.Context(), func(context.Context) (transaction.CommitVerification, error) {
			return transaction.CommitConfirmed, errors.Join(context.Canceled, privateError)
		}, goeferr.ErrCommitOutcomeUnknown, context.Canceled},
		{"unknown verification", t.Context(), func(context.Context) (transaction.CommitVerification, error) {
			return transaction.CommitVerification(100), nil
		}, goeferr.ErrCommitOutcomeUnknown, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tx.ReconcileCommit(tc.ctx, tc.verifier)
			if !errors.Is(err, tc.want) || (tc.additional != nil && !errors.Is(err, tc.additional)) ||
				errors.Is(err, privateError) || strings.Contains(err.Error(), privateError.Error()) {
				t.Fatalf("unsafe verification error: %v", err)
			}
			checkState(t, tx, db, transaction.StateOutcomeUnknown, 0)
		})
	}

	ctx, cancelDuringVerification := context.WithCancel(t.Context())
	defer cancelDuringVerification()
	err := tx.ReconcileCommit(ctx, func(ctx context.Context) (transaction.CommitVerification, error) {
		cancelDuringVerification()
		<-ctx.Done()
		return transaction.CommitConfirmed, nil
	})
	if !errors.Is(err, goeferr.ErrCommitOutcomeUnknown) || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled verification returned %v", err)
	}
	checkState(t, tx, db, transaction.StateOutcomeUnknown, 0)

	if err := tx.ReconcileCommit(t.Context(), func(context.Context) (transaction.CommitVerification, error) {
		return transaction.CommitConfirmed, nil
	}); err != nil {
		t.Fatalf("authoritative recheck after unavailable verifier: %v", err)
	}
	checkState(t, tx, db, transaction.StateCommitted, 0)
}

func TestReconcileCommitSerializesVerification(t *testing.T) {
	db := openProbe(t, &probe{commitErr: privateError})
	tx := begin(t, t.Context(), db)
	if err := tx.Commit(); !errors.Is(err, goeferr.ErrCommitOutcomeUnknown) {
		t.Fatal(err)
	}
	entered := make(chan struct{})
	proceed := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- tx.ReconcileCommit(t.Context(), func(ctx context.Context) (transaction.CommitVerification, error) {
			close(entered)
			select {
			case <-proceed:
				return transaction.CommitConfirmed, nil
			case <-ctx.Done():
				return transaction.CommitUnverified, ctx.Err()
			}
		})
	}()
	defer func() {
		close(proceed)
		select {
		case err := <-done:
			if err != nil || tx.State() != transaction.StateCommitted {
				t.Errorf("serialized verification finished with %v in %s", err, tx.State())
			}
		case <-time.After(2 * time.Second):
			t.Error("verification did not finish")
		}
	}()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("verification did not start")
	}
	if err := tx.ReconcileCommit(t.Context(), func(context.Context) (transaction.CommitVerification, error) {
		t.Error("concurrent call received verification ownership")
		return transaction.RollbackConfirmed, nil
	}); !errors.Is(err, goeferr.ErrContextBusy) {
		t.Fatalf("overlapping verification returned %v", err)
	}
}
