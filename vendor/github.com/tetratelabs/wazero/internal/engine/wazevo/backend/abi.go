package backend

import (
	"fmt"

	"github.com/tetratelabs/wazero/internal/engine/wazevo/backend/regalloc"
	"github.com/tetratelabs/wazero/internal/engine/wazevo/ssa"
)

type (
	// FunctionABI represents the ABI information for a function which corresponds to a ssa.Signature.
	FunctionABI struct {
		Initialized bool

		Args, Rets                 []ABIArg
		ArgStackSize, RetStackSize int64

		ArgIntRealRegs   byte
		ArgFloatRealRegs byte
		RetIntRealRegs   byte
		RetFloatRealRegs byte
	}

	// ABIArg represents either argument or return value's location.
	ABIArg struct {
		// Index is the index of the argument.
		Index int
		// Kind is the kind of the argument.
		Kind ABIArgKind
		// Reg is valid if Kind == ABIArgKindReg.
		// This VReg must be based on RealReg.
		Reg regalloc.VReg
		// Offset is valid if Kind == ABIArgKindStack.
		// This is the offset from the beginning of either arg or ret stack slot.
		Offset int64
		// Type is the type of the argument.
		Type ssa.Type
	}

	// ABIArgKind is the kind of ABI argument.
	ABIArgKind byte
)

const (
	// ABIArgKindReg represents an argument passed in a register.
	ABIArgKindReg = iota
	// ABIArgKindStack represents an argument passed in the stack.
	ABIArgKindStack
)

// String implements fmt.Stringer.
func (a *ABIArg) String() string {
	return fmt.Sprintf("args[%d]: %s", a.Index, a.Kind)
}

// String implements fmt.Stringer.
func (a ABIArgKind) String() string {
	switch a {
	case ABIArgKindReg:
		return "reg"
	case ABIArgKindStack:
		return "stack"
	default:
		panic("BUG")
	}
}

// Init initializes the abiImpl for the given signature.
func (a *FunctionABI) Init(sig *ssa.Signature, argResultInts, argResultFloats []regalloc.RealReg) {
	if len(a.Rets) < len(sig.Results) {
		a.Rets = make([]ABIArg, len(sig.Results))
	}
	a.Rets = a.Rets[:len(sig.Results)]
	a.RetStackSize = a.setABIArgs(a.Rets, sig.Results, argResultInts, argResultFloats)
	if argsNum := len(sig.Params); len(a.Args) < argsNum {
		a.Args = make([]ABIArg, argsNum)
	}
	a.Args = a.Args[:len(sig.Params)]
	a.ArgStackSize = a.setABIArgs(a.Args, sig.Params, argResultInts, argResultFloats)

	// Gather the real registers usages in arg/return.
	a.ArgIntRealRegs, a.ArgFloatRealRegs = 0, 0
	a.RetIntRealRegs, a.RetFloatRealRegs = 0, 0
	for i := range a.Rets {
		r := &a.Rets[i]
		if r.Kind == ABIArgKindReg {
			if r.Type.IsInt() {
				a.RetIntRealRegs++
			} else {
				a.RetFloatRealRegs++
			}
		}
	}
	for i := range a.Args {
		arg := &a.Args[i]
		if arg.Kind == ABIArgKindReg {
			if arg.Type.IsInt() {
				a.ArgIntRealRegs++
			} else {
				a.ArgFloatRealRegs++
			}
		}
	}

	a.Initialized = true
}

// setABIArgs sets the ABI arguments in the given slice. This assumes that len(s) >= len(types)
// where if len(s) > len(types), the last elements of s is for the multi-return slot.
func (a *FunctionABI) setABIArgs(s []ABIArg, types []ssa.Type, ints, floats []regalloc.RealReg) (stackSize int64) {
	il, fl := len(ints), len(floats)

	var stackOffset int64
	intParamIndex, floatParamIndex := 0, 0
	for i, typ := range types {
		arg := &s[i]
		arg.Index = i
		arg.Type = typ
		if typ.IsInt() {
			if intParamIndex >= il {
				arg.Kind = ABIArgKindStack
				const slotSize = 8 // Align 8 bytes.
				arg.Offset = stackOffset
				stackOffset += slotSize
			} else {
				arg.Kind = ABIArgKindReg
				arg.Reg = regalloc.FromRealReg(ints[intParamIndex], regalloc.RegTypeInt)
				intParamIndex++
			}
		} else {
			if floatParamIndex >= fl {
				arg.Kind = ABIArgKindStack
				slotSize := int64(8)   // Align at least 8 bytes.
				if typ.Bits() == 128 { // Vector.
					slotSize = 16
				}
				arg.Offset = stackOffset
				stackOffset += slotSize
			} else {
				arg.Kind = ABIArgKindReg
				arg.Reg = regalloc.FromRealReg(floats[floatParamIndex], regalloc.RegTypeFloat)
				floatParamIndex++
			}
		}
	}
	return stackOffset
}

func (a *FunctionABI) AlignedArgResultStackSlotSize() uint32 {
	stackSlotSize := a.RetStackSize + a.ArgStackSize
	// Align stackSlotSize to 16 bytes.
	stackSlotSize = (stackSlotSize + 15) &^ 15
	// Check overflow 32-bit.
	if stackSlotSize > 0xFFFFFFFF {
		panic("ABI stack slot size overflow")
	}
	return uint32(stackSlotSize)
}

func (a *FunctionABI) ABIInfoAsUint64() uint64 {
	return uint64(a.ArgIntRealRegs)<<56 |
		uint64(a.ArgFloatRealRegs)<<48 |
		uint64(a.RetIntRealRegs)<<40 |
		uint64(a.RetFloatRealRegs)<<32 |
		uint64(a.AlignedArgResultStackSlotSize())
}

func ABIInfoFromUint64(info uint64) (argIntRealRegs, argFloatRealRegs, retIntRealRegs, retFloatRealRegs byte, stackSlotSize uint32) {
	return byte(info >> 56), byte(info >> 48), byte(info >> 40), byte(info >> 32), uint32(info)
}
