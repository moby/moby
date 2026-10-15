//go:build 386

package regmask

// A 64 bit shift on 386 is synthesized from 32 bit instructions, and masking its
// count makes the compiler emit more code rather than less, so those masks are
// no-ops here: each is the largest value its count type can hold, which the
// compiler folds away. Shifts of narrower values behave as elsewhere.
const (
	// ShiftNByUint8 - shifting an N bit value by a uint8
	Shift8ByUint8  = 7
	Shift16ByUint8 = 15
	Shift32ByUint8 = 31
	Shift64ByUint8 = 0xff

	// ShiftNByUint16 - shifting an N bit value by a uint16
	Shift8ByUint16  = Shift8ByUint8
	Shift16ByUint16 = Shift16ByUint8
	Shift32ByUint16 = Shift32ByUint8
	Shift64ByUint16 = 0xffff

	// ShiftNByUint32 - shifting an N bit value by a uint32
	Shift8ByUint32  = Shift8ByUint8
	Shift16ByUint32 = Shift16ByUint8
	Shift32ByUint32 = Shift32ByUint8
	Shift64ByUint32 = 0xffffffff

	// ShiftNByUint64 - shifting an N bit value by a uint64
	Shift8ByUint64  = Shift8ByUint8
	Shift16ByUint64 = Shift16ByUint8
	Shift32ByUint64 = Shift32ByUint8
	Shift64ByUint64 = 0xffffffffffffffff

	// ShiftNByUint - shifting an N bit value by a uint
	Shift8ByUint  = Shift8ByUint8
	Shift16ByUint = Shift16ByUint8
	Shift32ByUint = Shift32ByUint8
	Shift64ByUint = ^uint(0)
)
