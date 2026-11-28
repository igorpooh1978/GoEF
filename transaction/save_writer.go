package transaction

import (
	"context"
	"database/sql"
	"sync"

	"github.com/igorpooh1978/GoEF/goeferr"
)

// SaveWriter executes ordered work on the transaction bound to SaveChanges.
// Use it synchronously during the save callback. A command, scan, or result
// failure prevents further work in the batch even if the callback ignores it.
// Copies share the same lifetime.
type SaveWriter struct {
	shared *saveWriterState
}

type saveWriterState struct {
	mu      sync.Mutex
	ctx     context.Context
	native  NativeCommands
	redact  func(error) error
	publish []func()
	err     error
}

func newSaveWriter(ctx context.Context, native NativeCommands, redact func(error) error) *SaveWriter {
	return &SaveWriter{shared: &saveWriterState{ctx: ctx, native: native, redact: redact}}
}

// Exec runs a parameterized command on the save's physical connection. Use
// transaction-preserving DML. Read result metadata before the callback returns.
func (w *SaveWriter) Exec(query string, args ...any) (sql.Result, error) {
	var result sql.Result
	err := w.use(func(state *saveWriterState) error {
		nativeResult, err := state.native.ExecContext(state.ctx, query, args...)
		if err != nil {
			return state.redact(err)
		}
		result = saveWriterResult{writer: SaveWriter{shared: state}, result: nativeResult}
		return nil
	})
	return result, err
}

// Scan runs a parameterized query that returns one row and copies it into dest.
// Use scratch destinations for generated values and StageValue for publishing
// them to the application after commit.
func (w *SaveWriter) Scan(query string, args []any, dest ...any) error {
	return w.use(func(state *saveWriterState) error {
		err := state.native.Scan(state.ctx, query, args, dest...)
		if err != nil {
			return state.redact(err)
		}
		return nil
	})
}

// StageValue queues an assignment for explicit acceptance after commit. The
// target stays unchanged until acceptance. Value is copied as in an ordinary
// Go assignment. The caller synchronizes access to its target and mutable data.
func StageValue[T any](w *SaveWriter, target *T, value T) error {
	return w.use(func(state *saveWriterState) error {
		if target == nil {
			return goeferr.NewInvalidState()
		}
		state.publish = append(state.publish, func() { *target = value })
		return nil
	})
}

func (w *SaveWriter) use(run func(*saveWriterState) error) error {
	if w == nil || w.shared == nil {
		return goeferr.NewInvalidState()
	}
	state := w.shared
	if !state.mu.TryLock() {
		return goeferr.NewContextBusy()
	}
	defer state.mu.Unlock()

	if state.native == nil {
		return goeferr.NewInvalidState()
	}
	if state.err != nil {
		return state.err
	}
	if err := state.ctx.Err(); err != nil {
		state.err = state.redact(err)
	} else {
		state.err = run(state)
	}
	return state.err
}

func (w *SaveWriter) finish() ([]func(), error) {
	state := w.shared
	state.mu.Lock()
	defer state.mu.Unlock()

	publish, err := state.publish, state.err
	state.ctx, state.native, state.redact = nil, nil, nil
	state.publish, state.err = nil, nil
	if err != nil {
		return nil, err
	}
	return publish, nil
}

type saveWriterResult struct {
	writer SaveWriter
	result sql.Result
}

func (r saveWriterResult) LastInsertId() (int64, error) {
	return r.read(sql.Result.LastInsertId)
}

func (r saveWriterResult) RowsAffected() (int64, error) {
	return r.read(sql.Result.RowsAffected)
}

func (r saveWriterResult) read(read func(sql.Result) (int64, error)) (int64, error) {
	var value int64
	err := r.writer.use(func(state *saveWriterState) error {
		var err error
		value, err = read(r.result)
		if err != nil {
			return state.redact(err)
		}
		return nil
	})
	return value, err
}

// SavedChanges holds assignments staged by a successful SaveChanges. They
// remain provisional until the caller explicitly accepts a confirmed commit
// that includes those writes. Discard the receipt after rollback, or keep it
// unapplied while the outcome is unknown. After [Tx.ReconcileCommit]
// confirms a commit, the same rules apply to an owned receipt. Copies share
// acceptance.
type SavedChanges struct {
	shared *savedChangesState
}

type savedChangesState struct {
	mu        sync.Mutex
	canAccept func() bool
	publish   []func()
	accepted  bool
}

func newSavedChanges(publish []func(), canAccept func() bool) *SavedChanges {
	return &SavedChanges{shared: &savedChangesState{canAccept: canAccept, publish: publish}}
}

// Accept applies the staged assignments once in their original order. An
// owned transaction must have committed. For a borrowed transaction, the
// caller confirms outside GoEF that these writes committed. Accept performs
// no transaction operation. Accept multiple receipts in save order.
func (s *SavedChanges) Accept() error {
	if s == nil || s.shared == nil {
		return goeferr.NewInvalidState()
	}
	state := s.shared
	state.mu.Lock()
	defer state.mu.Unlock()

	if state.accepted || (state.canAccept != nil && !state.canAccept()) {
		return goeferr.NewInvalidState()
	}
	for _, publish := range state.publish {
		publish()
	}
	state.publish, state.canAccept = nil, nil
	state.accepted = true
	return nil
}
