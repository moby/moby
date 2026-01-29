package arm64

// This file contains the logic to "find and determine operands" for instructions.
// In order to finalize the form of an operand, we might end up merging/eliminating
// the source instructions into an operand whenever possible.

import (
	"fmt"

	"github.com/tetratelabs/wazero/internal/engine/wazevo/backend"
	"github.com/tetratelabs/wazero/internal/engine/wazevo/backend/regalloc"
	"github.com/tetratelabs/wazero/internal/engine/wazevo/ssa"
)

type (
	// operand represents an operand of an instruction whose type is determined by the kind.
	operand struct {
		kind        operandKind
		data, data2 uint64
	}
	operandKind byte
)

// Here's the list of operand kinds. We use the abbreviation of the kind name not only for these consts,
// but also names of functions which return the operand of the kind.
const (
	// operandKindNR represents "NormalRegister" (NR). This is literally the register without any special operation unlike others.
	operandKindNR operandKind = iota
	// operandKindSR represents "Shifted Register" (SR). This is a register which is shifted by a constant.
	// Some of the arm64 instructions can take this kind of operand.
	operandKindSR
	// operandKindER represents "Extended Register (ER). This is a register which is sign/zero-extended to a larger size.
	// Some of the arm64 instructions can take this kind of operand.
	operandKindER
	// operandKindImm12 represents "Immediate 12" (Imm12). This is a 12-bit immediate value which can be either shifted by 12 or not.
	// See asImm12 function for detail.
	operandKindImm12
	// operandKindShiftImm represents "Shifted Immediate" (ShiftImm) used by shift operations.
	operandKindShiftImm
)

// String implements fmt.Stringer for debugging.
func (o operand) format(size byte) string {
	switch o.kind {
	case operandKindNR:
		return formatVRegSized(o.nr(), size)
	case operandKindSR:
		r, amt, sop := o.sr()
		return fmt.Sprintf("%s, %s #%d", formatVRegSized(r, size), sop, amt)
	case operandKindER:
		r, eop, _ := o.er()
		return fmt.Sprintf("%s %s", formatVRegSized(r, size), eop)
	case operandKindImm12:
		imm12, shiftBit := o.imm12()
		if shiftBit == 1 {
			return fmt.Sprintf("#%#x", uint64(imm12)<<12)
		} else {
			return fmt.Sprintf("#%#x", imm12)
		}
	default:
		panic(fmt.Sprintf("unknown operand kind: %d", o.kind))
	}
}

// operandNR encodes the given VReg as an operand of operandKindNR.
func operandNR(r regalloc.VReg) operand {
	return operand{kind: operandKindNR, data: uint64(r)}
}

// nr decodes the underlying VReg assuming the operand is of operandKindNR.
func (o operand) nr() regalloc.VReg {
	return regalloc.VReg(o.data)
}

// operandER encodes the given VReg as an operand of operandKindER.
func operandER(r regalloc.VReg, eop extendOp, to byte) operand {
	if to < 32 {
		panic("TODO?BUG?: when we need to extend to less than 32 bits?")
	}
	return operand{kind: operandKindER, data: uint64(r), data2: uint64(eop)<<32 | uint64(to)}
}

// er decodes the underlying VReg, extend operation, and the target size assuming the operand is of operandKindER.
func (o operand) er() (r regalloc.VReg, eop extendOp, to byte) {
	return regalloc.VReg(o.data), extendOp(o.data2>>32) & 0xff, byte(o.data2 & 0xff)
}

// operandSR encodes the given VReg as an operand of operandKindSR.
func operandSR(r regalloc.VReg, amt byte, sop shiftOp) operand {
	return operand{kind: operandKindSR, data: uint64(r), data2: uint64(amt)<<32 | uint64(sop)}
}

// sr decodes the underlying VReg, shift amount, and shift operation assuming the operand is of operandKindSR.
func (o operand) sr() (r regalloc.VReg, amt byte, sop shiftOp) {
	return regalloc.VReg(o.data), byte(o.data2>>32) & 0xff, shiftOp(o.data2) & 0xff
}

// operandImm12 encodes the given imm12 as an operand of operandKindImm12.
func operandImm12(imm12 uint16, shiftBit byte) operand {
	return operand{kind: operandKindImm12, data: uint64(imm12) | uint64(shiftBit)<<32}
}

// imm12 decodes the underlying imm12 data assuming the operand is of operandKindImm12.
func (o operand) imm12() (v uint16, shiftBit byte) {
	return uint16(o.data), byte(o.data >> 32)
}

// operandShiftImm encodes the given amount as an operand of operandKindShiftImm.
func operandShiftImm(amount byte) operand {
	return operand{kind: operandKindShiftImm, data: uint64(amount)}
}

// shiftImm decodes the underlying shift amount data assuming the operand is of operandKindShiftImm.
func (o operand) shiftImm() byte {
	return byte(o.data)
}

// reg returns the register of the operand if applicable.
func (o operand) reg() regalloc.VReg {
	switch o.kind {
	case operandKindNR:
		return o.nr()
	case operandKindSR:
		r, _, _ := o.sr()
		return r
	case operandKindER:
		r, _, _ := o.er()
		return r
	case operandKindImm12:
		// Does not have a register.
	case operandKindShiftImm:
		// Does not have a register.
	default:
		panic(o.kind)
	}
	return regalloc.VRegInvalid
}

func (o operand) realReg() regalloc.RealReg {
	return o.nr().RealReg()
}

func (o operand) assignReg(v regalloc.VReg) operand {
	switch o.kind {
	case operandKindNR:
		return operandNR(v)
	case operandKindSR:
		_, amt, sop := o.sr()
		return operandSR(v, amt, sop)
	case operandKindER:
		_, eop, to := o.er()
		return operandER(v, eop, to)
	case operandKindImm12:
		// Does not have a register.
	case operandKindShiftImm:
		// Does not have a register.
	}
	panic(o.kind)
}

// ensureValueNR returns an operand of either operandKindER, operandKindSR, or operandKindNR from the given value (defined by `def).
//
// `mode` is used to extend the operand if the bit length is smaller than mode.bits().
// If the operand can be expressed as operandKindImm12, `mode` is ignored.
func (m *machine) getOperand_Imm12_ER_SR_NR(def backend.SSAValueDefinition, mode extMode) (op operand) {
	if !def.IsFromInstr() {
		return operandNR(m.compiler.VRegOf(def.V))
	}

	instr := def.Instr
	if instr.Opcode() == ssa.OpcodeIconst {
		if imm12Op, ok := asImm12Operand(instr.ConstantVal()); ok {
			instr.MarkLowered()
			return imm12Op
		}
	}
	return m.getOperand_ER_SR_NR(def, mode)
}

// getOperand_MaybeNegatedImm12_ER_SR_NR is almost the same as getOperand_Imm12_ER_SR_NR, but this might negate the immediate value.
// If the immediate value is negated, the second return value is true, otherwise always false.
func (m *machine) getOperand_MaybeNegatedImm12_ER_SR_NR(def backend.SSAValueDefinition, mode extMode) (op operand, negatedImm12 bool) {
	if !def.IsFromInstr() {
		return operandNR(m.compiler.VRegOf(def.V)), false
	}

	instr := def.Instr
	if instr.Opcode() == ssa.OpcodeIconst {
		c := instr.ConstantVal()
		if imm12Op, ok := asImm12Operand(c); ok {
			instr.MarkLowered()
			return imm12Op, false
		}

		signExtended := int64(c)
		if def.V.Type().Bits() == 32 {
			signExtended = (signExtended << 32) >> 32
		}
		negatedWithoutSign := -signExtended
		if imm12Op, ok := asImm12Operand(uint64(negatedWithoutSign)); ok {
			instr.MarkLowered()
			return imm12Op, true
		}
	}
	return m.getOperand_ER_SR_NR(def, mode), false
}

// ensureValueNR returns an operand of either operandKindER, operandKindSR, or operandKindNR from the given value (defined by `def).
//
// `mode` is used to extend the operand if the bit length is smaller than mode.bits().
func (m *machine) getOperand_ER_SR_NR(def backend.SSAValueDefinition, mode extMode) (op operand) {
	if !def.IsFromInstr() {
		return operandNR(m.compiler.VRegOf(def.V))
	}

	if m.compiler.MatchInstr(def, ssa.OpcodeSExtend) || m.compiler.MatchInstr(def, ssa.OpcodeUExtend) {
		extInstr := def.Instr

		signed := extInstr.Opcode() == ssa.OpcodeSExtend
		innerExtFromBits, innerExtToBits := extInstr.ExtendFromToBits()
		modeBits, modeSigned := mode.bits(), mode.signed()
		if mode == extModeNone || innerExtToBits == modeBits {
			eop := extendOpFrom(signed, innerExtFromBits)
			extArg := m.getOperand_NR(m.compiler.ValueDefinition(extInstr.Arg()), extModeNone)
			op = operandER(extArg.nr(), eop, innerExtToBits)
			extInstr.MarkLowered()
			return
		}

		if innerExtToBits > modeBits {
			panic("BUG?TODO?: need the results of inner extension to be larger than the mode")
		}

		switch {
		case (!signed && !modeSigned) || (signed && modeSigned):
			// Two sign/zero extensions are equivalent to one sign/zero extension for the larger size.
			eop := extendOpFrom(modeSigned, innerExtFromBits)
			op = operandER(m.compiler.VRegOf(extInstr.Arg()), eop, modeBits)
			extInstr.MarkLowered()
		case (signed && !modeSigned) || (!signed && modeSigned):
			// We need to {sign, zero}-extend the result of the {zero,sign} extension.
			eop := extendOpFrom(modeSigned, innerExtToBits)
			op = operandER(m.compiler.VRegOf(extInstr.Return()), eop, modeBits)
			// Note that we failed to merge the inner extension instruction this case.
		}
		return
	}
	return m.getOperand_SR_NR(def, mode)
}

// ensureValueNR returns an operand of either operandKindSR or operandKindNR from the given value (defined by `def).
//
// `mode` is used to extend the operand if the bit length is smaller than mode.bits().
func (m *machine) getOperand_SR_NR(def backend.SSAValueDefinition, mode extMode) (op operand) {
	if !def.IsFromInstr() {
		return operandNR(m.compiler.VRegOf(def.V))
	}

	if m.compiler.MatchInstr(def, ssa.OpcodeIshl) {
		// Check if the shift amount is constant instruction.
		targetVal, amountVal := def.Instr.Arg2()
		targetVReg := m.getOperand_NR(m.compiler.ValueDefinition(targetVal), extModeNone).nr()
		amountDef := m.compiler.ValueDefinition(amountVal)
		if amountDef.IsFromInstr() && amountDef.Instr.Constant() {
			// If that is the case, we can use the shifted register operand (SR).
			c := byte(amountDef.Instr.ConstantVal()) & (targetVal.Type().Bits() - 1) // Clears the unnecessary bits.
			def.Instr.MarkLowered()
			amountDef.Instr.MarkLowered()
			return operandSR(targetVReg, c, shiftOpLSL)
		}
	}
	return m.getOperand_NR(def, mode)
}

// getOperand_ShiftImm_NR returns an operand of either operandKindShiftImm or operandKindNR from the given value (defined by `def).
func (m *machine) getOperand_ShiftImm_NR(def backend.SSAValueDefinition, mode extMode, shiftBitWidth byte) (op operand) {
	if !def.IsFromInstr() {
		return operandNR(m.compiler.VRegOf(def.V))
	}

	instr := def.Instr
	if instr.Constant() {
		amount := byte(instr.ConstantVal()) & (shiftBitWidth - 1) // Clears the unnecessary bits.
		return operandShiftImm(amount)
	}
	return m.getOperand_NR(def, mode)
}

// ensureValueNR returns an operand of operandKindNR from the given value (defined by `def).
//
// `mode` is used to extend the operand if the bit length is smaller than mode.bits().
func (m *machine) getOperand_NR(def backend.SSAValueDefinition, mode extMode) (op operand) {
	var v regalloc.VReg
	if def.IsFromInstr() && def.Instr.Constant() {
		// We inline all the constant instructions so that we could reduce the register usage.
		v = m.lowerConstant(def.Instr)
		def.Instr.MarkLowered()
	} else {
		v = m.compiler.VRegOf(def.V)
	}

	r := v
	switch inBits := def.V.Type().Bits(); {
	case mode == extModeNone:
	case inBits == 32 && (mode == extModeZeroExtend32 || mode == extModeSignExtend32):
	case inBits == 32 && mode == extModeZeroExtend64:
		extended := m.compiler.AllocateVReg(ssa.TypeI64)
		ext := m.allocateInstr()
		ext.asExtend(extended, v, 32, 64, false)
		m.insert(ext)
		r = extended
	case inBits == 32 && mode == extModeSignExtend64:
		extended := m.compiler.AllocateVReg(ssa.TypeI64)
		ext := m.allocateInstr()
		ext.asExtend(extended, v, 32, 64, true)
		m.insert(ext)
		r = extended
	case inBits == 64 && (mode == extModeZeroExtend64 || mode == extModeSignExtend64):
	}
	return operandNR(r)
}

func asImm12Operand(val uint64) (op operand, ok bool) {
	v, shiftBit, ok := asImm12(val)
	if !ok {
		return operand{}, false
	}
	return operandImm12(v, shiftBit), true
}

func asImm12(val uint64) (v uint16, shiftBit byte, ok bool) {
	const mask1, mask2 uint64 = 0xfff, 0xfff_000
	if val&^mask1 == 0 {
		return uint16(val), 0, true
	} else if val&^mask2 == 0 {
		return uint16(val >> 12), 1, true
	} else {
		return 0, 0, false
	}
}
