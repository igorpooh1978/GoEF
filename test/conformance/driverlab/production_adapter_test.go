package driverlab

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/igorpooh1978/GoEF/goeferr"
	"github.com/igorpooh1978/GoEF/provider"
	"github.com/igorpooh1978/GoEF/providers/mysql"
	"github.com/igorpooh1978/GoEF/providers/postgres"
	"github.com/igorpooh1978/GoEF/providers/sqlite"
	"github.com/igorpooh1978/GoEF/providers/sqlserver"
	"github.com/igorpooh1978/GoEF/transaction"
)

func TestProductionAdapters(t *testing.T) {
	name := os.Getenv("GOEF_DRIVERLAB_PROVIDER")
	if os.Getenv("GOEF_PRODUCTION_ADAPTERS") != "1" {
		t.Skip("set GOEF_DRIVERLAB_PROVIDER using the isolated lab runner")
	}
	if providers[name] == nil {
		t.Fatal("unknown production adapter fixture provider")
	}
	tests := []struct {
		id   string
		test func(*scenario)
	}{
		{"PROVIDER-EXEC-001-S01", checkProductionRoundTrip},
		{"PROVIDER-EXEC-001-S02", checkProductionConstraintErrors},
		{"PROVIDER-EXEC-001-S03", checkProductionCancellation},
		{"PROVIDER-EXEC-001-S04", checkProductionTransaction},
	}
	for _, tc := range tests {
		var version string
		completed := false
		if t.Run(tc.id, func(t *testing.T) {
			s := &scenario{t: t, provider: name}
			s.setup()
			version = s.facts["engine_version"]
			if version == "" {
				version = s.facts["version"]
			}
			if version == "" {
				version = s.facts["server_version"]
			}
			s.readBackend = productionBackend(s)
			tc.test(s)
			completed = true
		}) && completed && version != "" {
			report, err := json.Marshal(map[string]string{
				"provider": name, "profile": os.Getenv("GOEF_DRIVERLAB_PROFILE"),
				"engine_version": version, "scenario": tc.id, "status": "PASS",
			})
			if err != nil {
				t.Fatal("could not encode production adapter evidence")
			}
			fmt.Printf("GOEF_PRODUCTION_ADAPTER_RESULT=%s\n", report)
		}
	}
}

func productionBackend(s *scenario) provider.Backend {
	s.t.Helper()
	var backend provider.Backend
	var err error
	switch s.provider {
	case "postgres":
		backend, err = postgres.New(s.db)
	case "sqlite":
		backend, err = sqlite.New(s.db)
	case "sqlserver":
		backend, err = sqlserver.New(s.db)
	case "mysql":
		backend, err = mysql.New(s.db)
	}
	if err != nil || backend == nil {
		s.t.Fatal("could not construct the production adapter")
	}
	if descriptor := backend.Descriptor(); !descriptor.Compatible() || descriptor.ID != s.provider {
		s.t.Fatal("production adapter reported incompatible metadata")
	}
	return backend
}

func productionLease(s *scenario) *spiScenarioReader {
	s.t.Helper()
	lease, err := s.readBackend.Acquire(s.ctx)
	if err != nil || lease == nil {
		s.t.Fatal("could not acquire a production adapter lease")
	}
	reader := &spiScenarioReader{lease: lease}
	s.t.Cleanup(func() {
		if err := reader.finish(provider.Discard); err != nil {
			s.t.Error("could not release production adapter resources")
		}
	})
	return reader
}

func checkProductionRoundTrip(s *scenario) {
	// Reserve the fixture writer and observer. The third connection belongs
	// to the adapter and must return to its caller-owned pool on release.
	s.db.SetMaxOpenConns(3)
	reader := productionLease(s)
	insert := provider.Command{
		SQL:   s.insertParentSQL(),
		Slots: []provider.Slot{{Kind: provider.KindInteger}, {Kind: provider.KindText}},
	}
	const value = provider.Text("O'Brien & co")
	if write, err := reader.lease.Exec(s.ctx, insert, provider.Bindings{provider.Integer(41), provider.Integer(99)}); err == nil || write.RowsAffectedKnown {
		s.t.Fatal("adapter accepted a value with the wrong binding kind")
	}
	var count int
	if err := s.observer.QueryRowContext(s.ctx, "SELECT COUNT(*) FROM goef_driver_parent").Scan(&count); err != nil || count != 0 {
		s.t.Fatal("rejected binding changed the isolated table")
	}
	if write, err := reader.lease.Exec(s.ctx, insert, provider.Bindings{provider.Integer(41), value}); err != nil || write != (provider.WriteResult{RowsAffected: 1, RowsAffectedKnown: true}) {
		s.t.Fatalf("adapter did not report the inserted row: %v", err)
	}

	command := provider.Command{
		SQL:     "SELECT id, code FROM goef_driver_parent WHERE id = " + s.bind(1),
		Slots:   []provider.Slot{{Kind: provider.KindInteger}},
		Columns: []provider.Column{{Kind: provider.KindInteger}, {Kind: provider.KindText}},
	}
	var err error
	reader.rows, err = reader.lease.Query(s.ctx, command, provider.Bindings{provider.Integer(41)})
	if err != nil || reader.rows == nil {
		s.t.Fatalf("adapter did not open the filtered query: %v", err)
	}
	row, err := reader.rows.Next()
	if err != nil || len(row) != 2 || row[0] != provider.Integer(41) || row[1] != value {
		s.t.Fatal("adapter did not return the typed row with the original value")
	}
	if row, err := reader.rows.Next(); row != nil || !errors.Is(err, io.EOF) {
		s.t.Fatal("filtered query did not reach clean exhaustion")
	}
	if err := reader.finish(provider.Reuse); err != nil {
		s.t.Fatal("could not close the query and release its connection")
	}
	s.checkSPIPoolInUse(2)
	var text string
	if err := s.observer.QueryRowContext(s.ctx, "SELECT code FROM goef_driver_parent WHERE id = 41").Scan(&text); err != nil || text != string(value) {
		s.t.Fatal("independent observer did not see the committed write")
	}
}

func checkProductionConstraintErrors(s *scenario) {
	s.db.SetMaxOpenConns(3)
	s.seedParent()
	reader := productionLease(s)
	const privateValue = "synthetic private label 9142"
	insert := provider.Command{
		SQL:   s.insertParentSQL(),
		Slots: []provider.Slot{{Kind: provider.KindInteger}, {Kind: provider.KindText}},
	}
	if _, err := reader.lease.Exec(s.ctx, insert, provider.Bindings{provider.Integer(2), provider.Text(privateValue)}); err != nil {
		s.t.Fatal("could not prepare the production constraint negative control")
	}
	_, err := reader.lease.Exec(s.ctx, insert, provider.Bindings{provider.Integer(3), provider.Text(privateValue)})
	checkProductionError(s, err, goeferr.ErrUniqueViolation, s.uniqueCode, privateValue, insert.SQL)

	child := provider.Command{
		SQL:   s.insertChildSQL(),
		Slots: []provider.Slot{{Kind: provider.KindInteger}, {Kind: provider.KindInteger}},
	}
	_, err = reader.lease.Exec(s.ctx, child, provider.Bindings{provider.Integer(1), provider.Integer(70027)})
	kind := goeferr.ErrForeignKeyViolation
	if s.provider == "sqlserver" {
		// SQL Server reports 547 for both FK and CHECK violations.
		kind = goeferr.ErrUnknownDriver
	}
	checkProductionError(s, err, kind, s.foreignKeyCode, "70027", child.SQL)
	if err := reader.finish(provider.Reuse); err != nil {
		s.t.Fatal("could not release the constraint fixture connection")
	}
	s.checkSPIPoolInUse(2)
	var parents, children int
	if err := s.observer.QueryRowContext(s.ctx, "SELECT COUNT(*) FROM goef_driver_parent").Scan(&parents); err != nil || parents != 2 {
		s.t.Fatal("constraint failure changed the expected parent rows")
	}
	if err := s.observer.QueryRowContext(s.ctx, "SELECT COUNT(*) FROM goef_driver_child").Scan(&children); err != nil || children != 0 {
		s.t.Fatal("constraint failure inserted an orphan")
	}
}

func checkProductionError(s *scenario, err, kind error, native, value, statement string) {
	s.t.Helper()
	var detail *goeferr.ErrorDetail
	native, _, _ = strings.Cut(native, "/")
	if !errors.Is(err, kind) || !errors.As(err, &detail) || detail.NativeCode() != native {
		s.t.Fatalf("unexpected adapter error classification: %v", err)
	}
	if strings.Contains(err.Error(), value) || strings.Contains(err.Error(), statement) || s.code(err) != "" {
		s.t.Fatal("production adapter exposed private driver error information")
	}
}

func checkProductionCancellation(s *scenario) {
	// Reuse the test helpers on an independent pool so the fixture's separate
	// writer/observer do not affect the one-connection acquisition assertions.
	s.db = providers[s.provider](s.t).db
	s.db.SetMaxOpenConns(1)
	s.readBackend = productionBackend(s)
	ctx, cancel := context.WithTimeout(s.t.Context(), 5*time.Second)
	defer cancel()
	s.canceledReader = s.openSPIReader(ctx)
	s.checkSPIInteger(s.canceledReader.rows, 1)
	cancel()
	s.checkSPICancellation()
	s.checkSPIRecovery()
	s.checkSPIAcquireCancellation()
}

func checkProductionTransaction(s *scenario) {
	s.db.SetMaxOpenConns(3)
	backend, ok := s.readBackend.(interface{ DB() *sql.DB })
	if !ok || backend.DB() != s.db {
		s.t.Fatal("adapter did not expose the configured caller-owned pool")
	}
	dialect := savepointDialects[s.provider]
	tx, err := transaction.BeginWithOptions(s.ctx, backend.DB(), dialect, sql.TxOptions{})
	if err != nil {
		s.t.Fatal("could not begin an owned transaction using the adapter pool")
	}
	defer func() {
		if err := tx.Close(); err != nil {
			s.t.Error("could not close the production adapter transaction")
		}
	}()
	var accepted int
	saved, err := tx.SaveChanges(s.ctx, transaction.SaveOptions{Dialect: dialect, Savepoints: true}, func(writer *transaction.SaveWriter) error {
		if err := requireSaveWrite(writer.Exec(s.insertParentSQL(), 41, "transactional")); err != nil {
			return err
		}
		return transaction.StageValue(writer, &accepted, 41)
	})
	if err != nil || saved == nil || accepted != 0 || !errors.Is(saved.Accept(), goeferr.ErrInvalidState) {
		s.t.Fatal("adapter transaction did not stage the save before commit")
	}
	if err := tx.Commit(); err != nil || accepted != 0 {
		s.t.Fatal("adapter transaction did not commit with an unapplied receipt")
	}
	if err := saved.Accept(); err != nil || accepted != 41 {
		s.t.Fatal("adapter transaction receipt did not publish the staged value")
	}
	s.checkSPIPoolInUse(2)

	reader := productionLease(s)
	command := provider.Command{
		SQL:     "SELECT id FROM goef_driver_parent WHERE id = " + s.bind(1),
		Slots:   []provider.Slot{{Kind: provider.KindInteger}},
		Columns: []provider.Column{{Kind: provider.KindInteger}},
	}
	reader.rows, err = reader.lease.Query(s.ctx, command, provider.Bindings{provider.Integer(41)})
	if err != nil || reader.rows == nil {
		s.t.Fatal("adapter did not open the committed row reader")
	}
	s.checkSPIInteger(reader.rows, 41)
	if row, err := reader.rows.Next(); row != nil || err != io.EOF {
		s.t.Fatal("independent adapter read did not reach clean exhaustion")
	}
	if err := reader.finish(provider.Reuse); err != nil {
		s.t.Fatal("could not release the committed row reader")
	}
	s.checkSPIPoolInUse(2)
}
