package transaction

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"sync/atomic"

	"github.com/igorpooh1978/GoEF/goeferr"
)

// SaveOptions describes the savepoint capability of the transaction's effective
// engine and configuration. Enable Savepoints only for transactional statements
// and a connection configuration that supports recovery to a savepoint.
type SaveOptions struct {
	Dialect    Dialect
	Savepoints bool
}

// SaveChanges runs the ordered batch in the owned transaction using an automatic
// savepoint. Keep change synchronous and use only its writer for batch work.
// On success the transaction stays open. Accept the saved values after Commit.
// On failure a confirmed rewind keeps earlier writes. If recovery cannot be
// confirmed, GoEF rolls the owned transaction back in full.
func (tx *Tx) SaveChanges(ctx context.Context, options SaveOptions, change func(*SaveWriter) error) (*SavedChanges, error) {
	var saved *SavedChanges
	err := tx.withCommand(ctx, func(command context.Context, owned *ownedTx) error {
		values, err := saveBatch(command, owned.native, &owned.savepoints, options, change, func(err error) error {
			return commandError(ctx, command, err)
		}, owned.rollback)
		if err == nil {
			saved = owned.savepoints.saved(values, func() bool { return owned.state.Load().(State) == StateCommitted })
		}
		return err
	})
	return saved, err
}

// SaveChanges runs the ordered batch in the caller's native transaction.
// Keep change synchronous and use only its writer. The caller owns completion
// and accepts saved values only after confirming the batch was committed. If
// recovery cannot be confirmed, GoEF detaches the coordinator and the caller
// must roll back the native transaction.
func (e *External) SaveChanges(ctx context.Context, options SaveOptions, change func(*SaveWriter) error) (*SavedChanges, error) {
	return e.saveChanges(ctx, options, change, nil)
}

// SaveChanges runs the batch through this participant's external coordinator.
// It uses the same savepoint and ownership rules as External.SaveChanges.
func (p *Participant) SaveChanges(ctx context.Context, options SaveOptions, change func(*SaveWriter) error) (*SavedChanges, error) {
	if p == nil || p.enlistment == nil {
		return nil, goeferr.NewInvalidState()
	}
	enlisted := p.enlistment
	return enlisted.external.saveChanges(ctx, options, change, enlisted)
}

func (e *External) saveChanges(ctx context.Context, options SaveOptions, change func(*SaveWriter) error, enlisted *enlistment) (*SavedChanges, error) {
	if ctx == nil {
		return nil, goeferr.NewInvalidState()
	}
	var saved *SavedChanges
	err := e.withNative(func(native NativeCommands) error {
		if enlisted != nil && enlisted.detached {
			return goeferr.NewInvalidState()
		}
		values, err := saveBatch(ctx, native, &e.shared.savepoints, options, change, func(err error) error { return err }, func(bool) error {
			e.shared.detach()
			return nil
		})
		if err == nil {
			saved = e.shared.savepoints.saved(values, nil)
		} else if e.shared.native == nil && errors.Is(err, goeferr.ErrSavepointFailed) {
			// The owner still holds the native handle and must verify full
			// rollback. A failed release followed by a confirmed rewind
			// keeps the coordinator attached and does not add this error.
			return errors.Join(err, goeferr.NewRollbackFailed())
		}
		return err
	})
	return saved, err
}

func (s *savepointStack) saved(values []func(), committed func() bool) *SavedChanges {
	valid := new(atomic.Bool)
	valid.Store(true)
	s.receipts = append(s.receipts, valid)
	return newSavedChanges(values, func() bool {
		return (committed == nil || committed()) && valid.Load()
	})
}

// XACT_ABORT ON can roll back the transaction or make it uncommittable after a
// statement error, so a savepoint would be insufficient for atomic recovery.
const sqlServerXactAbort = "SELECT CASE WHEN (16384 & @@OPTIONS) = 16384 THEN 1 ELSE 0 END"

// saveBatch runs entirely under the transaction operation gate. Its private
// point is scoped to the batch, separate from caller-visible points.
func saveBatch(ctx context.Context, native NativeCommands, stack *savepointStack, options SaveOptions, change func(*SaveWriter) error, normalize func(error) error, abandon func(bool) error) ([]func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, operationError(normalize(err))
	}
	if change == nil {
		return nil, goeferr.NewInvalidState()
	}
	if !options.Savepoints {
		return nil, goeferr.NewUnsupportedFeature()
	}
	if err := stack.validateDialect(options.Dialect); err != nil {
		return nil, err
	}

	var badConnection, nativeDone bool
	observe := func(err error) {
		badConnection = badConnection || errors.Is(err, driver.ErrBadConn)
		nativeDone = nativeDone || errors.Is(err, sql.ErrTxDone)
	}
	unsafe := func(err error) error {
		stack.invalidate()
		return errors.Join(err, abandon(badConnection))
	}
	redact := func(err error) error {
		observe(err)
		if errors.Is(err, sql.ErrTxDone) {
			return goeferr.NewInvalidState()
		}
		return operationError(normalize(err))
	}
	if options.Dialect == DialectSQLServer {
		var xactAbort int
		if err := native.Scan(ctx, sqlServerXactAbort, nil, &xactAbort); err != nil {
			detail := redact(err)
			if nativeDone {
				return nil, errors.Join(detail, abandon(false))
			}
			if badConnection {
				detail = unsafe(detail)
			}
			return nil, detail
		}
		if xactAbort != 0 {
			return nil, goeferr.NewUnsupportedFeature()
		}
	}

	execPoint := func(call context.Context, statement string) error {
		_, err := native.ExecContext(call, statement)
		observe(err)
		return err
	}
	name := newSavepointName()
	create, rewind, release := savepointSQL(options.Dialect)
	if err := ctx.Err(); err != nil {
		return nil, operationError(normalize(err))
	}
	if err := execPoint(ctx, create+name); err != nil {
		if nativeDone {
			// No batch work ran. Detach without revoking receipts for
			// earlier saves the native owner may have committed.
			return nil, errors.Join(goeferr.NewInvalidState(), abandon(false))
		}
		return nil, unsafe(savepointError(normalize(err)))
	}
	stack.dialect = options.Dialect

	recoverPoint := func() error {
		if badConnection || nativeDone {
			return unsafe(goeferr.NewSavepointFailed())
		}
		cleanup, cancel := cleanupContext(ctx)
		defer cancel()
		if err := execPoint(cleanup, rewind+name); err != nil {
			return unsafe(savepointError(err))
		}
		if release != "" {
			if err := execPoint(cleanup, release+name); err != nil {
				return unsafe(savepointError(err))
			}
		}
		return nil
	}

	writer := newSaveWriter(ctx, native, redact)
	finished := false
	defer func() {
		if !finished {
			writer.finish()
			_ = recoverPoint()
		}
	}()
	// Preserve the private handle even when callback code overwrites its copy.
	callbackWriter := *writer
	callbackErr := change(&callbackWriter)
	values, statementErr := writer.finish()
	finished = true
	if statementErr != nil {
		return nil, errors.Join(statementErr, recoverPoint())
	}
	if callbackErr != nil {
		return nil, errors.Join(saveCallbackError(normalize(callbackErr)), recoverPoint())
	}
	if err := ctx.Err(); err != nil {
		return nil, errors.Join(operationError(normalize(err)), recoverPoint())
	}
	if release != "" {
		if err := execPoint(ctx, release+name); err != nil {
			return nil, errors.Join(savepointError(normalize(err)), recoverPoint())
		}
	}
	return values, nil
}

func saveCallbackError(err error) error {
	if cause := contextCause(err); cause != nil {
		return operationError(cause)
	}
	return goeferr.NewSaveChangesFailed()
}
