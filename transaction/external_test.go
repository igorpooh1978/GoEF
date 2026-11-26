package transaction_test

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/igorpooh1978/GoEF/goeferr"
	"github.com/igorpooh1978/GoEF/transaction"
)

func TestExternalEnlistmentLeavesCompletionAndConnectionWithOwner(t *testing.T) {
	t.Parallel()
	for _, completion := range []struct {
		name               string
		run                func(*sql.Tx) error
		commits, rollbacks int32
	}{
		{"commit", (*sql.Tx).Commit, 1, 0},
		{"rollback", (*sql.Tx).Rollback, 0, 1},
	} {
		t.Run(completion.name, func(t *testing.T) {
			t.Parallel()
			p := new(probe)
			db := openProbe(t, p)
			conn, err := db.Conn(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = conn.Close() }()
			native, err := conn.BeginTx(t.Context(), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = native.Rollback() }()
			ext := external(t, native)
			first := enlist(t, ext)
			extCopy := *ext
			second := enlist(t, &extCopy)
			observer := enlist(t, ext)

			participantCopy := *first
			for _, participant := range []*transaction.Participant{first, second} {
				result, err := participant.Exec(t.Context(), "synthetic write", "value")
				if err != nil {
					t.Fatal(err)
				}
				if count, err := result.RowsAffected(); count != 1 || err != nil {
					t.Fatalf("affected rows = %d, error = %v", count, err)
				}
				if err := participant.Close(); err != nil {
					t.Fatal(err)
				}
				if err := participant.Close(); err != nil {
					t.Fatal(err)
				}
			}
			if result, err := participantCopy.Exec(t.Context(), "synthetic write"); result != nil || !errors.Is(err, goeferr.ErrInvalidState) {
				t.Fatalf("detached copy returned = %v, %v", result, err)
			}
			if p.exec.Load() != 2 || p.open.Load() != 1 || p.close.Load() != 0 ||
				p.commit.Load() != 0 || p.rollback.Load() != 0 || db.Stats().InUse != 1 {
				t.Fatal("participant use changed ownership or escaped the original connection")
			}
			if _, err := native.ExecContext(t.Context(), "owner write"); err != nil {
				t.Fatalf("owner transaction was unavailable after detach: %v", err)
			}
			if err := completion.run(native); err != nil {
				t.Fatalf("owner completion: %v", err)
			}
			if result, err := observer.Exec(t.Context(), "synthetic write"); result != nil || !errors.Is(err, goeferr.ErrInvalidState) || errors.Is(err, sql.ErrTxDone) {
				t.Fatalf("participant use after owner completion = %v, %v", result, err)
			}
			if participant, err := extCopy.Enlist(); participant != nil || !errors.Is(err, goeferr.ErrInvalidState) {
				t.Fatalf("enlist after observed completion = %v, %v", participant, err)
			}
			if p.exec.Load() != 3 || p.open.Load() != 1 || p.close.Load() != 0 ||
				p.commit.Load() != completion.commits || p.rollback.Load() != completion.rollbacks || db.Stats().InUse != 1 {
				t.Fatal("only the owner should complete the transaction while retaining the connection")
			}
			if err := conn.Close(); err != nil || db.Stats().InUse != 0 {
				t.Fatalf("owner release error = %v, connections in use = %d", err, db.Stats().InUse)
			}
		})
	}
}

func TestExternalCoordinatorDetachLeavesNativeTransactionOpen(t *testing.T) {
	t.Parallel()
	p := new(probe)
	native := nativeTx(t, p)
	ext := external(t, native)
	copyHandle := *ext
	participants := []*transaction.Participant{enlist(t, ext), enlist(t, &copyHandle)}

	for _, coordinator := range []*transaction.External{&copyHandle, ext, nil, {}} {
		if err := coordinator.Close(); err != nil {
			t.Fatalf("coordinator close: %v", err)
		}
	}
	if participant, err := ext.Enlist(); participant != nil || !errors.Is(err, goeferr.ErrInvalidState) {
		t.Fatalf("enlist after coordinator detach = %v, %v", participant, err)
	}
	for _, participant := range participants {
		if result, err := participant.Exec(t.Context(), "synthetic write"); result != nil || !errors.Is(err, goeferr.ErrInvalidState) {
			t.Fatalf("participant use after coordinator detach = %v, %v", result, err)
		}
	}
	if p.exec.Load() != 0 || p.rollback.Load() != 0 || p.commit.Load() != 0 || p.close.Load() != 0 {
		t.Fatal("coordinator detach affected native ownership or dispatched a command")
	}
	if _, err := native.ExecContext(t.Context(), "owner write"); err != nil {
		t.Fatalf("coordinator detach affected the owner's transaction: %v", err)
	}
	if err := native.Commit(); err != nil || p.commit.Load() != 1 {
		t.Fatalf("owner commit = %v, commit calls = %d", err, p.commit.Load())
	}
}

func TestExternalEnlistmentRejectsInvalidInputs(t *testing.T) {
	t.Parallel()
	if ext, err := transaction.NewExternal(nil); ext != nil || !errors.Is(err, goeferr.ErrInvalidState) {
		t.Fatalf("nil native transaction = %v, %v", ext, err)
	}
	for _, ext := range []*transaction.External{nil, {}} {
		if participant, err := ext.Enlist(); participant != nil || !errors.Is(err, goeferr.ErrInvalidState) {
			t.Fatalf("uninitialized external coordinator = %v, %v", participant, err)
		}
	}
	for _, participant := range []*transaction.Participant{nil, {}} {
		if result, err := participant.Exec(t.Context(), "synthetic write"); result != nil || !errors.Is(err, goeferr.ErrInvalidState) {
			t.Fatalf("uninitialized participant = %v, %v", result, err)
		}
		if err := participant.Close(); err != nil {
			t.Fatalf("close uninitialized participant: %v", err)
		}
	}
	p := new(probe)
	native := nativeTx(t, p)
	participant := enlist(t, external(t, native))
	if result, err := participant.Exec(nil, "synthetic write"); result != nil || !errors.Is(err, goeferr.ErrInvalidState) {
		t.Fatalf("nil context = %v, %v", result, err)
	}
	if p.exec.Load() != 0 || p.commit.Load() != 0 || p.rollback.Load() != 0 {
		t.Fatal("invalid input reached the driver")
	}
}

func TestExternalEnlistmentSharesBusyGateAndLeavesCancellationToOwner(t *testing.T) {
	t.Parallel()
	p := &probe{execStarted: make(chan struct{})}
	native := nativeTx(t, p)
	ext := external(t, native)
	first, second := enlist(t, ext), enlist(t, ext)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := first.Exec(ctx, "synthetic waiting write")
		done <- err
	}()
	select {
	case <-p.execStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("command did not reach driver")
	}
	if result, err := second.Exec(t.Context(), "synthetic write"); result != nil || !errors.Is(err, goeferr.ErrContextBusy) {
		t.Fatalf("busy participant = %v, %v", result, err)
	}
	if participant, err := ext.Enlist(); participant != nil || !errors.Is(err, goeferr.ErrContextBusy) {
		t.Fatalf("enlist during another participant's operation = %v, %v", participant, err)
	}

	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) || !errors.Is(err, goeferr.ErrCanceled) {
			t.Fatalf("in-flight cancellation = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("command did not observe cancellation")
	}

	if p.exec.Load() != 1 || p.commit.Load() != 0 || p.rollback.Load() != 0 || p.close.Load() != 0 {
		t.Fatal("busy attempt or command cancellation affected owner state")
	}
	if err := native.Rollback(); err != nil || p.rollback.Load() != 1 {
		t.Fatalf("owner rollback = %v, rollback calls = %d", err, p.rollback.Load())
	}
}

func TestExternalEnlistmentRedactsDriverAndResultErrors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		probe     *probe
		execFails bool
	}{
		{"execution error", &probe{execErr: privateError}, true},
		{"result error", &probe{result: privateResult{}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			native := nativeTx(t, tc.probe)
			participant := enlist(t, external(t, native))
			result, err := participant.Exec(t.Context(), "synthetic write", "value")
			if tc.execFails {
				if result != nil {
					t.Fatal("failed execution returned a result")
				}
				checkExternalRedaction(t, err)
			} else {
				if err != nil || result == nil {
					t.Fatalf("successful execution = %v, %v", result, err)
				}
				_, idErr := result.LastInsertId()
				_, rowsErr := result.RowsAffected()
				checkExternalRedaction(t, idErr)
				checkExternalRedaction(t, rowsErr)
			}
			if tc.probe.rollback.Load() != 0 || tc.probe.commit.Load() != 0 || tc.probe.close.Load() != 0 {
				t.Fatal("participant completed or closed owner resources after a driver error")
			}
			if err := native.Rollback(); err != nil || tc.probe.rollback.Load() != 1 {
				t.Fatalf("owner rollback = %v, rollback calls = %d", err, tc.probe.rollback.Load())
			}
		})
	}
}

func nativeTx(t *testing.T, p *probe) *sql.Tx {
	t.Helper()
	db := openProbe(t, p)
	tx, err := db.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	return tx
}

func external(t *testing.T, tx *sql.Tx) *transaction.External {
	t.Helper()
	ext, err := transaction.NewExternal(tx)
	if err != nil {
		t.Fatal(err)
	}
	return ext
}

func enlist(t *testing.T, ext *transaction.External) *transaction.Participant {
	t.Helper()
	participant, err := ext.Enlist()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = participant.Close() })
	return participant
}

func checkExternalRedaction(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, goeferr.ErrUnknownDriver) || errors.Is(err, privateError) || strings.Contains(err.Error(), privateError.Error()) {
		t.Fatalf("driver error was not safely classified: %v", err)
	}
}
