package transaction_test

import (
	"context"
	"errors"
	"testing"

	"github.com/igorpooh1978/GoEF/goeferr"
	"github.com/igorpooh1978/GoEF/transaction"
)

func TestExternalSavepointPreflightPreservesPoint(t *testing.T) {
	probe, db := openSavepointProbe(t)
	native, err := db.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = native.Rollback() })
	ext := external(t, native)
	if _, err := ext.Savepoint(nil, transaction.DialectSQLite); !errors.Is(err, goeferr.ErrInvalidState) {
		t.Fatalf("nil context = %v", err)
	}
	point, err := ext.Savepoint(t.Context(), transaction.DialectSQLite)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := point.Rollback(ctx); !errors.Is(err, context.Canceled) || !errors.Is(err, goeferr.ErrCanceled) {
		t.Fatalf("canceled rollback = %v", err)
	}
	if len(probe.commands) != 1 {
		t.Fatal("a canceled savepoint command reached the driver")
	}
	if err := point.Rollback(t.Context()); err != nil {
		t.Fatalf("rollback to existing point = %v", err)
	}
	if err := point.Release(t.Context()); err != nil {
		t.Fatalf("release existing point = %v", err)
	}
}

func TestExternalSavepointObservesNativeCompletion(t *testing.T) {
	native := nativeTx(t, new(probe))
	ext := external(t, native)
	point, err := ext.Savepoint(t.Context(), transaction.DialectPostgreSQL)
	if err != nil {
		t.Fatal(err)
	}
	if err := native.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := point.Rollback(t.Context()); !errors.Is(err, goeferr.ErrInvalidState) {
		t.Fatalf("rollback after owner completion = %v", err)
	}
	if _, err := ext.Enlist(); !errors.Is(err, goeferr.ErrInvalidState) {
		t.Fatalf("enlist after observed completion = %v", err)
	}
}

func TestExternalSavepointFailureLeavesRecoveryToOwner(t *testing.T) {
	probe, db := openSavepointProbe(t)
	native, err := db.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = native.Rollback() })
	ext := external(t, native)
	point, err := ext.Savepoint(t.Context(), transaction.DialectPostgreSQL)
	if err != nil {
		t.Fatal(err)
	}
	probe.fail = privateError
	if err := point.Rollback(t.Context()); !errors.Is(err, goeferr.ErrSavepointFailed) || errors.Is(err, privateError) {
		t.Fatalf("unsafe savepoint failure = %v", err)
	}
	if err := point.Release(t.Context()); !errors.Is(err, goeferr.ErrInvalidState) {
		t.Fatalf("release after uncertain savepoint = %v", err)
	}
	if db.Stats().InUse != 1 || probe.rollback.Load() != 0 {
		t.Fatal("the coordinator finished the caller's transaction")
	}
	if err := native.Rollback(); err != nil {
		t.Fatalf("owner rollback = %v", err)
	}
}
