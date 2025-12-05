package regalloc

import (
	"fmt"

	"github.com/tetratelabs/wazero/internal/engine/wazevo/ssa"
)

// VReg represents a register which is assigned to an SSA value. This is used to represent a register in the backend.
// A VReg may or may not be a physical register, and the info of physical register can be obtained by RealReg.
type VReg uint64

// VRegID is the lower 32bit of VReg, which is the pure identifier of VReg without RealReg info.
type VRegID uint32

// RealReg returns the RealReg of this VReg.
func (v VReg) RealReg() RealReg {
	return RealReg(v >> 32)
}

// IsRealReg returns true if this VReg is backed by a physical register.
func (v VReg) IsRealReg() bool {
	return v.RealReg() != RealRegInvalid
}

// FromRealReg returns a VReg from the given RealReg and RegType.
// This is used to represent a specific pre-colored register in the backend.
func FromRealReg(r RealReg, typ RegType) VReg {
	rid := VRegID(r)
	if rid > vRegIDReservedForRealNum {
		panic(fmt.Sprintf("invalid real reg %d", r))
	}
	return VReg(r).SetRealReg(r).SetRegType(typ)
}

// SetRealReg sets the RealReg of this VReg and returns the updated VReg.
func (v VReg) SetRealReg(r RealReg) VReg {
	return VReg(r)<<32 | (v & 0xff_00_ffffffff)
}

// RegType returns the RegType of this VReg.
func (v VReg) RegType() RegType {
	return RegType(v >> 40)
}

// SetRegType sets the RegType of this VReg and returns the updated VReg.
func (v VReg) SetRegType(t RegType) VReg {
	return VReg(t)<<40 | (v & 0x00_ff_ffffffff)
}

// ID returns the VRegID of this VReg.
func (v VReg) ID() VRegID {
	return VRegID(v & 0xffffffff)
}

// Valid returns true if this VReg is Valid.
func (v VReg) Valid() bool {
	return v.ID() != vRegIDInvalid && v.RegType() != RegTypeInvalid
}

// RealReg represents a physical register.
type RealReg byte

const RealRegInvalid RealReg = 0

const (
	vRegIDInvalid            VRegID = 1 << 31
	VRegIDNonReservedBegin          = vRegIDReservedForRealNum
	vRegIDReservedForRealNum VRegID = 128
	VRegInvalid                     = VReg(vRegIDInvalid)
)

// String implements fmt.Stringer.
func (r RealReg) String() string {
	switch r {
	case RealRegInvalid:
		return "invalid"
	default:
		return fmt.Sprintf("r%d", r)
	}
}

// String implements fmt.Stringer.
func (v VReg) String() string {
	if v.IsRealReg() {
		return fmt.Sprintf("r%d", v.ID())
	}
	return fmt.Sprintf("v%d?", v.ID())
}

// RegType represents the type of a register.
type RegType byte

const (
	RegTypeInvalid RegType = iota
	RegTypeInt
	RegTypeFloat
	NumRegType
)

// String implements fmt.Stringer.
func (r RegType) String() string {
	switch r {
	case RegTypeInt:
		return "int"
	case RegTypeFloat:
		return "float"
	default:
		return "invalid"
	}
}

// RegTypeOf returns the RegType of the given ssa.Type.
func RegTypeOf(p ssa.Type) RegType {
	switch p {
	case ssa.TypeI32, ssa.TypeI64:
		return RegTypeInt
	case ssa.TypeF32, ssa.TypeF64, ssa.TypeV128:
		return RegTypeFloat
	default:
		panic("invalid type")
	}
}
