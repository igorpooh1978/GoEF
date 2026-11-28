package postgres_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/igorpooh1978/GoEF/goeferr"
	postgres "github.com/igorpooh1978/GoEF/providers/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestNativeTransactionRejectsInvalidInput(t *testing.T) {
	pool := new(pgxpool.Pool) // Invalid requests must stop before calling Acquire.
	if tx, err := postgres.BeginTx(nil, pool, sql.TxOptions{}); tx != nil || !errors.Is(err, goeferr.ErrInvalidState) {
		t.Fatalf("nil context = %v, %v", tx, err)
	}
	if tx, err := postgres.BeginTx(t.Context(), nil, sql.TxOptions{}); tx != nil || !errors.Is(err, goeferr.ErrInvalidState) {
		t.Fatalf("nil pool = %v, %v", tx, err)
	}
	if tx, err := postgres.BeginTx(t.Context(), pool, sql.TxOptions{Isolation: sql.LevelSnapshot}); tx != nil || !errors.Is(err, goeferr.ErrUnsupportedFeature) {
		t.Fatalf("unsupported options = %v, %v", tx, err)
	}
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	if tx, err := postgres.BeginTx(canceled, pool, sql.TxOptions{}); tx != nil ||
		!errors.Is(err, goeferr.ErrCanceled) || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled context = %v, %v", tx, err)
	}
	var typedNil *pgxpool.Tx
	for _, native := range []pgx.Tx{nil, typedNil} {
		if external, err := postgres.NewExternal(native); external != nil || !errors.Is(err, goeferr.ErrInvalidState) {
			t.Fatalf("nil native = %v, %v", external, err)
		}
	}
}
