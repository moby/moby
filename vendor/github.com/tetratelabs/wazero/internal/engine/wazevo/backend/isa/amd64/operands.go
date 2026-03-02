package amd64

import (
	"fmt"
	"unsafe"

	"github.com/tetratelabs/wazero/internal/engine/wazevo/backend"
	"github.com/tetratelabs/wazero/internal/engine/wazevo/backend/regalloc"
	"github.com/tetratelabs/wazero/internal/engine/wazevo/ssa"
	"github.com/tetratelabs/wazero/internal/engine/wazevo/wazevoapi"
)

type operand struct {
	kind operandKind
	data uint64
}

type operandKind byte

const (
	// operandKindReg is an operand which is an integer Register.
	operandKindReg operandKind = iota + 1

	// operandKindMem is a value in Memory.
	// 32, 64, or 128 bit value.
	operandKindMem

	// operandKindImm32 is a signed-32-bit integer immediate value.
	operandKindImm32

	// operandKindLabel is a label.
	operandKindLabel
)

// String implements fmt.Stringer.
func (o operandKind) String() string {
	switch o {
	case operandKindReg:
		return "reg"
	case operandKindMem:
		return "mem"
	case operandKindImm32:
		return "imm32"
	case operandKindLabel:
		return "label"
	default:
		panic("BUG: invalid operand kind")
	}
}

// format returns the string representation of the operand.
// _64 is only for the case where the operand is a register, and it's integer.
func (o *operand) format(_64 bool) string {
	switch o.kind {
	case operandKindReg:
		return formatVRegSized(o.reg(), _64)
	case operandKindMem:
		return o.addressMode().String()
	case operandKindImm32:
		return fmt.Sprintf("$%d", int32(o.imm32()))
	case operandKindLabel:
		return label(o.imm32()).String()
	default:
		panic(fmt.Sprintf("BUG: invalid operand: %s", o.kind))
	}
}

//go:inline
func (o *operand) reg() regalloc.VReg {
	return regalloc.VReg(o.data)
}

//go:inline
func (o *operand) setReg(r regalloc.VReg) {
	o.data = uint64(r)
}

//go:inline
func (o *operand) addressMode() *amode {
	return wazevoapi.PtrFromUintptr[amode](uintptr(o.data))
}

//go:inline
func (o *operand) imm32() uint32 {
	return uint32(o.data)
}

func (o *operand) label() label {
	switch o.kind {
	case operandKindLabel:
		return label(o.data)
	case operandKindMem:
		mem := o.addressMode()
		if mem.kind() != amodeRipRel {
			panic("BUG: invalid label")
		}
		return label(mem.imm32)
	default:
		panic("BUG: invalid operand kind")
	}
}

func newOperandLabel(label label) operand {
	return operand{kind: operandKindLabel, data: uint64(label)}
}

func newOperandReg(r regalloc.VReg) operand {
	return operand{kind: operandKindReg, data: uint64(r)}
}

func newOperandImm32(imm32 uint32) operand {
	return operand{kind: operandKindImm32, data: uint64(imm32)}
}

func newOperandMem(amode *amode) operand {
	return operand{kind: operandKindMem, data: uint64(uintptr(unsafe.Pointer(amode)))}
}

// amode is a memory operand (addressing mode).
type amode struct {
	kindWithShift uint32
	imm32         uint32
	base          regalloc.VReg

	// For amodeRegRegShift:
	index regalloc.VReg
}

type amodeKind byte

const (
	// amodeRegRegShift calculates sign-extend-32-to-64(Immediate) + base
	amodeImmReg amodeKind = iota + 1

	// amodeImmRBP is the same as amodeImmReg, but the base register is fixed to RBP.
	// The only differece is that it doesn't tell the register allocator to use RBP which is distracting for the
	// register allocator.
	amodeImmRBP

	// amodeRegRegShift calculates sign-extend-32-to-64(Immediate) + base + (Register2 << Shift)
	amodeRegRegShift

	// amodeRipRel is a RIP-relative addressing mode specified by the label.
	amodeRipRel

	// TODO: there are other addressing modes such as the one without base register.
)

func (a *amode) kind() amodeKind {
	return amodeKind(a.kindWithShift & 0xff)
}

func (a *amode) shift() byte {
	return byte(a.kindWithShift >> 8)
}

func (a *amode) uses(rs *[]regalloc.VReg) {
	switch a.kind() {
	case amodeImmReg:
		*rs = append(*rs, a.base)
	case amodeRegRegShift:
		*rs = append(*rs, a.base, a.index)
	case amodeImmRBP, amodeRipRel:
	default:
		panic("BUG: invalid amode kind")
	}
}

func (a *amode) nregs() int {
	switch a.kind() {
	case amodeImmReg:
		return 1
	case amodeRegRegShift:
		return 2
	case amodeImmRBP, amodeRipRel:
		return 0
	default:
		panic("BUG: invalid amode kind")
	}
}

func (a *amode) assignUses(i int, reg regalloc.VReg) {
	switch a.kind() {
	case amodeImmReg:
		if i == 0 {
			a.base = reg
		} else {
			panic("BUG: invalid amode assignment")
		}
	case amodeRegRegShift:
		if i == 0 {
			a.base = reg
		} else if i == 1 {
			a.index = reg
		} else {
			panic("BUG: invalid amode assignment")
		}
	default:
		panic("BUG: invalid amode assignment")
	}
}

func (m *machine) newAmodeImmReg(imm32 uint32, base regalloc.VReg) *amode {
	ret := m.amodePool.Allocate()
	*ret = amode{kindWithShift: uint32(amodeImmReg), imm32: imm32, base: base}
	return ret
}

func (m *machine) newAmodeImmRBPReg(imm32 uint32) *amode {
	ret := m.amodePool.Allocate()
	*ret = amode{kindWithShift: uint32(amodeImmRBP), imm32: imm32, base: rbpVReg}
	return ret
}

func (m *machine) newAmodeRegRegShift(imm32 uint32, base, index regalloc.VReg, shift byte) *amode {
	if shift > 3 {
		panic(fmt.Sprintf("BUG: invalid shift (must be 3>=): %d", shift))
	}
	ret := m.amodePool.Allocate()
	*ret = amode{kindWithShift: uint32(amodeRegRegShift) | uint32(shift)<<8, imm32: imm32, base: base, index: index}
	return ret
}

func (m *machine) newAmodeRipRel(label label) *amode {
	ret := m.amodePool.Allocate()
	*ret = amode{kindWithShift: uint32(amodeRipRel), imm32: uint32(label)}
	return ret
}

// String implements fmt.Stringer.
func (a *amode) String() string {
	switch a.kind() {
	case amodeImmReg, amodeImmRBP:
		if a.imm32 == 0 {
			return fmt.Sprintf("(%s)", formatVRegSized(a.base, true))
		}
		return fmt.Sprintf("%d(%s)", int32(a.imm32), formatVRegSized(a.base, true))
	case amodeRegRegShift:
		shift := 1 << a.shift()
		if a.imm32 == 0 {
			return fmt.Sprintf(
				"(%s,%s,%d)",
				formatVRegSized(a.base, true), formatVRegSized(a.index, true), shift)
		}
		return fmt.Sprintf(
			"%d(%s,%s,%d)",
			int32(a.imm32), formatVRegSized(a.base, true), formatVRegSized(a.index, true), shift)
	case amodeRipRel:
		return fmt.Sprintf("%s(%%rip)", label(a.imm32))
	default:
		panic("BUG: invalid amode kind")
	}
}

func (m *machine) getOperand_Mem_Reg(def backend.SSAValueDefinition) (op operand) {
	if !def.IsFromInstr() {
		return newOperandReg(m.c.VRegOf(def.V))
	}

	if def.V.Type() == ssa.TypeV128 {
		// SIMD instructions require strict memory alignment, so we don't support the memory operand for V128 at the moment.
		return m.getOperand_Reg(def)
	}

	if m.c.MatchInstr(def, ssa.OpcodeLoad) {
		instr := def.Instr
		ptr, offset, _ := instr.LoadData()
		op = newOperandMem(m.lowerToAddressMode(ptr, offset))
		instr.MarkLowered()
		return op
	}
	return m.getOperand_Reg(def)
}

func (m *machine) getOperand_Mem_Imm32_Reg(def backend.SSAValueDefinition) (op operand) {
	if !def.IsFromInstr() {
		return newOperandReg(m.c.VRegOf(def.V))
	}

	if m.c.MatchInstr(def, ssa.OpcodeLoad) {
		instr := def.Instr
		ptr, offset, _ := instr.LoadData()
		op = newOperandMem(m.lowerToAddressMode(ptr, offset))
		instr.MarkLowered()
		return op
	}
	return m.getOperand_Imm32_Reg(def)
}

func (m *machine) getOperand_Imm32_Reg(def backend.SSAValueDefinition) (op operand) {
	if !def.IsFromInstr() {
		return newOperandReg(m.c.VRegOf(def.V))
	}

	instr := def.Instr
	if instr.Constant() {
		// If the operation is 64-bit, x64 sign-extends the 32-bit immediate value.
		// Therefore, we need to check if the immediate value is within the 32-bit range and if the sign bit is set,
		// we should not use the immediate value.
		if op, ok := asImm32Operand(instr.ConstantVal(), instr.Return().Type() == ssa.TypeI32); ok {
			instr.MarkLowered()
			return op
		}
	}
	return m.getOperand_Reg(def)
}

func asImm32Operand(val uint64, allowSignExt bool) (operand, bool) {
	if imm32, ok := asImm32(val, allowSignExt); ok {
		return newOperandImm32(imm32), true
	}
	return operand{}, false
}

func asImm32(val uint64, allowSignExt bool) (uint32, bool) {
	u32val := uint32(val)
	if uint64(u32val) != val {
		return 0, false
	}
	if !allowSignExt && u32val&0x80000000 != 0 {
		return 0, false
	}
	return u32val, true
}

func (m *machine) getOperand_Reg(def backend.SSAValueDefinition) (op operand) {
	var v regalloc.VReg
	if instr := def.Instr; instr != nil && instr.Constant() {
		// We inline all the constant instructions so that we could reduce the register usage.
		v = m.lowerConstant(instr)
		instr.MarkLowered()
	} else {
		v = m.c.VRegOf(def.V)
	}
	return newOperandReg(v)
}
