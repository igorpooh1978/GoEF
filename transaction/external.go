package transaction

import (
	"context"
	"database/sql"
	"errors"
	"sync"

	"github.com/igorpooh1978/GoEF/goeferr"
)

// External coordinates participants on a caller-owned SQL transaction.
// Copies share the same coordinator. The caller owns completion of the
// native transaction and any connection lease it acquired.
type External struct {
	shared *externalTx
}

type externalTx struct {
	mu         sync.Mutex
	native     NativeCommands
	savepoints savepointStack
}

// NewExternal coordinates participation in a native transaction opened by
// the caller with database/sql. Use one coordinator for that transaction and
// serialize direct native use with coordinator operations. The coordinator
// first observes native completion when a command reports sql.ErrTxDone.
func NewExternal(native *sql.Tx) (*External, error) {
	if native == nil {
		return nil, goeferr.NewInvalidState()
	}
	return &External{shared: &externalTx{native: sqlCommands{native}}}, nil
}

// Enlist attaches an independent participant to the shared transaction.
// The participant's commands use the coordinator's native transaction.
func (e *External) Enlist() (*Participant, error) {
	var participant *Participant
	err := e.withNative(func(NativeCommands) error {
		participant = &Participant{enlistment: &enlistment{external: *e}}
		return nil
	})
	return participant, err
}

// Close waits for any active coordinator operation, then detaches all
// participants. Repeated calls are safe. The native transaction remains
// available to its caller for further use or completion.
func (e *External) Close() error {
	if e == nil || e.shared == nil {
		return nil
	}
	shared := e.shared
	shared.mu.Lock()
	defer shared.mu.Unlock()
	shared.detach()
	return nil
}

// withNative runs action under the coordinator's shared gate. Actions return
// sql.ErrTxDone when they observe native completion and redact other errors.
func (e *External) withNative(action func(NativeCommands) error) error {
	if e == nil || e.shared == nil {
		return goeferr.NewInvalidState()
	}
	shared := e.shared
	if !shared.mu.TryLock() {
		return goeferr.NewContextBusy()
	}
	defer shared.mu.Unlock()
	if shared.native == nil {
		return goeferr.NewInvalidState()
	}
	err := action(shared.native)
	if errors.Is(err, sql.ErrTxDone) {
		shared.detach()
		return goeferr.NewInvalidState()
	}
	return err
}

// detach must be called with mu held.
func (s *externalTx) detach() {
	s.native = nil
	// The native owner may still commit. Normal detach retires manual points
	// without revoking the receipts for successful batches.
	s.savepoints = savepointStack{uncertain: true}
}

// Participant runs commands in a caller-owned transaction. Copies share
// the same enrollment, and every participant shares the coordinator's gate.
type Participant struct {
	enlistment *enlistment
}

type enlistment struct {
	external External
	detached bool // guarded by the external transaction gate
}

// Exec runs a transaction-preserving command on the caller's transaction.
// It forwards the supplied context to the native command. Conflicting
// operations return ErrContextBusy. Command errors leave recovery and
// completion to the owner.
func (p *Participant) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	var result sql.Result
	err := p.withCommands(ctx, func(native NativeCommands) error {
		var err error
		result, err = native.ExecContext(ctx, query, args...)
		return err
	})
	if err != nil {
		return nil, err
	}
	return safeResult{underlying: result}, nil
}

// Scan reads one row through the caller's transaction using the participant's
// operation gate. A missing row returns sql.ErrNoRows. The caller owns recovery
// or completion after cancellation or an execution error.
func (p *Participant) Scan(ctx context.Context, query string, args []any, dest ...any) error {
	return p.withCommands(ctx, func(native NativeCommands) error {
		return native.Scan(ctx, query, args, dest...)
	})
}

func (p *Participant) withCommands(ctx context.Context, run func(NativeCommands) error) error {
	if p == nil || p.enlistment == nil || ctx == nil {
		return goeferr.NewInvalidState()
	}
	enlisted := p.enlistment
	return enlisted.external.withNative(func(native NativeCommands) error {
		if enlisted.detached {
			return goeferr.NewInvalidState()
		}
		if err := ctx.Err(); err != nil {
			return operationError(err)
		}
		err := run(native)
		if errors.Is(err, sql.ErrNoRows) {
			return sql.ErrNoRows
		}
		if errors.Is(err, sql.ErrTxDone) {
			return sql.ErrTxDone
		}
		return operationError(err)
	})
}

// Close waits for any active coordinator operation, then detaches this
// participant. Repeated calls are safe, and other participants stay attached.
func (p *Participant) Close() error {
	if p == nil || p.enlistment == nil {
		return nil
	}
	enlisted := p.enlistment
	shared := enlisted.external.shared
	shared.mu.Lock()
	defer shared.mu.Unlock()
	enlisted.detached = true
	return nil
}
