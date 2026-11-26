package transaction_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/igorpooh1978/GoEF/goeferr"
	"github.com/igorpooh1978/GoEF/transaction"
)

type savepointProbe struct {
	*probe
	commands  []string
	fail      error
	onCommand func(string)
}

func (p *savepointProbe) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := p.probe.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &savepointConn{probeConn: conn.(*probeConn), probe: p}, nil
}

type savepointConn struct {
	*probeConn
	probe *savepointProbe
}

func (c *savepointConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	c.probe.commands = append(c.probe.commands, query)
	if c.probe.onCommand != nil {
		c.probe.onCommand(query)
	}
	if c.probe.fail != nil {
		return nil, c.probe.fail
	}
	return c.probeConn.ExecContext(ctx, query, args)
}

func openSavepointProbe(t *testing.T) (*savepointProbe, *sql.DB) {
	t.Helper()
	p := &savepointProbe{probe: new(probe)}
	db := sql.OpenDB(p)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	return p, db
}

func newSavepoint(t *testing.T, tx *transaction.Tx, dialect transaction.Dialect) *transaction.Savepoint {
	t.Helper()
	point, err := tx.Savepoint(t.Context(), dialect)
	if err != nil {
		t.Fatal(err)
	}
	return point
}

func TestManagedSavepointStackAndCommands(t *testing.T) {
	names := regexp.MustCompile(`^goef_[A-Z2-7]{26}$`)
	for _, tc := range []struct {
		name                      string
		dialect                   transaction.Dialect
		create, rollback, release string
	}{
		{"postgres", transaction.DialectPostgreSQL, "SAVEPOINT ", "ROLLBACK TO SAVEPOINT ", "RELEASE SAVEPOINT "},
		{"sqlite", transaction.DialectSQLite, "SAVEPOINT ", "ROLLBACK TO SAVEPOINT ", "RELEASE SAVEPOINT "},
		{"sqlserver", transaction.DialectSQLServer, "SAVE TRANSACTION ", "ROLLBACK TRANSACTION ", ""},
		{"mysql", transaction.DialectMySQL, "SAVEPOINT ", "ROLLBACK TO SAVEPOINT ", "RELEASE SAVEPOINT "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, db := openSavepointProbe(t)
			tx := begin(t, t.Context(), db)
			outer := newSavepoint(t, tx, tc.dialect)
			inner := newSavepoint(t, tx, tc.dialect)
			if len(p.commands) != 2 {
				t.Fatalf("creation dispatched %d SQL commands, want 2", len(p.commands))
			}
			outerName, outerOK := strings.CutPrefix(p.commands[0], tc.create)
			innerName, innerOK := strings.CutPrefix(p.commands[1], tc.create)
			if !outerOK || !innerOK || !names.MatchString(outerName) || !names.MatchString(innerName) || outerName == innerName {
				t.Fatalf("unsafe or duplicate savepoint identifiers in %q", p.commands)
			}
			if err := outer.Release(t.Context()); !errors.Is(err, goeferr.ErrInvalidState) {
				t.Fatalf("release of non-innermost point = %v", err)
			}
			if err := outer.Rollback(t.Context()); err != nil {
				t.Fatal(err)
			}
			if err := inner.Rollback(t.Context()); !errors.Is(err, goeferr.ErrInvalidState) {
				t.Fatalf("rollback to invalidated point = %v", err)
			}
			if err := inner.Release(t.Context()); !errors.Is(err, goeferr.ErrInvalidState) {
				t.Fatalf("release of invalidated point = %v", err)
			}
			if err := outer.Rollback(t.Context()); err != nil {
				t.Fatalf("outer point did not survive rollback: %v", err)
			}
			clone := *outer
			if err := clone.Release(t.Context()); err != nil {
				t.Fatal(err)
			}
			if err := outer.Rollback(t.Context()); !errors.Is(err, goeferr.ErrInvalidState) {
				t.Fatalf("rollback after release = %v", err)
			}
			if err := outer.Release(t.Context()); !errors.Is(err, goeferr.ErrInvalidState) {
				t.Fatalf("duplicate release = %v", err)
			}
			expected := []string{tc.create + outerName, tc.create + innerName, tc.rollback + outerName, tc.rollback + outerName}
			if tc.dialect == transaction.DialectSQLServer {
				expected = []string{tc.create + outerName, tc.create + innerName, tc.rollback + outerName, tc.create + outerName, tc.rollback + outerName, tc.create + outerName}
			}
			if tc.release != "" {
				expected = append(expected, tc.release+outerName)
			}
			if !slices.Equal(p.commands, expected) || p.open.Load() != 1 || p.commit.Load() != 0 || p.rollback.Load() != 0 {
				t.Fatalf("unexpected dispatch or completion: %q", p.commands)
			}
			if err := tx.Commit(); err != nil {
				t.Fatal(err)
			}
			if _, err := tx.Savepoint(t.Context(), tc.dialect); !errors.Is(err, goeferr.ErrInvalidState) {
				t.Fatalf("savepoint after commit = %v", err)
			}
			checkState(t, tx, db, transaction.StateCommitted, 0)
		})
	}
}

func TestSavepointPreflightDoesNotDispatchOrInvalidate(t *testing.T) {
	p, db := openSavepointProbe(t)
	tx := begin(t, t.Context(), db)
	if _, err := tx.Savepoint(t.Context(), "other"); !errors.Is(err, goeferr.ErrUnsupportedFeature) || len(p.commands) != 0 {
		t.Fatalf("unknown dialect = %v, dispatches = %d", err, len(p.commands))
	}
	if _, err := tx.Savepoint(nil, transaction.DialectPostgreSQL); !errors.Is(err, goeferr.ErrInvalidState) {
		t.Fatalf("nil context = %v", err)
	}
	outer := newSavepoint(t, tx, transaction.DialectPostgreSQL)
	if _, err := tx.Savepoint(t.Context(), transaction.DialectSQLite); !errors.Is(err, goeferr.ErrUnsupportedFeature) {
		t.Fatalf("conflicting dialect = %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := outer.Rollback(ctx); !errors.Is(err, context.Canceled) || errors.Is(err, goeferr.ErrSavepointFailed) {
		t.Fatalf("canceled rollback before dispatch = %v", err)
	}
	if len(p.commands) != 1 {
		t.Fatalf("preflight reached the driver: %q", p.commands)
	}
	if err := outer.Release(t.Context()); err != nil {
		t.Fatalf("preflight invalidated the point: %v", err)
	}
	var empty *transaction.Savepoint
	for _, invalid := range []*transaction.Savepoint{empty, {}} {
		if !errors.Is(invalid.Rollback(t.Context()), goeferr.ErrInvalidState) ||
			!errors.Is(invalid.Release(t.Context()), goeferr.ErrInvalidState) {
			t.Fatal("invalid savepoint must fail safely")
		}
	}
}

func TestNativeSavepointFailureRetiresUncertainStack(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"native error", privateError},
		{"native cancellation", context.DeadlineExceeded},
		{"bad connection", driver.ErrBadConn},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, db := openSavepointProbe(t)
			tx := begin(t, t.Context(), db)
			outer := newSavepoint(t, tx, transaction.DialectPostgreSQL)
			p.fail = tc.err
			err := outer.Rollback(t.Context())
			if !errors.Is(err, goeferr.ErrSavepointFailed) || errors.Is(err, privateError) ||
				errors.Is(err, context.DeadlineExceeded) != (tc.err == context.DeadlineExceeded) {
				t.Fatalf("unsafe savepoint failure = %v", err)
			}
			if err := outer.Release(t.Context()); !errors.Is(err, goeferr.ErrInvalidState) {
				t.Fatalf("release after uncertain native failure = %v", err)
			}
			if _, err := tx.Savepoint(t.Context(), transaction.DialectPostgreSQL); !errors.Is(err, goeferr.ErrInvalidState) {
				t.Fatalf("creation after uncertain native failure = %v", err)
			}
			if len(p.commands) != 2 {
				t.Fatalf("savepoint SQL reached the uncertain transaction again: %q", p.commands)
			}
			if tc.err == driver.ErrBadConn {
				checkState(t, tx, db, transaction.StateFailed, 0)
				if p.close.Load() != 1 {
					t.Fatal("failed physical connection was not discarded")
				}
			} else {
				checkState(t, tx, db, transaction.StateActive, 1)
				if err := tx.Rollback(); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestSavepointObservesOwnedTransactionBusyAndLifetime(t *testing.T) {
	p, db := openSavepointProbe(t)
	p.execStarted = make(chan struct{})
	ctx, stop := context.WithCancel(t.Context())
	t.Cleanup(stop)
	tx := begin(t, ctx, db)
	done := make(chan error, 1)
	go func() {
		_, err := tx.Savepoint(t.Context(), transaction.DialectPostgreSQL)
		done <- err
	}()
	select {
	case <-p.execStarted:
	case <-time.After(2 * time.Second):
		stop()
		t.Fatal("savepoint did not reach the driver")
	}
	if _, err := tx.Savepoint(t.Context(), transaction.DialectPostgreSQL); !errors.Is(err, goeferr.ErrContextBusy) {
		stop()
		t.Fatalf("concurrent savepoint = %v", err)
	}
	stop()
	select {
	case err := <-done:
		if !errors.Is(err, goeferr.ErrSavepointFailed) || !errors.Is(err, context.Canceled) {
			t.Fatalf("savepoint cancellation = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("savepoint did not observe lifetime cancellation")
	}
	if err := tx.Close(); err != nil {
		t.Fatal(err)
	}
	checkState(t, tx, db, transaction.StateRolledBack, 0)
}

func TestSQLServerRearmFailureInvalidatesManagedStack(t *testing.T) {
	for _, tc := range []struct {
		name         string
		cancelRearm  bool
		wantCommands int
	}{
		{"native rearm failure", false, 4},
		{"cancellation before rearm dispatch", true, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, db := openSavepointProbe(t)
			tx := begin(t, t.Context(), db)
			outer := newSavepoint(t, tx, transaction.DialectSQLServer)
			target := newSavepoint(t, tx, transaction.DialectSQLServer)
			targetName := strings.TrimPrefix(p.commands[1], "SAVE TRANSACTION ")
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			p.onCommand = func(query string) {
				if tc.cancelRearm && query == "ROLLBACK TRANSACTION "+targetName {
					cancel()
				}
				if !tc.cancelRearm && query == "SAVE TRANSACTION "+targetName {
					p.fail = privateError
				}
			}
			err := target.Rollback(ctx)
			if !errors.Is(err, goeferr.ErrSavepointFailed) || errors.Is(err, privateError) ||
				errors.Is(err, context.Canceled) != tc.cancelRearm {
				t.Fatalf("rearm failure = %v", err)
			}
			if err := outer.Rollback(t.Context()); !errors.Is(err, goeferr.ErrInvalidState) {
				t.Fatalf("rollback after failed rearm = %v", err)
			}
			if err := target.Release(t.Context()); !errors.Is(err, goeferr.ErrInvalidState) {
				t.Fatalf("release after failed rearm = %v", err)
			}
			if _, err := tx.Savepoint(t.Context(), transaction.DialectSQLServer); !errors.Is(err, goeferr.ErrInvalidState) {
				t.Fatalf("new point after failed rearm = %v", err)
			}
			if len(p.commands) != tc.wantCommands {
				t.Fatalf("rearm command count = %d, want %d", len(p.commands), tc.wantCommands)
			}
			checkState(t, tx, db, transaction.StateActive, 1)
			if err := tx.Rollback(); err != nil {
				t.Fatal(err)
			}
			checkState(t, tx, db, transaction.StateRolledBack, 0)
		})
	}
}
