package transaction

import (
	"context"
	"database/sql"
	"errors"

	"github.com/igorpooh1978/GoEF/goeferr"
)

// Savepoint creates a managed point in the caller's transaction. The caller
// supplies the dialect. All participants and savepoints share the coordinator's
// gate. Detaching a participant leaves coordinator savepoints available.
func (e *External) Savepoint(ctx context.Context, dialect Dialect) (*Savepoint, error) {
	return createSavepoint(ctx, e, dialect)
}

func (e *External) withSavepoints(ctx context.Context, run func(*savepointStack, func(string) error) error) error {
	if ctx == nil {
		return goeferr.NewInvalidState()
	}
	return e.withNative(func(native NativeCommands) error {
		if err := ctx.Err(); err != nil {
			return operationError(err)
		}
		return run(&e.shared.savepoints, func(statement string) error {
			if err := ctx.Err(); err != nil {
				return operationError(err)
			}
			_, err := native.ExecContext(ctx, statement)
			if errors.Is(err, sql.ErrTxDone) {
				return sql.ErrTxDone
			}
			return savepointError(err)
		})
	})
}
