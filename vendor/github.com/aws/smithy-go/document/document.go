package document

import (
	"fmt"
	"math/big"
	"strconv"
	"time"
)

// Marshaler is an interface for a type that marshals a document to its protocol-specific byte representation and
// returns the resulting bytes. A non-nil error will be returned if an error is encountered during marshaling.
//
// Marshal supports basic scalars (int,uint,float,bool,string), big.Int, and big.Float, maps, slices, and structs.
// Anonymous nested types are flattened based on Go anonymous type visibility.
//
// When defining struct types. the `document` struct tag can be used to control how the value will be
// marshaled into the resulting protocol document.
//
//	// Field is ignored
//	Field int `document:"-"`
//
//	// Field object of key "myName"
//	Field int `document:"myName"`
//
//	// Field object key of key "myName", and
//	// Field is omitted if the field is a zero value for the type.
//	Field int `document:"myName,omitempty"`
//
//	// Field object key of "Field", and
//	// Field is omitted if the field is a zero value for the type.
//	Field int `document:",omitempty"`
//
// All struct fields, including anonymous fields, are marshaled unless the
// any of the following conditions are meet.
//
//   - the field is not exported
//   - document field tag is "-"
//   - document field tag specifies "omitempty", and is a zero value.
//
// Pointer and interface values are encoded as the value pointed to or
// contained in the interface. A nil value encodes as a null
// value unless `omitempty` struct tag is provided.
//
// Channel, complex, and function values are not encoded and will be skipped
// when walking the value to be marshaled.
//
// time.Time is not supported and will cause the Marshaler to return an error. These values should be represented
// by your application as a string or numerical representation.
//
// Errors that occur when marshaling will stop the marshaler, and return the error.
//
// Marshal cannot represent cyclic data structures and will not handle them.
// Passing cyclic structures to Marshal will result in an infinite recursion.
//
// Marshaler is not used in schema-serde based services (which are currently
// being rolled out) since having an implementation of Marshaler locks a
// document into support for a specific serial format. Existing implementations
// of Marshaler will continue to encode to JSON as that is effectively the only
// serial format supported for Document prior to the introduction of
// schema-serde. In schema-serde services it is replaced by [Value].
type Marshaler interface {
	MarshalSmithyDocument() ([]byte, error)
}

// Unmarshaler is an interface for a type that unmarshals a document from its protocol-specific representation, and
// stores the result into the value pointed by v. If v is nil or not a pointer then InvalidUnmarshalError will be
// returned.
//
// Unmarshaler supports the same encodings produced by a document Marshaler. This includes support for the `document`
// struct field tag for controlling how struct fields are unmarshaled.
//
// Both generic interface{} and concrete types are valid unmarshal destination types. When unmarshaling a document
// into an empty interface the Unmarshaler will store one of these values:
//
//	bool,                   for boolean values
//	document.Number,        for arbitrary-precision numbers (int64, float64, big.Int, big.Float)
//	string,                 for string values
//	[]interface{},          for array values
//	map[string]interface{}, for objects
//	nil,                    for null values
//
// When unmarshaling, any error that occurs will halt the unmarshal and return the error.
type Unmarshaler interface {
	UnmarshalSmithyDocument(v interface{}) error
}

// Value is a sealed type representing a Smithy document value. It covers the
// full Smithy data model including blob and timestamp.
//
// The following types implement Value:
//   - [Null]
//   - [Boolean]
//   - [Number]
//   - [String]
//   - [Blob]
//   - [Timestamp]
//   - [List]
//   - [Map]
//   - [Structure]
//   - [Opaque]
type Value interface {
	isValue()
}

// Null is a document null value.
type Null struct{}

func (Null) isValue() {}

// Boolean is a document boolean value.
type Boolean bool

func (Boolean) isValue() {}

// String is a document string value.
type String string

func (String) isValue() {}

// Blob is a document blob value.
type Blob []byte

func (Blob) isValue() {}

// Timestamp is a document timestamp value.
type Timestamp time.Time

func (Timestamp) isValue() {}

// List is a document list value.
type List []Value

func (List) isValue() {}

// Map is a document map value with string keys.
type Map map[string]Value

func (Map) isValue() {}

// Structure is a document structure value with an optional discriminator
// identifying the shape it represents.
type Structure struct {
	// Discriminator is the absolute shape ID (e.g.
	// "com.example#MyShape") of the concrete type this structure
	// represents. It may be empty if the type is unknown.
	Discriminator string

	// Members maps member names to their document values.
	Members map[string]Value
}

func (Structure) isValue() {}

// Opaque wraps an arbitrary Go value for backward compatibility with the
// legacy reflection-based document serialization path.
type Opaque struct {
	Value any
}

func (Opaque) isValue() {}

type noSerde interface {
	noSmithyDocumentSerde()
}

// NoSerde is a sentinel value to indicate that a given type should not be marshaled or unmarshaled
// into a protocol document.
type NoSerde struct{}

func (n NoSerde) noSmithyDocumentSerde() {}

var _ noSerde = (*NoSerde)(nil)

// IsNoSerde returns whether the given type implements the no smithy document serde interface.
func IsNoSerde(x interface{}) bool {
	_, ok := x.(noSerde)
	return ok
}

// Number is an arbitrary precision numerical value
type Number string

func (Number) isValue() {}

// Int64 returns the number as a string.
func (n Number) String() string {
	return string(n)
}

// Int64 returns the number as an int64.
func (n Number) Int64() (int64, error) {
	return n.intOfBitSize(64)
}

func (n Number) intOfBitSize(bitSize int) (int64, error) {
	return strconv.ParseInt(string(n), 10, bitSize)
}

// Uint64 returns the number as a uint64.
func (n Number) Uint64() (uint64, error) {
	return n.uintOfBitSize(64)
}

func (n Number) uintOfBitSize(bitSize int) (uint64, error) {
	return strconv.ParseUint(string(n), 10, bitSize)
}

// Float32 returns the number parsed as a 32-bit float, returns a float64.
func (n Number) Float32() (float64, error) {
	return n.floatOfBitSize(32)
}

// Float64 returns the number as a float64.
func (n Number) Float64() (float64, error) {
	return n.floatOfBitSize(64)
}

// Float64 returns the number as a float64.
func (n Number) floatOfBitSize(bitSize int) (float64, error) {
	return strconv.ParseFloat(string(n), bitSize)
}

// BigFloat attempts to convert the number to a big.Float, returns an error if the operation fails.
func (n Number) BigFloat() (*big.Float, error) {
	f, ok := (&big.Float{}).SetString(string(n))
	if !ok {
		return nil, fmt.Errorf("failed to convert to big.Float")
	}
	return f, nil
}

// BigInt attempts to convert the number to a big.Int, returns an error if the operation fails.
func (n Number) BigInt() (*big.Int, error) {
	f, ok := (&big.Int{}).SetString(string(n), 10)
	if !ok {
		return nil, fmt.Errorf("failed to convert to big.Float")
	}
	return f, nil
}
