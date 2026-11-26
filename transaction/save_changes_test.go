package transaction_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"

	"github.com/igorpooh1978/GoEF/goeferr"
	"github.com/igorpooh1978/GoEF/transaction"
)

var saveInPostgres = transaction.SaveOptions{Dialect: transaction.DialectPostgreSQL, Savepoints: true}

func TestSaveChangesRecoversIgnoredStatementFailure(t *testing.T) {
	p, db := openSavepointProbe(t)
	tx := begin(t, t.Context(), db)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	p.onCommand = func(query string) {
		p.fail = nil
		if query == "FAIL" {
			p.fail = privateError
			cancel()
		}
	}
	key := int64(-1)
	saved, err := tx.SaveChanges(ctx, saveInPostgres, func(w *transaction.SaveWriter) error {
		if _, err := w.Exec("WRITE"); err != nil {
			return err
		}
		if err := transaction.StageValue(w, &key, int64(12)); err != nil {
			return err
		}
		_, _ = w.Exec("FAIL")
		*w = transaction.SaveWriter{}
		return nil
	})
	if saved != nil || err == nil || errors.Is(err, privateError) || key != -1 {
		t.Fatalf("failed save returned (%v, %v), key = %d", saved, err, key)
	}
	if len(p.commands) != 5 || !strings.HasPrefix(p.commands[3], "ROLLBACK TO SAVEPOINT ") ||
		!strings.HasPrefix(p.commands[4], "RELEASE SAVEPOINT ") {
		t.Fatalf("save did not recover with its own context: %v", p.commands)
	}
	checkState(t, tx, db, transaction.StateActive, 1)
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

func TestSaveChangesReceiptsRespectManualRewind(t *testing.T) {
	_, db := openSavepointProbe(t)
	tx := begin(t, t.Context(), db)
	var before, after int
	save := func(target *int) *transaction.SavedChanges {
		t.Helper()
		r, err := tx.SaveChanges(t.Context(), saveInPostgres, func(w *transaction.SaveWriter) error {
			return transaction.StageValue(w, target, 7)
		})
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	earlier := save(&before)
	point := newSavepoint(t, tx, transaction.DialectPostgreSQL)
	later := save(&after)
	if err := earlier.Accept(); !errors.Is(err, goeferr.ErrInvalidState) {
		t.Fatalf("accepted before commit: %v", err)
	}
	if err := point.Rollback(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := earlier.Accept(); err != nil || before != 7 {
		t.Fatalf("earlier receipt did not survive: %v, value %d", err, before)
	}
	if err := later.Accept(); !errors.Is(err, goeferr.ErrInvalidState) || after != 0 {
		t.Fatalf("accepted rewound receipt: %v, value %d", err, after)
	}
}

func TestSaveChangesUnconfirmedRecovery(t *testing.T) {
	for _, external := range []bool{false, true} {
		name := "owned"
		if external {
			name = "external"
		}
		t.Run(name, func(t *testing.T) {
			p, db := openSavepointProbe(t)
			p.onCommand = func(query string) {
				p.fail = nil
				if query == "FAIL" || strings.HasPrefix(query, "ROLLBACK TO ") {
					p.fail = privateError
				}
			}
			fail := func(w *transaction.SaveWriter) error {
				_, err := w.Exec("FAIL")
				return err
			}
			var err error
			if external {
				native, beginErr := db.BeginTx(t.Context(), nil)
				if beginErr != nil {
					t.Fatal(beginErr)
				}
				t.Cleanup(func() { _ = native.Rollback() })
				e, startErr := transaction.NewExternal(native)
				if startErr != nil {
					t.Fatal(startErr)
				}
				participant, enlistErr := e.Enlist()
				if enlistErr != nil {
					t.Fatal(enlistErr)
				}
				_, err = participant.SaveChanges(t.Context(), saveInPostgres, fail)
				if p.rollback.Load() != 0 || p.commit.Load() != 0 {
					t.Fatal("GoEF completed the caller transaction")
				}
				if _, err := e.Enlist(); !errors.Is(err, goeferr.ErrInvalidState) {
					t.Fatalf("failed coordinator still enlists: %v", err)
				}
				if err := native.Rollback(); err != nil {
					t.Fatal(err)
				}
			} else {
				tx := begin(t, t.Context(), db)
				_, err = tx.SaveChanges(t.Context(), saveInPostgres, fail)
				checkState(t, tx, db, transaction.StateRolledBack, 0)
				if p.rollback.Load() != 1 || !errors.Is(tx.Commit(), goeferr.ErrInvalidState) {
					t.Fatal("owned transaction remained committable after unconfirmed rewind")
				}
			}
			if !errors.Is(err, goeferr.ErrUnknownDriver) || !errors.Is(err, goeferr.ErrSavepointFailed) ||
				errors.Is(err, privateError) || errors.Is(err, sql.ErrTxDone) ||
				errors.Is(err, goeferr.ErrRollbackFailed) != external {
				t.Fatalf("did not preserve sanitized primary and recovery errors: %v", err)
			}
		})
	}
}

func TestSaveChangesDetachesEndedExternal(t *testing.T) {
	for _, dialect := range []transaction.Dialect{transaction.DialectPostgreSQL, transaction.DialectSQLServer} {
		t.Run(string(dialect), func(t *testing.T) {
			p, db := openSavepointProbe(t)
			native, err := db.BeginTx(t.Context(), nil)
			if err != nil {
				t.Fatal(err)
			}
			e, err := transaction.NewExternal(native)
			if err != nil {
				t.Fatal(err)
			}
			if err := native.Commit(); err != nil {
				t.Fatal(err)
			}
			saved, err := e.SaveChanges(t.Context(), transaction.SaveOptions{Dialect: dialect, Savepoints: true}, func(*transaction.SaveWriter) error {
				t.Fatal("callback ran after native commit")
				return nil
			})
			if saved != nil || !errors.Is(err, goeferr.ErrInvalidState) || errors.Is(err, sql.ErrTxDone) {
				t.Fatalf("save after caller commit = (%v, %v)", saved, err)
			}
			if _, err := e.Enlist(); !errors.Is(err, goeferr.ErrInvalidState) {
				t.Fatalf("ended coordinator still enlists: %v", err)
			}
			if p.rollback.Load() != 0 || len(p.commands) != 0 {
				t.Fatal("closed native transaction was reused")
			}
		})
	}
}

func TestSaveChangesPreflightKeepsTransactionActive(t *testing.T) {
	p, db := openSavepointProbe(t)
	native, err := db.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = native.Rollback() })
	e, err := transaction.NewExternal(native)
	if err != nil {
		t.Fatal(err)
	}
	participant, err := e.Enlist()
	if err != nil {
		t.Fatal(err)
	}
	if err := participant.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	unreachable := func(*transaction.SaveWriter) error {
		t.Fatal("preflight called the save callback")
		return nil
	}
	for _, tc := range []struct {
		name   string
		call   func() (*transaction.SavedChanges, error)
		target error
	}{
		{"invalid participant", func() (*transaction.SavedChanges, error) {
			return (*transaction.Participant)(nil).SaveChanges(t.Context(), saveInPostgres, unreachable)
		}, goeferr.ErrInvalidState},
		{"detached participant", func() (*transaction.SavedChanges, error) {
			return participant.SaveChanges(t.Context(), saveInPostgres, unreachable)
		}, goeferr.ErrInvalidState},
		{"nil context", func() (*transaction.SavedChanges, error) {
			return e.SaveChanges(nil, saveInPostgres, unreachable)
		}, goeferr.ErrInvalidState},
		{"canceled", func() (*transaction.SavedChanges, error) {
			return e.SaveChanges(ctx, saveInPostgres, unreachable)
		}, context.Canceled},
		{"nil callback", func() (*transaction.SavedChanges, error) {
			return e.SaveChanges(t.Context(), saveInPostgres, nil)
		}, goeferr.ErrInvalidState},
		{"savepoints disabled", func() (*transaction.SavedChanges, error) {
			return e.SaveChanges(t.Context(), transaction.SaveOptions{Dialect: transaction.DialectPostgreSQL}, unreachable)
		}, goeferr.ErrUnsupportedFeature},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if saved, err := tc.call(); saved != nil || !errors.Is(err, tc.target) || len(p.commands) != 0 {
				t.Fatalf("preflight returned (%v, %v), commands %v", saved, err, p.commands)
			}
		})
	}
	if err := native.Commit(); err != nil {
		t.Fatal(err)
	}
}

func TestSaveChangesCreationBadConnectionAbortsOwned(t *testing.T) {
	p, db := openSavepointProbe(t)
	tx := begin(t, t.Context(), db)
	p.fail = driver.ErrBadConn
	saved, err := tx.SaveChanges(t.Context(), saveInPostgres, func(*transaction.SaveWriter) error {
		t.Fatal("callback ran without a savepoint")
		return nil
	})
	if saved != nil || !errors.Is(err, goeferr.ErrSavepointFailed) || errors.Is(err, driver.ErrBadConn) {
		t.Fatalf("create failure returned (%v, %v)", saved, err)
	}
	checkState(t, tx, db, transaction.StateFailed, 0)
	if p.close.Load() != 1 || len(p.commands) != 1 {
		t.Fatal("bad connection was reused or did not discard")
	}
}

func TestSaveChangesReleaseFailureCanRecover(t *testing.T) {
	p, db := openSavepointProbe(t)
	native, err := db.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = native.Rollback() })
	e, err := transaction.NewExternal(native)
	if err != nil {
		t.Fatal(err)
	}
	failFirstRelease := true
	p.onCommand = func(query string) {
		p.fail = nil
		if strings.HasPrefix(query, "RELEASE ") && failFirstRelease {
			failFirstRelease = false
			p.fail = privateError
		}
	}
	saved, err := e.SaveChanges(t.Context(), saveInPostgres, func(w *transaction.SaveWriter) error {
		_, err := w.Exec("WRITE")
		return err
	})
	if saved != nil || !errors.Is(err, goeferr.ErrSavepointFailed) || errors.Is(err, goeferr.ErrRollbackFailed) || errors.Is(err, privateError) {
		t.Fatalf("recoverable release failure returned (%v, %v)", saved, err)
	}
	if _, err := e.Enlist(); err != nil || len(p.commands) != 5 {
		t.Fatalf("coordinator did not survive recovered release: %v; commands %v", err, p.commands)
	}
}

func TestSaveChangesRecoversCallbackFailureAndPanic(t *testing.T) {
	p, db := openSavepointProbe(t)
	tx := begin(t, t.Context(), db)
	for _, tc := range []struct {
		name   string
		cause  error
		target error
	}{
		{"private", privateError, goeferr.ErrSaveChangesFailed},
		{"canceled", context.Canceled, context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			saved, err := tx.SaveChanges(t.Context(), saveInPostgres, func(*transaction.SaveWriter) error { return tc.cause })
			if saved != nil || !errors.Is(err, tc.target) || errors.Is(err, privateError) {
				t.Fatalf("callback returned (%v, %v)", saved, err)
			}
		})
	}
	func() {
		defer func() {
			if recovered := recover(); recovered != privateError {
				t.Fatal("batch did not preserve the caller's panic")
			}
		}()
		_, _ = tx.SaveChanges(t.Context(), saveInPostgres, func(*transaction.SaveWriter) error { panic(privateError) })
	}()
	if len(p.commands) != 9 || !strings.HasPrefix(p.commands[7], "ROLLBACK TO SAVEPOINT ") {
		t.Fatalf("failed callbacks did not rewind: %v", p.commands)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}
