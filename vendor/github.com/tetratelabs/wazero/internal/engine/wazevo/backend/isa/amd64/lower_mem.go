package amd64

import (
	"fmt"

	"github.com/tetratelabs/wazero/internal/engine/wazevo/backend"
	"github.com/tetratelabs/wazero/internal/engine/wazevo/backend/regalloc"
	"github.com/tetratelabs/wazero/internal/engine/wazevo/ssa"
)

var addendsMatchOpcodes = [...]ssa.Opcode{ssa.OpcodeUExtend, ssa.OpcodeSExtend, ssa.OpcodeIadd, ssa.OpcodeIconst, ssa.OpcodeIshl}

type addend struct {
	r     regalloc.VReg
	off   int64
	shift byte
}

func (a addend) String() string {
	return fmt.Sprintf("addend{r=%s, off=%d, shift=%d}", a.r, a.off, a.shift)
}

// lowerToAddressMode converts a pointer to an addressMode that can be used as an operand for load/store instructions.
func (m *machine) lowerToAddressMode(ptr ssa.Value, offsetBase uint32) (am *amode) {
	def := m.c.ValueDefinition(ptr)

	if offsetBase&0x80000000 != 0 {
		// Special casing the huge base offset whose MSB is set. In x64, the immediate is always
		// sign-extended, but our IR semantics requires the offset base is always unsigned.
		// Note that this should be extremely rare or even this shouldn't hit in the real application,
		// therefore we don't need to optimize this case in my opinion.

		a := m.lowerAddend(def)
		off64 := a.off + int64(offsetBase)
		offsetBaseReg := m.c.AllocateVReg(ssa.TypeI64)
		m.lowerIconst(offsetBaseReg, uint64(off64), true)
		if a.r != regalloc.VRegInvalid {
			return m.newAmodeRegRegShift(0, offsetBaseReg, a.r, a.shift)
		} else {
			return m.newAmodeImmReg(0, offsetBaseReg)
		}
	}

	if op := m.c.MatchInstrOneOf(def, addendsMatchOpcodes[:]); op == ssa.OpcodeIadd {
		add := def.Instr
		x, y := add.Arg2()
		xDef, yDef := m.c.ValueDefinition(x), m.c.ValueDefinition(y)
		ax := m.lowerAddend(xDef)
		ay := m.lowerAddend(yDef)
		add.MarkLowered()
		return m.lowerAddendsToAmode(ax, ay, offsetBase)
	} else {
		// If it is not an Iadd, then we lower the one addend.
		a := m.lowerAddend(def)
		// off is always 0 if r is valid.
		if a.r != regalloc.VRegInvalid {
			if a.shift != 0 {
				tmpReg := m.c.AllocateVReg(ssa.TypeI64)
				m.lowerIconst(tmpReg, 0, true)
				return m.newAmodeRegRegShift(offsetBase, tmpReg, a.r, a.shift)
			}
			return m.newAmodeImmReg(offsetBase, a.r)
		} else {
			off64 := a.off + int64(offsetBase)
			tmpReg := m.c.AllocateVReg(ssa.TypeI64)
			m.lowerIconst(tmpReg, uint64(off64), true)
			return m.newAmodeImmReg(0, tmpReg)
		}
	}
}

func (m *machine) lowerAddendsToAmode(x, y addend, offBase uint32) *amode {
	if x.r != regalloc.VRegInvalid && x.off != 0 || y.r != regalloc.VRegInvalid && y.off != 0 {
		panic("invalid input")
	}

	u64 := uint64(x.off+y.off) + uint64(offBase)
	if u64 != 0 {
		if _, ok := asImm32(u64, false); !ok {
			tmpReg := m.c.AllocateVReg(ssa.TypeI64)
			m.lowerIconst(tmpReg, u64, true)
			// Blank u64 as it has been already lowered.
			u64 = 0

			if x.r == regalloc.VRegInvalid {
				x.r = tmpReg
			} else if y.r == regalloc.VRegInvalid {
				y.r = tmpReg
			} else {
				// We already know that either rx or ry is invalid,
				// so we overwrite it with the temporary register.
				panic("BUG")
			}
		}
	}

	u32 := uint32(u64)
	switch {
	// We assume rx, ry are valid iff offx, offy are 0.
	case x.r != regalloc.VRegInvalid && y.r != regalloc.VRegInvalid:
		switch {
		case x.shift != 0 && y.shift != 0:
			// Cannot absorb two shifted registers, must lower one to a shift instruction.
			shifted := m.allocateInstr()
			shifted.asShiftR(shiftROpShiftLeft, newOperandImm32(uint32(x.shift)), x.r, true)
			m.insert(shifted)

			return m.newAmodeRegRegShift(u32, x.r, y.r, y.shift)
		case x.shift != 0 && y.shift == 0:
			// Swap base and index.
			x, y = y, x
			fallthrough
		default:
			return m.newAmodeRegRegShift(u32, x.r, y.r, y.shift)
		}
	case x.r == regalloc.VRegInvalid && y.r != regalloc.VRegInvalid:
		x, y = y, x
		fallthrough
	case x.r != regalloc.VRegInvalid && y.r == regalloc.VRegInvalid:
		if x.shift != 0 {
			zero := m.c.AllocateVReg(ssa.TypeI64)
			m.lowerIconst(zero, 0, true)
			return m.newAmodeRegRegShift(u32, zero, x.r, x.shift)
		}
		return m.newAmodeImmReg(u32, x.r)
	default: // Both are invalid: use the offset.
		tmpReg := m.c.AllocateVReg(ssa.TypeI64)
		m.lowerIconst(tmpReg, u64, true)
		return m.newAmodeImmReg(0, tmpReg)
	}
}

func (m *machine) lowerAddend(x backend.SSAValueDefinition) addend {
	if !x.IsFromInstr() {
		return addend{m.c.VRegOf(x.V), 0, 0}
	}
	// Ensure the addend is not referenced in multiple places; we will discard nested Iadds.
	op := m.c.MatchInstrOneOf(x, addendsMatchOpcodes[:])
	if op != ssa.OpcodeInvalid && op != ssa.OpcodeIadd {
		return m.lowerAddendFromInstr(x.Instr)
	}
	p := m.getOperand_Reg(x)
	return addend{p.reg(), 0, 0}
}

// lowerAddendFromInstr takes an instruction returns a Vreg and an offset that can be used in an address mode.
// The Vreg is regalloc.VRegInvalid if the addend cannot be lowered to a register.
// The offset is 0 if the addend can be lowered to a register.
func (m *machine) lowerAddendFromInstr(instr *ssa.Instruction) addend {
	instr.MarkLowered()
	switch op := instr.Opcode(); op {
	case ssa.OpcodeIconst:
		u64 := instr.ConstantVal()
		if instr.Return().Type().Bits() == 32 {
			return addend{regalloc.VRegInvalid, int64(int32(u64)), 0} // sign-extend.
		} else {
			return addend{regalloc.VRegInvalid, int64(u64), 0}
		}
	case ssa.OpcodeUExtend, ssa.OpcodeSExtend:
		input := instr.Arg()
		inputDef := m.c.ValueDefinition(input)
		if input.Type().Bits() != 32 {
			panic("BUG: invalid input type " + input.Type().String())
		}
		constInst := inputDef.IsFromInstr() && inputDef.Instr.Constant()
		switch {
		case constInst && op == ssa.OpcodeSExtend:
			return addend{regalloc.VRegInvalid, int64(uint32(inputDef.Instr.ConstantVal())), 0}
		case constInst && op == ssa.OpcodeUExtend:
			return addend{regalloc.VRegInvalid, int64(int32(inputDef.Instr.ConstantVal())), 0} // sign-extend!
		default:
			r := m.getOperand_Reg(inputDef)
			return addend{r.reg(), 0, 0}
		}
	case ssa.OpcodeIshl:
		// If the addend is a shift, we can only handle it if the shift amount is a constant.
		x, amount := instr.Arg2()
		amountDef := m.c.ValueDefinition(amount)
		if amountDef.IsFromInstr() && amountDef.Instr.Constant() && amountDef.Instr.ConstantVal() <= 3 {
			r := m.getOperand_Reg(m.c.ValueDefinition(x))
			return addend{r.reg(), 0, uint8(amountDef.Instr.ConstantVal())}
		}
		r := m.getOperand_Reg(m.c.ValueDefinition(x))
		return addend{r.reg(), 0, 0}
	}
	panic("BUG: invalid opcode")
}
