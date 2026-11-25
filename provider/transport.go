// Package provider defines the versioned execution boundary between GoEF and
// independently implemented database adapters.
package provider

import (
	"context"
	"fmt"
)

// SPIMajor is the required execution contract generation. An incompatible
// required method or semantic change needs a new generation.
const SPIMajor uint16 = 1

// Descriptor identifies an adapter and the execution generation it implements.
type Descriptor struct {
	// ID is a stable adapter identifier and must not contain connection data.
	ID string
	// Major is the adapter's implementation of the execution SPI generation.
	Major uint16
}

// Compatible reports whether the descriptor identifies a v1 adapter.
func (d Descriptor) Compatible() bool {
	return d.ID != "" && d.Major == SPIMajor
}

// Backend supplies the runtime descriptor and connection acquisition. The
// consumer must validate Descriptor before calling Acquire.
type Backend interface {
	// Descriptor returns static metadata without opening a connection.
	Descriptor() Descriptor
	Connector
}

// Connector provides a lease on one physical database connection.
type Connector interface {
	// Acquire transfers one lease on success. On error no lease is transferred.
	Acquire(context.Context) (Lease, error)
}

// Disposition tells the adapter whether a connection may be reused.
type Disposition uint8

const (
	// Discard prevents the physical connection from returning to circulation.
	// It is also the zero value for an unknown or unsafe connection state.
	Discard Disposition = iota
	// Reuse permits reuse only after the adapter confirms a clean connection.
	Reuse
)

// Lease owns exclusive use of one physical connection. Use it serially and
// close any current Rows before issuing another operation or releasing it.
type Lease interface {
	Executor

	// Release consumes the lease exactly once, including on error. Discard
	// unsafe connections and do not close the caller's pool. When ordinary
	// cleanup is uncertain, quarantine the connection before returning.
	// The caller can supply a separate bounded context after cancellation.
	Release(context.Context, Disposition) error
}

// Executor dispatches commands on its bound connection. It never falls back
// to a pool or transparently replays an operation.
type Executor interface {
	// Query transfers an open Rows on success. On failure, the adapter cleans
	// up any resources it acquired and transfers no Rows. The query context
	// controls dispatch and the entire result stream lifetime.
	Query(context.Context, Command, Bindings) (Rows, error)
	// Exec reports the affected-row state for a command that returns no rows.
	Exec(context.Context, Command, Bindings) (WriteResult, error)
}

// Slot describes one input position in a compiled command.
type Slot struct {
	// Name is the adapter's parameter name when the dialect uses one.
	Name string
	// Kind is the expected non-null value kind.
	Kind Kind
	// Nullable permits a Null binding in this slot.
	Nullable bool
}

// Column describes one output position in a compiled command.
type Column struct {
	// Kind is the expected kind for a non-null cell.
	Kind Kind
	// Nullable permits a Null cell in this column.
	Nullable bool
}

// Command is a reusable adapter-specific SQL template and its ordered layouts.
// SQL contains adapter-generated or explicitly authorized SQL, with values
// supplied separately through bindings. Slots and Columns are read-only to the
// adapter. An adapter that retains either slice beyond the call must copy it.
type Command struct {
	SQL     string
	Slots   []Slot
	Columns []Column
}

// ValidateBindings checks the input layout without dispatching SQL or retaining
// values. Adapters can use this before Query and Exec. Diagnostics include only
// slot positions and counts.
func (c Command) ValidateBindings(bindings Bindings) error {
	if len(c.Slots) != len(bindings) {
		return fmt.Errorf("provider bindings: got %d values for %d slots", len(bindings), len(c.Slots))
	}
	for i, slot := range c.Slots {
		if slot.Kind < KindInteger || slot.Kind > KindTimestamp {
			return fmt.Errorf("provider bindings: slot %d has an invalid kind", i+1)
		}
		value := bindings[i]
		switch value.(type) {
		case Null:
			if !slot.Nullable {
				return fmt.Errorf("provider bindings: slot %d does not accept null", i+1)
			}
		case Integer, Float, Boolean, Text, Binary, Timestamp:
			if value.Kind() != slot.Kind {
				return fmt.Errorf("provider bindings: slot %d has the wrong value kind", i+1)
			}
		default:
			return fmt.Errorf("provider bindings: slot %d has an unsupported value type", i+1)
		}
	}
	return nil
}

// Bindings supplies one value per slot, in command order. The adapter must
// validate count, concrete value types and nullability before dispatch. Only
// the declared value types are valid. Reject pointers and nil interfaces. Do not mutate
// bindings or retain them after the call without making a copy. That copy
// includes any Binary slice it needs after the call.
type Bindings []Value

// WriteResult distinguishes a known zero from a count the driver cannot report.
// RowsAffected is nonnegative when known and ignored otherwise.
type WriteResult struct {
	RowsAffected      int64
	RowsAffectedKnown bool
}

// Rows is a serial result stream on the current lease. The caller must close
// it on exhaustion, early return and failure.
type Rows interface {
	// Next returns one row in command-column order, or (nil, io.EOF) on clean
	// exhaustion. A late error is preserved as an error other than io.EOF.
	// The query context must interrupt an in-flight read and remain observable
	// through a causal error instead of appearing as clean exhaustion.
	// The row slice and any Binary cells are borrowed until the next
	// Next or Close call. Copy them before retaining them.
	Next() ([]Value, error)

	// Close releases result resources, is safe to repeat, and reports cleanup
	// failure on the first close. It does not release the enclosing lease.
	Close(context.Context) error
}
