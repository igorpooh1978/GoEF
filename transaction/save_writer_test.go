package transaction

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/igorpooh1978/GoEF/goeferr"
)

var privateSaveDiagnostic = errors.New("private test diagnostic")

type saveWriterProbe struct {
	calls                     []writerCall
	result                    driver.Result
	rows                      *saveWriterRows
	execErr, queryErr         error
	execStarted, execContinue chan struct{}
}

type writerCall struct {
	query string
	args  []driver.NamedValue
}

func (*saveWriterProbe) Driver() driver.Driver                          { return nil }
func (p *saveWriterProbe) Connect(context.Context) (driver.Conn, error) { return p, nil }
func (*saveWriterProbe) Prepare(string) (driver.Stmt, error)            { return nil, privateSaveDiagnostic }
func (*saveWriterProbe) Close() error                                   { return nil }
func (p *saveWriterProbe) Begin() (driver.Tx, error)                    { return p, nil }
func (*saveWriterProbe) Commit() error                                  { return nil }
func (*saveWriterProbe) Rollback() error                                { return nil }

func (p *saveWriterProbe) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	p.calls = append(p.calls, writerCall{query, args})
	if p.execStarted != nil {
		close(p.execStarted)
		<-p.execContinue
	}
	if p.execErr != nil {
		return nil, p.execErr
	}
	if p.result != nil {
		return p.result, nil
	}
	return driver.RowsAffected(1), nil
}

func (p *saveWriterProbe) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	p.calls = append(p.calls, writerCall{query, args})
	if p.queryErr != nil {
		return nil, p.queryErr
	}
	return p.rows, nil
}

type saveWriterRows struct {
	value        int64
	read, closed bool
}

func (*saveWriterRows) Columns() []string { return []string{"generated"} }
func (r *saveWriterRows) Close() error    { r.closed = true; return nil }
func (r *saveWriterRows) Next(dest []driver.Value) error {
	if r.read {
		return io.EOF
	}
	dest[0] = r.value
	r.read = true
	return nil
}

type saveWriterPrivateResult struct{}

func (saveWriterPrivateResult) LastInsertId() (int64, error) { return 0, privateSaveDiagnostic }
func (saveWriterPrivateResult) RowsAffected() (int64, error) { return 0, privateSaveDiagnostic }

func probeSaveWriter(t *testing.T, ctx context.Context, p *saveWriterProbe) *SaveWriter {
	t.Helper()
	db := sql.OpenDB(p)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	native, err := db.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = native.Rollback() })
	return newSaveWriter(ctx, sqlCommands{native}, operationError)
}

func TestSaveWriterStagesValuesAndClosesRows(t *testing.T) {
	rows := &saveWriterRows{value: 41}
	probe := &saveWriterProbe{rows: rows}
	writer := probeSaveWriter(t, t.Context(), probe)
	copied := *writer

	key := int64(-1)
	var generated int64
	if err := copied.Scan("insert parent returning key", []any{"parent"}, &generated); err != nil {
		t.Fatal(err)
	}
	if generated != 41 || !rows.closed {
		t.Fatalf("generated = %d, rows closed = %t", generated, rows.closed)
	}
	if err := StageValue(writer, &key, generated); err != nil {
		t.Fatal(err)
	}
	result, err := copied.Exec("insert child", generated)
	if err != nil {
		t.Fatal(err)
	}
	if n, err := result.RowsAffected(); n != 1 || err != nil {
		t.Fatalf("RowsAffected = %d, %v", n, err)
	}
	publish, err := writer.finish()
	if err != nil || key != -1 {
		t.Fatalf("finish = %v, premature key = %d", err, key)
	}
	if want := []writerCall{
		{"insert parent returning key", []driver.NamedValue{{Ordinal: 1, Value: "parent"}}},
		{"insert child", []driver.NamedValue{{Ordinal: 1, Value: int64(41)}}},
	}; !reflect.DeepEqual(probe.calls, want) {
		t.Fatalf("calls = %v, want %v", probe.calls, want)
	}
	_, lateExecErr := copied.Exec("late write")
	_, lateResultErr := result.RowsAffected()
	for _, err := range []error{lateExecErr, lateResultErr, copied.Scan("", nil), StageValue(&copied, &key, int64(0))} {
		if !errors.Is(err, goeferr.ErrInvalidState) {
			t.Fatalf("late use = %v", err)
		}
	}
	receipt := newSavedChanges(publish, nil)
	if err := receipt.Accept(); err != nil || key != 41 {
		t.Fatalf("Accept = %v, key = %d", err, key)
	}
}

func TestSaveWriterLatchesIgnoredFailures(t *testing.T) {
	for _, test := range []struct {
		name  string
		probe saveWriterProbe
		call  func(w *SaveWriter) error
	}{
		{"exec", saveWriterProbe{execErr: privateSaveDiagnostic}, func(w *SaveWriter) error {
			_, err := w.Exec("synthetic insert")
			return err
		}},
		{"scan", saveWriterProbe{queryErr: privateSaveDiagnostic}, func(w *SaveWriter) error {
			var key int64
			return w.Scan("synthetic insert returning key", nil, &key)
		}},
		{"insert-id", saveWriterProbe{result: saveWriterPrivateResult{}}, func(w *SaveWriter) error {
			result, err := w.Exec("synthetic insert")
			if err != nil {
				return err
			}
			_, err = result.LastInsertId()
			return err
		}},
		{"rows-affected", saveWriterProbe{result: saveWriterPrivateResult{}}, func(w *SaveWriter) error {
			result, err := w.Exec("synthetic update")
			if err != nil {
				return err
			}
			_, err = result.RowsAffected()
			return err
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			writer := probeSaveWriter(t, t.Context(), &test.probe)
			key := -1
			if err := StageValue(writer, &key, 41); err != nil {
				t.Fatal(err)
			}
			firstErr := test.call(writer)
			if !errors.Is(firstErr, goeferr.ErrUnknownDriver) || errors.Is(firstErr, privateSaveDiagnostic) ||
				strings.Contains(firstErr.Error(), privateSaveDiagnostic.Error()) {
				t.Fatalf("unsafe failure = %v", firstErr)
			}
			_, lateErr := writer.Exec("must not dispatch")
			publish, finishErr := writer.finish()
			if lateErr != firstErr || finishErr != firstErr || len(publish) != 0 ||
				key != -1 || len(test.probe.calls) != 1 {
				t.Fatalf("sticky error = %v, finish = %v, pending operations = %d, key = %d, calls = %d",
					lateErr, finishErr, len(publish), key, len(test.probe.calls))
			}
		})
	}
}

func TestSaveWriterRejectsInvalidAndCanceledUse(t *testing.T) {
	for _, writer := range []*SaveWriter{nil, {}} {
		_, err := writer.Exec("synthetic insert")
		if !errors.Is(err, goeferr.ErrInvalidState) {
			t.Fatalf("uninitialized Exec = %v", err)
		}
		if err := StageValue(writer, new(int), 1); !errors.Is(err, goeferr.ErrInvalidState) {
			t.Fatalf("uninitialized StageValue = %v", err)
		}
	}
	probe := new(saveWriterProbe)
	ctx, cancel := context.WithCancel(t.Context())
	writer := probeSaveWriter(t, ctx, probe)
	cancel()
	if _, err := writer.Exec("synthetic insert"); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Exec = %v", err)
	}
	if _, err := writer.finish(); !errors.Is(err, goeferr.ErrCanceled) || len(probe.calls) != 0 {
		t.Fatalf("canceled finish = %v, dispatched calls = %d", err, len(probe.calls))
	}

	writer = probeSaveWriter(t, t.Context(), new(saveWriterProbe))
	if err := StageValue[int](writer, nil, 1); !errors.Is(err, goeferr.ErrInvalidState) {
		t.Fatalf("nil target StageValue = %v", err)
	}
	if _, err := writer.finish(); !errors.Is(err, goeferr.ErrInvalidState) {
		t.Fatalf("nil target finish = %v", err)
	}
}

func TestSaveWriterBusyWhileCommandUsesNativeTransaction(t *testing.T) {
	probe := &saveWriterProbe{execStarted: make(chan struct{}), execContinue: make(chan struct{})}
	writer := probeSaveWriter(t, t.Context(), probe)
	cmdDone := make(chan error, 1)
	go func() {
		_, err := writer.Exec("synthetic insert")
		cmdDone <- err
	}()
	select {
	case <-probe.execStarted:
	case <-time.After(2 * time.Second):
		close(probe.execContinue)
		t.Fatal("command did not reach driver")
	}
	_, busyErr := writer.Exec("conflicting insert")
	close(probe.execContinue)
	if !errors.Is(busyErr, goeferr.ErrContextBusy) {
		t.Fatalf("concurrent Exec = %v", busyErr)
	}
	select {
	case err := <-cmdDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("command did not finish")
	}
	if _, err := writer.finish(); err != nil || len(probe.calls) != 1 {
		t.Fatalf("finish = %v, dispatched calls = %d", err, len(probe.calls))
	}
}

func TestSavedChangesAcceptsAfterConfirmationOnceAcrossCopies(t *testing.T) {
	for _, receipt := range []*SavedChanges{nil, {}} {
		if err := receipt.Accept(); !errors.Is(err, goeferr.ErrInvalidState) {
			t.Fatalf("uninitialized Accept = %v", err)
		}
	}

	writer := probeSaveWriter(t, t.Context(), new(saveWriterProbe))
	version := "before"
	for _, value := range []string{"first", "last"} {
		if err := StageValue(writer, &version, value); err != nil {
			t.Fatal(err)
		}
	}
	publish, err := writer.finish()
	if err != nil {
		t.Fatal(err)
	}
	committed := false
	receipt := newSavedChanges(publish, func() bool { return committed })
	copied := *receipt
	if err := copied.Accept(); !errors.Is(err, goeferr.ErrInvalidState) || version != "before" {
		t.Fatalf("precommit Accept = %v, version = %q", err, version)
	}
	committed = true
	if err := receipt.Accept(); err != nil || version != "last" {
		t.Fatalf("postcommit Accept = %v, version = %q", err, version)
	}
	version = "caller edit"
	if err := copied.Accept(); !errors.Is(err, goeferr.ErrInvalidState) || version != "caller edit" {
		t.Fatalf("repeated Accept = %v, version = %q", err, version)
	}
}
