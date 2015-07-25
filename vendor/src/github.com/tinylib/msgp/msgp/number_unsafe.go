// +build !appengine

package msgp

import (
	"reflect"
	"unsafe"
)

const (
	// spec says int and uint are always
	// the same size, but that int/uint
	// size may not be machine word size
	smallint = unsafe.Sizeof(int(0)) == 4
)

// UnsafeString returns the byte slice as a volatile string
// THIS SHOULD ONLY BE USED BY THE CODE GENERATOR.
// THIS IS EVIL CODE.
// YOU HAVE BEEN WARNED.
func UnsafeString(b []byte) string {
	return *(*string)(unsafe.Pointer(&reflect.StringHeader{Data: uintptr(unsafe.Pointer(&b[0])), Len: len(b)}))
}

// UnsafeBytes returns the string as a byte slice
// THIS SHOULD ONLY BE USED BY THE CODE GENERATOR.
// THIS IS EVIL CODE.
// YOU HAVE BEEN WARNED.
func UnsafeBytes(s string) []byte {
	return *(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{
		Len:  len(s),
		Cap:  len(s),
		Data: (*(*reflect.StringHeader)(unsafe.Pointer(&s))).Data,
	}))
}

// Number can be
// an int64, uint64, float32,
// or float64 internally.
// It can decode itself
// from any of the native
// messagepack number types.
// The zero-value of Number
// is Int(0). Using the equality
// operator with Number compares
// both the type and the value
// of the number.
type Number struct {
	// internally, this
	// is just a tagged union.
	// the raw bits of the number
	// are stored the same way regardless.
	bits uint64
	typ  Type
}

// AsFloat64 sets the number to
// a float64.
func (n *Number) AsFloat64(f float64) {
	n.typ = Float64Type
	n.bits = *(*uint64)(unsafe.Pointer(&f))
}

// AsInt sets the number to an int64.
func (n *Number) AsInt(i int64) {

	// we always store int(0)
	// as {0, InvalidType} in
	// order to preserve
	// the behavior of the == operator
	if i == 0 {
		n.typ = InvalidType
		n.bits = 0
		return
	}

	n.typ = IntType
	n.bits = uint64(i)
}

// AsUint sets the number to a uint64.
func (n *Number) AsUint(u uint64) {
	n.typ = UintType
	n.bits = u
}

// AsFloat32 sets the number to a float32.
func (n *Number) AsFloat32(f float32) {
	n.typ = Float32Type
	g := float64(f)
	n.bits = *(*uint64)(unsafe.Pointer(&g))
}

// Type will return one of:
// Float64Type, Float32Type, UintType, or IntType.
func (n *Number) Type() Type {
	if n.typ == InvalidType {
		return IntType
	}
	return n.typ
}

// Float casts the number of the float,
// and returns whether or not that was
// the underlying type. (This is legal
// for both float32 and float64 types.)
func (n *Number) Float() (float64, bool) {
	return *(*float64)(unsafe.Pointer(&n.bits)), n.typ == Float64Type || n.typ == Float32Type
}

// Int casts the number as an int64, and
// returns whether or not that was the
// underlying type.
func (n *Number) Int() (int64, bool) {
	return int64(n.bits), n.typ == IntType || n.typ == InvalidType
}

// Uint casts the number as a uint64, and returns
// whether or not that was the underlying type.
func (n *Number) Uint() (uint64, bool) {
	return n.bits, n.typ == UintType
}

// EncodeMsg implements msgp.Encodable
func (n *Number) EncodeMsg(w *Writer) error {
	switch n.typ {
	case InvalidType:
		return w.WriteInt(0)
	case IntType:
		return w.WriteInt64(int64(n.bits))
	case UintType:
		return w.WriteUint64(n.bits)
	case Float64Type:
		return w.WriteFloat64(*(*float64)(unsafe.Pointer(&n.bits)))
	case Float32Type:
		return w.WriteFloat32(float32(*(*float64)(unsafe.Pointer(&n.bits))))
	default:
		// this should never ever happen
		panic("(*Number).typ is invalid")
	}
}

// MarshalMsg implements msgp.Marshaler
func (n *Number) MarshalMsg(b []byte) ([]byte, error) {
	switch n.typ {
	case InvalidType:
		return AppendInt(b, 0), nil
	case IntType:
		return AppendInt64(b, int64(n.bits)), nil
	case UintType:
		return AppendUint64(b, n.bits), nil
	case Float64Type:
		return AppendFloat64(b, *(*float64)(unsafe.Pointer(&n.bits))), nil
	case Float32Type:
		return AppendFloat32(b, float32(*(*float64)(unsafe.Pointer(&n.bits)))), nil
	default:
		panic("(*Number).typ is invalid")
	}
}
