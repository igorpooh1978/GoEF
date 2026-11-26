package driverlab

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/igorpooh1978/GoEF/provider"
)

func (s *scenario) checkSPIAcquireCancellation() {
	s.t.Helper()

	ctx, stop := context.WithTimeout(s.t.Context(), 15*time.Second)
	defer stop()
	lease, err := s.readBackend.Acquire(ctx)
	if err != nil || lease == nil {
		s.t.Fatal("could not acquire the only SPI fixture lease")
	}
	holder := &spiScenarioReader{lease: lease}

	initial := s.db.Stats()
	if initial.MaxOpenConnections != 1 || initial.InUse != 1 {
		if err := holder.finish(provider.Reuse); err != nil {
			s.t.Error("could not release the SPI fixture holder")
		}
		s.t.Fatal("SPI acquisition fixture requires a full one-slot pool")
	}

	waiterCtx, cancelWaiter := context.WithCancel(ctx)
	done := make(chan struct{})
	var waiterLease provider.Lease
	var waiterErr error
	go func() {
		defer close(done)
		waiterLease, waiterErr = s.readBackend.Acquire(waiterCtx)
	}()
	defer func() {
		cancelWaiter()
		// Return the held slot before joining on failure, even if the waiter
		// ignores cancellation and can only complete when the pool has room.
		if err := holder.finish(provider.Reuse); err != nil {
			s.t.Error("could not release the SPI fixture holder")
		}
		<-done
		if waiterLease != nil {
			unexpected := spiScenarioReader{lease: waiterLease}
			if err := unexpected.finish(provider.Discard); err != nil {
				s.t.Error("could not discard the unexpected SPI waiter lease")
			}
		}
	}()

	watchdog, stopWatchdog := context.WithTimeout(s.t.Context(), 5*time.Second)
	defer stopWatchdog()
	poll := time.NewTicker(5 * time.Millisecond)
	defer poll.Stop()
	for s.db.Stats().WaitCount == initial.WaitCount {
		select {
		case <-done:
			s.t.Fatal("SPI fixture waiter returned before entering the pool queue")
		case <-watchdog.Done():
			s.t.Fatal("could not observe the SPI acquisition in the pool queue")
		case <-poll.C:
		}
	}
	if s.db.Stats().WaitCount-initial.WaitCount != 1 || waiterCtx.Err() != nil {
		s.t.Fatal("SPI fixture did not observe one live acquisition entering the pool queue")
	}
	s.checkSPIPoolInUse(1)
	select {
	case <-done:
		s.t.Fatal("queued SPI acquisition returned while the holder still owned the slot")
	default:
	}

	cancelWaiter()
	select {
	case <-done:
	case <-watchdog.Done():
		s.t.Fatal("queued SPI acquisition did not return after caller cancellation")
	}
	if waiterLease != nil || !errors.Is(waiterErr, context.Canceled) {
		s.t.Fatal("queued SPI acquisition did not preserve caller cancellation without a lease")
	}
	if s.db.Stats().WaitCount-initial.WaitCount != 1 {
		s.t.Fatal("canceled SPI acquisition did not account for exactly one pool wait")
	}
	s.checkSPIPoolInUse(1)

	if err := holder.finish(provider.Reuse); err != nil {
		s.t.Fatal("could not return the SPI fixture holder")
	}
	s.checkSPIPoolInUse(0)

	reader := s.openSPIReader(ctx)
	s.checkSPIInteger(reader.rows, 1)
	s.checkSPIInteger(reader.rows, 2)
	if row, err := reader.rows.Next(); row != nil || err != io.EOF {
		s.t.Fatal("SPI acquisition recovery query did not reach clean end of stream")
	}
	if err := reader.finish(provider.Reuse); err != nil {
		s.t.Fatal("could not return the SPI acquisition recovery lease")
	}
	s.checkSPIPoolInUse(0)
	if s.db.Stats().WaitCount-initial.WaitCount != 1 {
		s.t.Fatal("the control query had to wait for another connection")
	}
	s.facts["acquire_cancel_cause"] = "context.Canceled"
	s.facts["acquire_wait_count_delta"] = "1"
	s.facts["acquire_pool_in_use_after"] = "0"
}
