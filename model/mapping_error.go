package model

import (
	"fmt"

	"github.com/igorpooh1978/GoEF/goeferr"
)

// MappingPosition locates a declaration using one-based positions. Entity is
// the registration order, Property is the Go struct field position, Option is
// the fluent option order, and TagOffset is a byte position in the decoded
// goef tag. A zero coordinate is not applicable to that location.
type MappingPosition struct {
	Entity    int
	Property  int
	Option    int
	TagOffset int
}

// MappingError reports invalid Code First configuration without exposing raw
// identifiers or values. Previous is populated for a two-declaration conflict.
type MappingError struct {
	Position MappingPosition
	Previous MappingPosition
	reason   string
	cause    error
}

// Error reports the declaration locations and safe problem description.
func (e *MappingError) Error() string {
	if e == nil || e.Position.Entity < 1 {
		return "invalid model mapping"
	}
	at := e.Position.description()
	if e.Previous.Entity > 0 {
		at += " and " + e.Previous.description()
	}
	return fmt.Sprintf("invalid model mapping at %s (%s)", at, e.reason)
}

func (p MappingPosition) description() string {
	location := fmt.Sprintf("entity %d", p.Entity)
	if p.Property > 0 {
		location += fmt.Sprintf(", field %d", p.Property)
	}
	if p.Option > 0 {
		location += fmt.Sprintf(", option %d", p.Option)
	}
	if p.TagOffset > 0 {
		location += fmt.Sprintf(", tag offset %d", p.TagOffset)
	}
	return location
}

// Unwrap returns the portable invalid-model cause, if one was recorded.
func (e *MappingError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func invalidMapping(at, previous MappingPosition, reason string) *MappingError {
	return &MappingError{Position: at, Previous: previous, reason: reason, cause: goeferr.NewInvalidModel()}
}

func mappingFacetError(err error, at MappingPosition) *MappingError {
	conflict, ok := err.(*facetConflict)
	if !ok {
		return invalidMapping(at, MappingPosition{}, "invalid facet declaration")
	}
	first, second := at, at
	switch conflict.source {
	case tagSource:
		first.TagOffset, second.TagOffset = conflict.first.offset, conflict.second.offset
	case fluentSource:
		first.Option, second.Option = conflict.first.input, conflict.second.input
	}
	return invalidMapping(second, first, "conflicting "+conflict.facet+" declarations")
}
