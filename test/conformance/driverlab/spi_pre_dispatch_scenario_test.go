package driverlab

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/igorpooh1978/GoEF/provider"
)

func (s *scenario) checkSPIPreDispatch() {
	s.t.Helper()
	ctx, stop := context.WithTimeout(s.t.Context(), 5*time.Second)
	defer stop()

	lease, err := s.readBackend.Acquire(ctx)
	if err != nil || lease == nil {
		s.t.Fatal("could not acquire the pre-dispatch SPI fixture lease")
	}
	reader := &spiScenarioReader{lease: lease}
	s.t.Cleanup(func() {
		if err := reader.finish(provider.Discard); err != nil {
			s.t.Error("could not clean up the pre-dispatch SPI fixture lease")
		}
	})
	measured, ok := lease.(*spiReadLease)
	if !ok {
		s.t.Fatal("could not measure calls to database/sql on the SPI fixture lease")
	}
	s.checkSPIPoolInUse(1)

	canceled, cancel := context.WithCancel(ctx)
	cancel()
	expired, stopExpired := context.WithDeadline(ctx, time.Now().Add(-time.Second))
	defer stopExpired()

	for _, operation := range []struct {
		ctx   context.Context
		cause error
	}{
		{canceled, context.Canceled},
		{expired, context.DeadlineExceeded},
	} {
		reader.rows, err = lease.Query(operation.ctx, spiTwoRows, nil)
		if reader.rows != nil || !errors.Is(err, operation.cause) || errors.Is(err, io.EOF) {
			s.t.Fatalf("SPI query did not preserve the pre-dispatch cause %v", operation.cause)
		}
		if measured.queryDispatches != 0 {
			s.t.Fatal("SPI query reached database/sql with an already-ended context")
		}
		s.checkSPIPoolInUse(1)
	}

	reader.rows, err = lease.Query(ctx, spiTwoRows, nil)
	if err != nil || reader.rows == nil {
		s.t.Fatal("could not open the live query on the same SPI fixture lease")
	}
	s.checkSPIInteger(reader.rows, 1)
	s.checkSPIInteger(reader.rows, 2)
	if row, err := reader.rows.Next(); row != nil || err != io.EOF {
		s.t.Fatal("live pre-dispatch control query did not reach clean end of stream")
	}
	if measured.queryDispatches != 1 {
		s.t.Fatal("live SPI query did not reach database/sql exactly once")
	}
	if err := reader.finish(provider.Reuse); err != nil {
		s.t.Fatal("could not return the pre-dispatch SPI fixture lease")
	}
	s.checkSPIPoolInUse(0)
	s.facts["pre_dispatch_canceled"] = "context.Canceled"
	s.facts["pre_dispatch_deadline"] = "context.DeadlineExceeded"
	s.facts["pre_dispatch_calls"] = "0,0,1"
	s.facts["pre_dispatch_pool_in_use_after"] = "0"
}
