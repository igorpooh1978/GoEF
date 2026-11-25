package provider

import "time"

// Kind identifies a portable transport value. The zero value is invalid.
type Kind uint8

// Supported transport kinds in v1. KindNull is used only for values.
const (
	KindNull Kind = iota + 1
	KindInteger
	KindFloat
	KindBoolean
	KindText
	KindBinary
	KindTimestamp
)

// Value is one of the concrete transport values declared in this package.
// Domain-specific types must be converted by a codec before dispatch.
type Value interface {
	Kind() Kind
	providerValue()
}

// Null is an explicit SQL NULL. A nil Value interface is invalid.
type Null struct{}

// Integer carries a signed 64-bit value.
type Integer int64

// Float carries a binary 64-bit floating-point value.
type Float float64

// Boolean carries a true or false value.
type Boolean bool

// Text carries a string value.
type Text string

// Binary carries bytes. A nil Binary slice counts as empty binary data.
type Binary []byte

// Timestamp carries a time value. Precision and timezone mapping belong to the codec.
type Timestamp time.Time

// Kind returns the kind of a null.
func (Null) Kind() Kind { return KindNull }

// Kind returns the kind of an integer.
func (Integer) Kind() Kind { return KindInteger }

// Kind returns the kind of a float.
func (Float) Kind() Kind { return KindFloat }

// Kind returns the kind of a boolean.
func (Boolean) Kind() Kind { return KindBoolean }

// Kind returns the kind of text.
func (Text) Kind() Kind { return KindText }

// Kind returns the kind of binary data.
func (Binary) Kind() Kind { return KindBinary }

// Kind returns the kind of a timestamp.
func (Timestamp) Kind() Kind { return KindTimestamp }

func (Null) providerValue()      {}
func (Integer) providerValue()   {}
func (Float) providerValue()     {}
func (Boolean) providerValue()   {}
func (Text) providerValue()      {}
func (Binary) providerValue()    {}
func (Timestamp) providerValue() {}
