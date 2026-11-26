package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/igorpooh1978/GoEF/goeferr"
	"github.com/igorpooh1978/GoEF/migration"
)

var (
	artifact     = migration.Entry{ID: "first", Digest: sha256.Sum256([]byte("reviewed first artifact"))}
	privateError = errors.New("private SQL literal and path")
)

type scriptedStep struct {
	statement string
	columns   int
	values    [][]driver.Value
	args      []driver.Value
	err       error
	rowsErr   error
	closeErr  error
	onCall    func()
}

func exec(statement string, args ...driver.Value) scriptedStep {
	return scriptedStep{statement: statement, args: args}
}

func query(statement string, columns int, values ...[]driver.Value) scriptedStep {
	return scriptedStep{statement: statement, columns: columns, values: values}
}

func scalar(statement string, value driver.Value) scriptedStep {
	return query(statement, 1, []driver.Value{value})
}

type script struct {
	t         *testing.T
	steps     []scriptedStep
	opened    bool
	discarded bool
}

func (s *script) Connect(context.Context) (driver.Conn, error) {
	if s.opened {
		return nil, errors.New("unexpected second connection")
	}
	s.opened = true
	return &scriptedConn{script: s}, nil
}

func (s *script) Driver() driver.Driver { return nil }

type scriptedConn struct{ script *script }

func (c *scriptedConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected Prepare")
}

func (c *scriptedConn) Begin() (driver.Tx, error) {
	return nil, errors.New("unexpected Begin")
}

func (c *scriptedConn) Close() error {
	c.script.discarded = true
	return nil
}

func (c *scriptedConn) ExecContext(ctx context.Context, statement string, args []driver.NamedValue) (driver.Result, error) {
	step := c.consume(ctx, statement, args, false)
	return driver.RowsAffected(1), step.err
}

func (c *scriptedConn) QueryContext(ctx context.Context, statement string, args []driver.NamedValue) (driver.Rows, error) {
	step := c.consume(ctx, statement, args, true)
	if step.err != nil {
		return nil, step.err
	}
	return &scriptedRows{step: step}, nil
}

func (c *scriptedConn) consume(ctx context.Context, statement string, args []driver.NamedValue, reading bool) scriptedStep {
	c.script.t.Helper()
	if ctx.Err() != nil {
		c.script.t.Errorf("dispatched %q with a canceled context", statement)
	}
	if len(c.script.steps) == 0 {
		c.script.t.Errorf("unexpected SQL %q", statement)
		return scriptedStep{err: privateError}
	}
	want := c.script.steps[0]
	c.script.steps = c.script.steps[1:]
	if statement != want.statement || reading != (want.columns != 0) {
		c.script.t.Errorf("dispatched %q (query %t); want %q", statement, reading, want.statement)
	}
	if len(args) != len(want.args) {
		c.script.t.Errorf("dispatched %d arguments; want %d", len(args), len(want.args))
	} else {
		for i, wantArg := range want.args {
			switch value := wantArg.(type) {
			case []byte:
				blob, ok := args[i].Value.([]byte)
				if !ok || !slices.Equal(blob, value) {
					c.script.t.Errorf("unexpected binary argument at position %d", i+1)
				}
			default:
				if args[i].Value != value {
					c.script.t.Errorf("unexpected argument at position %d", i+1)
				}
			}
		}
	}
	if want.onCall != nil {
		want.onCall()
	}
	return want
}

type scriptedRows struct{ step scriptedStep }

func (r *scriptedRows) Columns() []string {
	return make([]string, r.step.columns)
}

func (r *scriptedRows) Close() error { return r.step.closeErr }

func (r *scriptedRows) Next(values []driver.Value) error {
	if len(r.step.values) != 0 {
		copy(values, r.step.values[0])
		r.step.values = r.step.values[1:]
		return nil
	}
	if r.step.rowsErr != nil {
		return r.step.rowsErr
	}
	return io.EOF
}

func fixturePlan() Plan {
	check := func(statement string) func(context.Context, Queryer) (bool, error) {
		return func(ctx context.Context, query Queryer) (bool, error) {
			var matches bool
			err := query.QueryRowContext(ctx, statement).Scan(&matches)
			return matches, err
		}
	}
	return Plan{
		ApplicationID: 42,
		Lineage:       []migration.Entry{artifact},
		CheckSource:   check("CHECK SOURCE"),
		CheckTarget:   check("CHECK TARGET"),
		Run: func(ctx context.Context, session Session) error {
			_, err := session.ExecContext(ctx, "RUN REVIEWED STATEMENTS")
			return err
		},
	}
}

func preflight() []scriptedStep {
	return []scriptedStep{
		scalar("PRAGMA foreign_keys", int64(1)),
		exec("PRAGMA foreign_keys=OFF"),
		scalar("PRAGMA foreign_keys", int64(0)),
		exec("BEGIN IMMEDIATE"),
		scalar("PRAGMA main.application_id", int64(42)),
		exec(createHistory),
		query(selectHistory, 4),
	}
}

func operations() []scriptedStep {
	return []scriptedStep{
		scalar("CHECK SOURCE", true),
		exec("RUN REVIEWED STATEMENTS"),
		scalar("CHECK TARGET", true),
		query("PRAGMA main.foreign_key_check", 4),
		exec(insertHistory, int64(1), artifact.ID, artifact.Predecessor, artifact.Digest[:]),
	}
}

func finish(statement string) []scriptedStep {
	return []scriptedStep{
		exec(statement),
		exec("PRAGMA foreign_keys=ON"),
		scalar("PRAGMA foreign_keys", int64(1)),
	}
}

func runScenario(t *testing.T, ctx context.Context, plan Plan, steps []scriptedStep, wantApplied, wantDiscard bool, wants ...error) {
	t.Helper()
	fixture := &script{t: t, steps: steps}
	db := sql.OpenDB(fixture)
	t.Cleanup(func() { _ = db.Close() })

	applied, err := Apply(ctx, db, plan)
	if applied != wantApplied {
		t.Errorf("applied = %t; want %t", applied, wantApplied)
	}
	if len(wants) == 0 && err != nil {
		t.Errorf("Apply returned %v", err)
	}
	for _, want := range wants {
		if !errors.Is(err, want) {
			t.Errorf("Apply returned %v; want %v", err, want)
		}
	}
	if err != nil && strings.Contains(err.Error(), privateError.Error()) {
		t.Error("Apply exposed private error text")
	}
	if len(fixture.steps) != 0 {
		t.Errorf("%d expected statements did not run; first %q", len(fixture.steps), fixture.steps[0].statement)
	}
	if fixture.discarded != wantDiscard {
		t.Errorf("discarded = %t; want %t", fixture.discarded, wantDiscard)
	}
	if db.Stats().InUse != 0 {
		t.Error("migration connection is still leased")
	}
}

func TestApplyCommitsAndRestoresSettings(t *testing.T) {
	steps := append(preflight(), operations()...)
	steps = append(steps, finish("COMMIT")...)
	runScenario(t, t.Context(), fixturePlan(), steps, true, false)
}

func TestApplyValidatesAlreadyAppliedTarget(t *testing.T) {
	for _, tc := range []struct {
		name    string
		matches bool
		want    []error
	}{
		{name: "match", matches: true},
		{name: "drift", want: []error{goeferr.ErrMigrationRecoveryRequired}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			steps := preflight()
			steps[6] = query(selectHistory, 4, []driver.Value{int64(1), artifact.ID, artifact.Predecessor, artifact.Digest[:]})
			steps = append(steps, scalar("CHECK TARGET", tc.matches))
			if tc.matches {
				steps = append(steps, query("PRAGMA main.foreign_key_check", 4))
			}
			steps = append(steps, finish("ROLLBACK")...)
			runScenario(t, t.Context(), fixturePlan(), steps, false, false, tc.want...)
		})
	}
}

func TestApplyRejectsUnusableConnection(t *testing.T) {
	tests := []struct {
		name  string
		index int
		edit  func(*scriptedStep)
		want  error
	}{
		{"pragma read", 0, func(s *scriptedStep) { s.err = privateError }, goeferr.ErrUnknownDriver},
		{"pragma scalar", 0, func(s *scriptedStep) { s.values[0][0] = int64(2) }, goeferr.ErrUnknownDriver},
		{"pragma write", 1, func(s *scriptedStep) { s.err = privateError }, goeferr.ErrUnknownDriver},
		{"pragma verify read", 2, func(s *scriptedStep) { s.err = privateError }, goeferr.ErrUnknownDriver},
		{"pragma verify mismatch", 2, func(s *scriptedStep) { s.values[0][0] = int64(1) }, goeferr.ErrUnknownDriver},
		{"begin unknown", 3, func(s *scriptedStep) { s.err = privateError }, goeferr.ErrUnknownDriver},
		{"begin locked", 3, func(s *scriptedStep) { s.err = codedError(6 | 1<<8) }, goeferr.ErrMigrationLocked},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			steps := preflight()[:tc.index+1]
			tc.edit(&steps[tc.index])
			runScenario(t, t.Context(), fixturePlan(), steps, false, true, tc.want)
		})
	}
}

type codedError int

func (e codedError) Error() string { return privateError.Error() }
func (e codedError) Code() int     { return int(e) }

func TestApplyCancellationInAcquisition(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	steps := preflight()[:4]
	steps[3].onCall = cancel
	steps[3].err = codedError(5)
	runScenario(t, ctx, fixturePlan(), steps, false, true, goeferr.ErrCanceled, context.Canceled)

	expired, stop := context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
	t.Cleanup(stop)
	runScenario(t, expired, fixturePlan(), nil, false, false, goeferr.ErrDeadlineExceeded, context.DeadlineExceeded)
}

func TestApplyRollsBackOnVerificationAndExecutionFailures(t *testing.T) {
	badHistory := []driver.Value{int64(1), artifact.ID, artifact.Predecessor, artifact.Digest[:]}
	tests := []struct {
		name  string
		index int
		edit  func(*scriptedStep)
		want  error
	}{
		{"target read error", 4, func(s *scriptedStep) { s.err = privateError }, goeferr.ErrUnknownDriver},
		{"wrong target", 4, func(s *scriptedStep) { s.values[0][0] = int64(43) }, goeferr.ErrMigrationRecoveryRequired},
		{"create history", 5, func(s *scriptedStep) { s.err = privateError }, goeferr.ErrUnknownDriver},
		{"history query", 6, func(s *scriptedStep) { s.err = privateError }, goeferr.ErrUnknownDriver},
		{"history iteration", 6, func(s *scriptedStep) { s.rowsErr = privateError }, goeferr.ErrUnknownDriver},
		{"history row conversion", 6, func(s *scriptedStep) { s.values = [][]driver.Value{{"bad", "", "", nil}} }, goeferr.ErrUnknownDriver},
		{"history noncontiguous", 6, func(s *scriptedStep) { s.values = [][]driver.Value{{int64(2), "", "", artifact.Digest[:]}} }, goeferr.ErrMigrationConflict},
		{"history digest size", 6, func(s *scriptedStep) { s.values = [][]driver.Value{{int64(1), "", "", []byte{1}}} }, goeferr.ErrMigrationConflict},
		{"history extra entries", 6, func(s *scriptedStep) { s.values = [][]driver.Value{badHistory, badHistory}; s.closeErr = privateError }, goeferr.ErrMigrationConflict},
		{"history divergent entry", 6, func(s *scriptedStep) { s.values = [][]driver.Value{{int64(1), "different", "", artifact.Digest[:]}} }, goeferr.ErrMigrationConflict},
		{"source drift", 7, func(s *scriptedStep) { s.values[0][0] = false }, goeferr.ErrMigrationRecoveryRequired},
		{"source unavailable", 7, func(s *scriptedStep) { s.err = privateError }, goeferr.ErrMigrationRecoveryRequired},
		{"driver statement error", 8, func(s *scriptedStep) { s.err = privateError }, goeferr.ErrUnknownDriver},
		{"target postcondition", 9, func(s *scriptedStep) { s.values[0][0] = false }, goeferr.ErrMigrationRecoveryRequired},
		{"foreign key query", 10, func(s *scriptedStep) { s.err = privateError }, goeferr.ErrUnknownDriver},
		{"foreign key iteration", 10, func(s *scriptedStep) { s.rowsErr = privateError }, goeferr.ErrUnknownDriver},
		{"foreign key violation", 10, func(s *scriptedStep) {
			s.values = [][]driver.Value{{"child", int64(1), "parent", int64(0)}}
			s.closeErr = privateError
		}, goeferr.ErrMigrationRecoveryRequired},
		{"insert history", 11, func(s *scriptedStep) { s.err = privateError }, goeferr.ErrUnknownDriver},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			steps := append(preflight(), operations()...)[:tc.index+1]
			tc.edit(&steps[tc.index])
			steps = append(steps, finish("ROLLBACK")...)
			runScenario(t, t.Context(), fixturePlan(), steps, false, false, tc.want)
		})
	}
}

func TestApplyRejectsMissingPredecessor(t *testing.T) {
	plan := fixturePlan()
	plan.Lineage = append(plan.Lineage, migration.Entry{ID: "second", Predecessor: artifact.ID, Digest: artifact.Digest})
	steps := append(preflight(), finish("ROLLBACK")...)
	runScenario(t, t.Context(), plan, steps, false, false, goeferr.ErrMigrationConflict)

	plan.Lineage[1].Predecessor = "missing"
	runScenario(t, t.Context(), plan, nil, false, false, goeferr.ErrMigrationConflict)
}

func TestApplyQuarantinesUnconfirmedResults(t *testing.T) {
	t.Run("commit response lost", func(t *testing.T) {
		steps := append(preflight(), operations()...)
		commit := exec("COMMIT")
		commit.err = fmt.Errorf("%w: %w", privateError, context.DeadlineExceeded)
		steps = append(steps, commit)
		runScenario(t, t.Context(), fixturePlan(), steps, false, true, goeferr.ErrCommitOutcomeUnknown, context.DeadlineExceeded)
	})
	t.Run("acknowledged commit and restoration failure", func(t *testing.T) {
		steps := append(preflight(), operations()...)
		cleanup := finish("COMMIT")
		cleanup[1].err = privateError
		steps = append(steps, cleanup[:2]...)
		runScenario(t, t.Context(), fixturePlan(), steps, true, true, goeferr.ErrUnknownDriver)
	})
	t.Run("rollback response lost", func(t *testing.T) {
		steps := append(preflight(), operations()[:2]...)
		steps[len(steps)-1].err = privateError
		rollback := exec("ROLLBACK")
		rollback.err = privateError
		steps = append(steps, rollback)
		runScenario(t, t.Context(), fixturePlan(), steps, false, true, goeferr.ErrUnknownDriver, goeferr.ErrRollbackFailed)
	})
}

func TestApplyCancellationBeforeCommitRollsBack(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	steps := append(preflight(), operations()...)
	steps[len(steps)-1].onCall = cancel
	steps = append(steps, finish("ROLLBACK")...)
	runScenario(t, ctx, fixturePlan(), steps, false, false, goeferr.ErrCanceled, context.Canceled)
}

func TestApplyRestoresDisabledForeignKeys(t *testing.T) {
	steps := preflight()
	steps[0].values[0][0] = int64(0)
	steps = append(steps, operations()...)
	steps = append(steps, exec("COMMIT"), exec("PRAGMA foreign_keys=OFF"), scalar("PRAGMA foreign_keys", int64(0)))
	runScenario(t, t.Context(), fixturePlan(), steps, true, false)
}
