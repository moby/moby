//go:build !386

// Package regmask provides masks for variable shift counts.
//
// Go defines a shift by more than the width of the shifted value as producing
// zero, so the compiler has to guard every variable shift against that: on
// amd64 with a CMP and an SBB, on arm64 with a CMP and a CSEL, and so on. When
// the count is known to be smaller than the width anyway, masking it with the
// width minus one lets the compiler drop the guard. The mask itself costs
// nothing on the architectures whose shift instructions already use only the
// low bits of the count, which the compiler knows about.
//
// Each constant is named for the width of the value being shifted and the type
// of the shift count, and may only be used where the count is always smaller
// than that width:
//
//	x := y >> (n & regmask.Shift64ByUint) // y is 64 bits, n is a uint below 64
package regmask

const (
	// ShiftNByUint8 - shifting an N bit value by a uint8
	Shift8ByUint8  = 7
	Shift16ByUint8 = 15
	Shift32ByUint8 = 31
	Shift64ByUint8 = 63

	// ShiftNByUint16 - shifting an N bit value by a uint16
	Shift8ByUint16  = Shift8ByUint8
	Shift16ByUint16 = Shift16ByUint8
	Shift32ByUint16 = Shift32ByUint8
	Shift64ByUint16 = Shift64ByUint8

	// ShiftNByUint32 - shifting an N bit value by a uint32
	Shift8ByUint32  = Shift8ByUint8
	Shift16ByUint32 = Shift16ByUint8
	Shift32ByUint32 = Shift32ByUint8
	Shift64ByUint32 = Shift64ByUint8

	// ShiftNByUint64 - shifting an N bit value by a uint64
	Shift8ByUint64  = Shift8ByUint8
	Shift16ByUint64 = Shift16ByUint8
	Shift32ByUint64 = Shift32ByUint8
	Shift64ByUint64 = Shift64ByUint8

	// ShiftNByUint - shifting an N bit value by a uint
	Shift8ByUint  = Shift8ByUint8
	Shift16ByUint = Shift16ByUint8
	Shift32ByUint = Shift32ByUint8
	Shift64ByUint = Shift64ByUint8
)
