package ssa

type Type byte

const (
	typeInvalid Type = iota

	// TODO: add 8, 16 bit types when it's needed for optimizations.

	// TypeI32 represents an integer type with 32 bits.
	TypeI32

	// TypeI64 represents an integer type with 64 bits.
	TypeI64

	// TypeF32 represents 32-bit floats in the IEEE 754.
	TypeF32

	// TypeF64 represents 64-bit floats in the IEEE 754.
	TypeF64

	// TypeV128 represents 128-bit SIMD vectors.
	TypeV128

	// -- Do not add new types after this line. ----
	typeEnd
)

// String implements fmt.Stringer.
func (t Type) String() (ret string) {
	switch t {
	case typeInvalid:
		return "invalid"
	case TypeI32:
		return "i32"
	case TypeI64:
		return "i64"
	case TypeF32:
		return "f32"
	case TypeF64:
		return "f64"
	case TypeV128:
		return "v128"
	default:
		panic(int(t))
	}
}

// IsInt returns true if the type is an integer type.
func (t Type) IsInt() bool {
	return t == TypeI32 || t == TypeI64
}

// IsFloat returns true if the type is a floating point type.
func (t Type) IsFloat() bool {
	return t == TypeF32 || t == TypeF64
}

// Bits returns the number of bits required to represent the type.
func (t Type) Bits() byte {
	switch t {
	case TypeI32, TypeF32:
		return 32
	case TypeI64, TypeF64:
		return 64
	case TypeV128:
		return 128
	default:
		panic(int(t))
	}
}

// Size returns the number of bytes required to represent the type.
func (t Type) Size() byte {
	return t.Bits() / 8
}

func (t Type) invalid() bool {
	return t == typeInvalid
}

// VecLane represents a lane in a SIMD vector.
type VecLane byte

const (
	VecLaneInvalid VecLane = 1 + iota
	VecLaneI8x16
	VecLaneI16x8
	VecLaneI32x4
	VecLaneI64x2
	VecLaneF32x4
	VecLaneF64x2
)

// String implements fmt.Stringer.
func (vl VecLane) String() (ret string) {
	switch vl {
	case VecLaneInvalid:
		return "invalid"
	case VecLaneI8x16:
		return "i8x16"
	case VecLaneI16x8:
		return "i16x8"
	case VecLaneI32x4:
		return "i32x4"
	case VecLaneI64x2:
		return "i64x2"
	case VecLaneF32x4:
		return "f32x4"
	case VecLaneF64x2:
		return "f64x2"
	default:
		panic(int(vl))
	}
}
