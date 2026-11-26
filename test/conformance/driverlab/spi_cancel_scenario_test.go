package driverlab

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/igorpooh1978/GoEF/provider"
)

var spiTwoRows = provider.Command{
	SQL:     "SELECT v FROM (SELECT 1 AS v UNION ALL SELECT 2) AS goef_read_values ORDER BY v",
	Columns: []provider.Column{{Kind: provider.KindInteger}},
}

type spiScenarioReader struct {
	lease provider.Lease
	rows  provider.Rows
}

func (s *scenario) setupSPICancel() {
	s.t.Helper()
	s.fixture = providers[s.provider](s.t)
	s.db.SetMaxOpenConns(1)
	s.db.SetMaxIdleConns(1)
	if s.provider != "sqlite" {
		ctx, cancel := context.WithTimeout(s.t.Context(), 5*time.Second)
		defer cancel()
		s.verifyMarker(ctx, s.db.QueryContext)
	}
	s.readBackend = &spiReadBackend{id: s.provider, db: s.db}
	if descriptor := s.readBackend.Descriptor(); !descriptor.Compatible() || descriptor.ID != s.provider {
		s.t.Fatal("reader backend is incompatible with the provider SPI")
	}
	s.checkSPIPoolInUse(0)
}

func (s *scenario) cancelSPIReader() {
	s.t.Helper()
	ctx, cancel := context.WithTimeout(s.t.Context(), 5*time.Second)
	defer cancel()

	s.canceledReader = s.openSPIReader(ctx)
	s.checkSPIInteger(s.canceledReader.rows, 1)
	s.checkSPIPoolInUse(1)
	cancel()
}

func (s *scenario) checkSPICancellation() {
	s.t.Helper()
	row, err := s.canceledReader.rows.Next()
	if row != nil || !errors.Is(err, context.Canceled) || errors.Is(err, io.EOF) {
		s.t.Fatal("canceled SPI reader did not report caller cancellation")
	}
	if err := s.canceledReader.finish(provider.Discard); err != nil {
		s.t.Fatal("could not close and discard the canceled SPI reader")
	}
	s.checkSPIPoolInUse(0)
	if open := s.db.Stats().OpenConnections; open != 0 {
		s.t.Fatalf("SPI reader pool has %d open connections after discard, want 0", open)
	}
	s.facts["cancel_cause"] = "context.Canceled"
}

func (s *scenario) checkSPIRecovery() {
	s.t.Helper()
	ctx, cancel := context.WithTimeout(s.t.Context(), 5*time.Second)
	defer cancel()

	reader := s.openSPIReader(ctx)
	s.checkSPIInteger(reader.rows, 1)
	s.checkSPIInteger(reader.rows, 2)
	if row, err := reader.rows.Next(); row != nil || err != io.EOF {
		s.t.Fatal("fresh SPI reader did not reach clean end of stream")
	}
	if err := reader.finish(provider.Reuse); err != nil {
		s.t.Fatal("could not close and release the fresh SPI reader")
	}
	s.checkSPIPoolInUse(0)
	s.facts["pool_in_use_after"] = "0"
}

func (s *scenario) openSPIReader(ctx context.Context) *spiScenarioReader {
	s.t.Helper()
	lease, err := s.readBackend.Acquire(ctx)
	if err != nil || lease == nil {
		s.t.Fatal("could not acquire a provider SPI lease")
	}
	reader := &spiScenarioReader{lease: lease}
	s.t.Cleanup(func() {
		if err := reader.finish(provider.Discard); err != nil {
			s.t.Error("could not clean up the provider SPI reader")
		}
	})

	reader.rows, err = lease.Query(ctx, spiTwoRows, nil)
	if err != nil || reader.rows == nil {
		s.t.Fatal("could not open the provider SPI reader")
	}
	s.checkSPIPoolInUse(1)
	return reader
}

func (s *scenario) checkSPIInteger(rows provider.Rows, want provider.Integer) {
	s.t.Helper()
	row, err := rows.Next()
	if err != nil || len(row) != 1 {
		s.t.Fatal("SPI reader did not return the expected integer row")
	}
	if value, ok := row[0].(provider.Integer); !ok || value != want {
		s.t.Fatalf("SPI reader did not return integer %d", want)
	}
}

func (s *scenario) checkSPIPoolInUse(want int) {
	s.t.Helper()
	if inUse := s.db.Stats().InUse; inUse != want {
		s.t.Fatalf("SPI reader pool has %d connections in use, want %d", inUse, want)
	}
}

func (r *spiScenarioReader) finish(disposition provider.Disposition) error {
	lease := r.lease
	if lease == nil {
		return nil
	}
	r.lease = nil
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var firstClose, secondClose error
	if r.rows != nil {
		firstClose = r.rows.Close(ctx)
		secondClose = r.rows.Close(ctx)
		if firstClose != nil || secondClose != nil {
			disposition = provider.Discard
		}
	}
	return errors.Join(firstClose, secondClose, lease.Release(ctx, disposition))
}
