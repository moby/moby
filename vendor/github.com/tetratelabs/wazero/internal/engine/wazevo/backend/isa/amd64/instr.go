package amd64

import (
	"fmt"

	"github.com/tetratelabs/wazero/internal/engine/wazevo/backend"
	"github.com/tetratelabs/wazero/internal/engine/wazevo/backend/regalloc"
	"github.com/tetratelabs/wazero/internal/engine/wazevo/ssa"
)

type instruction struct {
	prev, next          *instruction
	op1, op2            operand
	u1, u2              uint64
	b1                  bool
	addedBeforeRegAlloc bool
	kind                instructionKind
}

// IsCall implements regalloc.Instr.
func (i *instruction) IsCall() bool { return i.kind == call }

// IsIndirectCall implements regalloc.Instr.
func (i *instruction) IsIndirectCall() bool { return i.kind == callIndirect }

// IsReturn implements regalloc.Instr.
func (i *instruction) IsReturn() bool { return i.kind == ret }

// String implements regalloc.Instr.
func (i *instruction) String() string {
	switch i.kind {
	case nop0:
		return "nop"
	case sourceOffsetInfo:
		return fmt.Sprintf("source_offset_info %d", i.u1)
	case ret:
		return "ret"
	case imm:
		if i.b1 {
			return fmt.Sprintf("movabsq $%d, %s", int64(i.u1), i.op2.format(true))
		} else {
			return fmt.Sprintf("movl $%d, %s", int32(i.u1), i.op2.format(false))
		}
	case aluRmiR:
		return fmt.Sprintf("%s %s, %s", aluRmiROpcode(i.u1), i.op1.format(i.b1), i.op2.format(i.b1))
	case movRR:
		if i.b1 {
			return fmt.Sprintf("movq %s, %s", i.op1.format(true), i.op2.format(true))
		} else {
			return fmt.Sprintf("movl %s, %s", i.op1.format(false), i.op2.format(false))
		}
	case xmmRmR:
		return fmt.Sprintf("%s %s, %s", sseOpcode(i.u1), i.op1.format(false), i.op2.format(false))
	case gprToXmm:
		return fmt.Sprintf("%s %s, %s", sseOpcode(i.u1), i.op1.format(i.b1), i.op2.format(i.b1))
	case xmmUnaryRmR:
		return fmt.Sprintf("%s %s, %s", sseOpcode(i.u1), i.op1.format(false), i.op2.format(false))
	case xmmUnaryRmRImm:
		return fmt.Sprintf("%s $%d, %s, %s", sseOpcode(i.u1), roundingMode(i.u2), i.op1.format(false), i.op2.format(false))
	case unaryRmR:
		var suffix string
		if i.b1 {
			suffix = "q"
		} else {
			suffix = "l"
		}
		return fmt.Sprintf("%s%s %s, %s", unaryRmROpcode(i.u1), suffix, i.op1.format(i.b1), i.op2.format(i.b1))
	case not:
		var op string
		if i.b1 {
			op = "notq"
		} else {
			op = "notl"
		}
		return fmt.Sprintf("%s %s", op, i.op1.format(i.b1))
	case neg:
		var op string
		if i.b1 {
			op = "negq"
		} else {
			op = "negl"
		}
		return fmt.Sprintf("%s %s", op, i.op1.format(i.b1))
	case div:
		var prefix string
		var op string
		if i.b1 {
			op = "divq"
		} else {
			op = "divl"
		}
		if i.u1 != 0 {
			prefix = "i"
		}
		return fmt.Sprintf("%s%s %s", prefix, op, i.op1.format(i.b1))
	case mulHi:
		signed, _64 := i.u1 != 0, i.b1
		var op string
		switch {
		case signed && _64:
			op = "imulq"
		case !signed && _64:
			op = "mulq"
		case signed && !_64:
			op = "imull"
		case !signed && !_64:
			op = "mull"
		}
		return fmt.Sprintf("%s %s", op, i.op1.format(i.b1))
	case signExtendData:
		var op string
		if i.b1 {
			op = "cqo"
		} else {
			op = "cdq"
		}
		return op
	case movzxRmR:
		return fmt.Sprintf("movzx.%s %s, %s", extMode(i.u1), i.op1.format(true), i.op2.format(true))
	case mov64MR:
		return fmt.Sprintf("movq %s, %s", i.op1.format(true), i.op2.format(true))
	case lea:
		return fmt.Sprintf("lea %s, %s", i.op1.format(true), i.op2.format(true))
	case movsxRmR:
		return fmt.Sprintf("movsx.%s %s, %s", extMode(i.u1), i.op1.format(true), i.op2.format(true))
	case movRM:
		var suffix string
		switch i.u1 {
		case 1:
			suffix = "b"
		case 2:
			suffix = "w"
		case 4:
			suffix = "l"
		case 8:
			suffix = "q"
		}
		return fmt.Sprintf("mov.%s %s, %s", suffix, i.op1.format(true), i.op2.format(true))
	case shiftR:
		var suffix string
		if i.b1 {
			suffix = "q"
		} else {
			suffix = "l"
		}
		return fmt.Sprintf("%s%s %s, %s", shiftROp(i.u1), suffix, i.op1.format(false), i.op2.format(i.b1))
	case xmmRmiReg:
		return fmt.Sprintf("%s %s, %s", sseOpcode(i.u1), i.op1.format(true), i.op2.format(true))
	case cmpRmiR:
		var op, suffix string
		if i.u1 != 0 {
			op = "cmp"
		} else {
			op = "test"
		}
		if i.b1 {
			suffix = "q"
		} else {
			suffix = "l"
		}
		if op == "test" && i.op1.kind == operandKindMem {
			// Print consistently with AT&T syntax.
			return fmt.Sprintf("%s%s %s, %s", op, suffix, i.op2.format(i.b1), i.op1.format(i.b1))
		}
		return fmt.Sprintf("%s%s %s, %s", op, suffix, i.op1.format(i.b1), i.op2.format(i.b1))
	case setcc:
		return fmt.Sprintf("set%s %s", cond(i.u1), i.op2.format(true))
	case cmove:
		var suffix string
		if i.b1 {
			suffix = "q"
		} else {
			suffix = "l"
		}
		return fmt.Sprintf("cmov%s%s %s, %s", cond(i.u1), suffix, i.op1.format(i.b1), i.op2.format(i.b1))
	case push64:
		return fmt.Sprintf("pushq %s", i.op1.format(true))
	case pop64:
		return fmt.Sprintf("popq %s", i.op1.format(true))
	case xmmMovRM:
		return fmt.Sprintf("%s %s, %s", sseOpcode(i.u1), i.op1.format(true), i.op2.format(true))
	case xmmLoadConst:
		panic("TODO")
	case xmmToGpr:
		return fmt.Sprintf("%s %s, %s", sseOpcode(i.u1), i.op1.format(i.b1), i.op2.format(i.b1))
	case cvtUint64ToFloatSeq:
		panic("TODO")
	case cvtFloatToSintSeq:
		panic("TODO")
	case cvtFloatToUintSeq:
		panic("TODO")
	case xmmMinMaxSeq:
		panic("TODO")
	case xmmCmpRmR:
		return fmt.Sprintf("%s %s, %s", sseOpcode(i.u1), i.op1.format(false), i.op2.format(false))
	case xmmRmRImm:
		op := sseOpcode(i.u1)
		r1, r2 := i.op1.format(op == sseOpcodePextrq || op == sseOpcodePinsrq),
			i.op2.format(op == sseOpcodePextrq || op == sseOpcodePinsrq)
		return fmt.Sprintf("%s $%d, %s, %s", op, i.u2, r1, r2)
	case jmp:
		return fmt.Sprintf("jmp %s", i.op1.format(true))
	case jmpIf:
		return fmt.Sprintf("j%s %s", cond(i.u1), i.op1.format(true))
	case jmpTableIsland:
		return fmt.Sprintf("jump_table_island: jmp_table_index=%d", i.u1)
	case exitSequence:
		return fmt.Sprintf("exit_sequence %s", i.op1.format(true))
	case ud2:
		return "ud2"
	case call:
		return fmt.Sprintf("call %s", ssa.FuncRef(i.u1))
	case callIndirect:
		return fmt.Sprintf("callq *%s", i.op1.format(true))
	case xchg:
		var suffix string
		switch i.u1 {
		case 1:
			suffix = "b"
		case 2:
			suffix = "w"
		case 4:
			suffix = "l"
		case 8:
			suffix = "q"
		}
		return fmt.Sprintf("xchg.%s %s, %s", suffix, i.op1.format(true), i.op2.format(true))
	case zeros:
		return fmt.Sprintf("xor %s, %s", i.op2.format(true), i.op2.format(true))
	case fcvtToSintSequence:
		execCtx, src, tmpGp, tmpGp2, tmpXmm, src64, dst64, sat := i.fcvtToSintSequenceData()
		return fmt.Sprintf(
			"fcvtToSintSequence execCtx=%s, src=%s, tmpGp=%s, tmpGp2=%s, tmpXmm=%s, src64=%v, dst64=%v, sat=%v",
			formatVRegSized(execCtx, true),
			formatVRegSized(src, true),
			formatVRegSized(tmpGp, true),
			formatVRegSized(tmpGp2, true),
			formatVRegSized(tmpXmm, true), src64, dst64, sat)
	case fcvtToUintSequence:
		execCtx, src, tmpGp, tmpGp2, tmpXmm, tmpXmm2, src64, dst64, sat := i.fcvtToUintSequenceData()
		return fmt.Sprintf(
			"fcvtToUintSequence execCtx=%s, src=%s, tmpGp=%s, tmpGp2=%s, tmpXmm=%s, tmpXmm2=%s, src64=%v, dst64=%v, sat=%v",
			formatVRegSized(execCtx, true),
			formatVRegSized(src, true),
			formatVRegSized(tmpGp, true),
			formatVRegSized(tmpGp2, true),
			formatVRegSized(tmpXmm, true),
			formatVRegSized(tmpXmm2, true), src64, dst64, sat)
	case idivRemSequence:
		execCtx, divisor, tmpGp, isDiv, signed, _64 := i.idivRemSequenceData()
		return fmt.Sprintf("idivRemSequence execCtx=%s, divisor=%s, tmpGp=%s, isDiv=%v, signed=%v, _64=%v",
			formatVRegSized(execCtx, true), formatVRegSized(divisor, _64), formatVRegSized(tmpGp, _64), isDiv, signed, _64)
	case defineUninitializedReg:
		return fmt.Sprintf("defineUninitializedReg %s", i.op2.format(true))
	case xmmCMov:
		return fmt.Sprintf("xmmcmov%s %s, %s", cond(i.u1), i.op1.format(true), i.op2.format(true))
	case blendvpd:
		return fmt.Sprintf("blendvpd %s, %s, %%xmm0", i.op1.format(false), i.op2.format(false))
	case mfence:
		return "mfence"
	case lockcmpxchg:
		var suffix string
		switch i.u1 {
		case 1:
			suffix = "b"
		case 2:
			suffix = "w"
		case 4:
			suffix = "l"
		case 8:
			suffix = "q"
		}
		return fmt.Sprintf("lock cmpxchg.%s %s, %s", suffix, i.op1.format(true), i.op2.format(true))
	case lockxadd:
		var suffix string
		switch i.u1 {
		case 1:
			suffix = "b"
		case 2:
			suffix = "w"
		case 4:
			suffix = "l"
		case 8:
			suffix = "q"
		}
		return fmt.Sprintf("lock xadd.%s %s, %s", suffix, i.op1.format(true), i.op2.format(true))

	case nopUseReg:
		return fmt.Sprintf("nop_use_reg %s", i.op1.format(true))

	default:
		panic(fmt.Sprintf("BUG: %d", int(i.kind)))
	}
}

// Defs implements regalloc.Instr.
func (i *instruction) Defs(regs *[]regalloc.VReg) []regalloc.VReg {
	*regs = (*regs)[:0]
	switch dk := defKinds[i.kind]; dk {
	case defKindNone:
	case defKindOp2:
		*regs = append(*regs, i.op2.reg())
	case defKindCall:
		_, _, retIntRealRegs, retFloatRealRegs, _ := backend.ABIInfoFromUint64(i.u2)
		for i := byte(0); i < retIntRealRegs; i++ {
			*regs = append(*regs, regInfo.RealRegToVReg[intArgResultRegs[i]])
		}
		for i := byte(0); i < retFloatRealRegs; i++ {
			*regs = append(*regs, regInfo.RealRegToVReg[floatArgResultRegs[i]])
		}
	case defKindDivRem:
		_, _, _, isDiv, _, _ := i.idivRemSequenceData()
		if isDiv {
			*regs = append(*regs, raxVReg)
		} else {
			*regs = append(*regs, rdxVReg)
		}
	default:
		panic(fmt.Sprintf("BUG: invalid defKind \"%s\" for %s", dk, i))
	}
	return *regs
}

// Uses implements regalloc.Instr.
func (i *instruction) Uses(regs *[]regalloc.VReg) []regalloc.VReg {
	*regs = (*regs)[:0]
	switch uk := useKinds[i.kind]; uk {
	case useKindNone:
	case useKindOp1Op2Reg, useKindOp1RegOp2:
		opAny, opReg := &i.op1, &i.op2
		if uk == useKindOp1RegOp2 {
			opAny, opReg = opReg, opAny
		}
		// The destination operand (op2) can be only reg,
		// the source operand (op1) can be imm32, reg or mem.
		switch opAny.kind {
		case operandKindReg:
			*regs = append(*regs, opAny.reg())
		case operandKindMem:
			opAny.addressMode().uses(regs)
		case operandKindImm32:
		default:
			panic(fmt.Sprintf("BUG: invalid operand: %s", i))
		}
		if opReg.kind != operandKindReg {
			panic(fmt.Sprintf("BUG: invalid operand: %s", i))
		}
		*regs = append(*regs, opReg.reg())
	case useKindOp1:
		op := i.op1
		switch op.kind {
		case operandKindReg:
			*regs = append(*regs, op.reg())
		case operandKindMem:
			op.addressMode().uses(regs)
		case operandKindImm32, operandKindLabel:
		default:
			panic(fmt.Sprintf("BUG: invalid operand: %s", i))
		}
	case useKindCallInd:
		op := i.op1
		switch op.kind {
		case operandKindReg:
			*regs = append(*regs, op.reg())
		case operandKindMem:
			op.addressMode().uses(regs)
		default:
			panic(fmt.Sprintf("BUG: invalid operand: %s", i))
		}
		fallthrough
	case useKindCall:
		argIntRealRegs, argFloatRealRegs, _, _, _ := backend.ABIInfoFromUint64(i.u2)
		for i := byte(0); i < argIntRealRegs; i++ {
			*regs = append(*regs, regInfo.RealRegToVReg[intArgResultRegs[i]])
		}
		for i := byte(0); i < argFloatRealRegs; i++ {
			*regs = append(*regs, regInfo.RealRegToVReg[floatArgResultRegs[i]])
		}
	case useKindFcvtToSintSequence:
		execCtx, src, tmpGp, tmpGp2, tmpXmm, _, _, _ := i.fcvtToSintSequenceData()
		*regs = append(*regs, execCtx, src, tmpGp, tmpGp2, tmpXmm)
	case useKindFcvtToUintSequence:
		execCtx, src, tmpGp, tmpGp2, tmpXmm, tmpXmm2, _, _, _ := i.fcvtToUintSequenceData()
		*regs = append(*regs, execCtx, src, tmpGp, tmpGp2, tmpXmm, tmpXmm2)
	case useKindDivRem:
		execCtx, divisor, tmpGp, _, _, _ := i.idivRemSequenceData()
		// idiv uses rax and rdx as implicit operands.
		*regs = append(*regs, raxVReg, rdxVReg, execCtx, divisor, tmpGp)
	case useKindBlendvpd:
		*regs = append(*regs, xmm0VReg)

		opAny, opReg := &i.op1, &i.op2
		switch opAny.kind {
		case operandKindReg:
			*regs = append(*regs, opAny.reg())
		case operandKindMem:
			opAny.addressMode().uses(regs)
		default:
			panic(fmt.Sprintf("BUG: invalid operand: %s", i))
		}
		if opReg.kind != operandKindReg {
			panic(fmt.Sprintf("BUG: invalid operand: %s", i))
		}
		*regs = append(*regs, opReg.reg())

	case useKindRaxOp1RegOp2:
		opReg, opAny := &i.op1, &i.op2
		*regs = append(*regs, raxVReg, opReg.reg())
		switch opAny.kind {
		case operandKindReg:
			*regs = append(*regs, opAny.reg())
		case operandKindMem:
			opAny.addressMode().uses(regs)
		default:
			panic(fmt.Sprintf("BUG: invalid operand: %s", i))
		}
		if opReg.kind != operandKindReg {
			panic(fmt.Sprintf("BUG: invalid operand: %s", i))
		}

	default:
		panic(fmt.Sprintf("BUG: invalid useKind %s for %s", uk, i))
	}
	return *regs
}

// AssignUse implements regalloc.Instr.
func (i *instruction) AssignUse(index int, v regalloc.VReg) {
	switch uk := useKinds[i.kind]; uk {
	case useKindNone:
	case useKindCallInd:
		if index != 0 {
			panic("BUG")
		}
		op := &i.op1
		switch op.kind {
		case operandKindReg:
			op.setReg(v)
		case operandKindMem:
			op.addressMode().assignUses(index, v)
		default:
			panic("BUG")
		}
	case useKindOp1Op2Reg, useKindOp1RegOp2:
		op, opMustBeReg := &i.op1, &i.op2
		if uk == useKindOp1RegOp2 {
			op, opMustBeReg = opMustBeReg, op
		}
		switch op.kind {
		case operandKindReg:
			if index == 0 {
				op.setReg(v)
			} else if index == 1 {
				opMustBeReg.setReg(v)
			} else {
				panic("BUG")
			}
		case operandKindMem:
			nregs := op.addressMode().nregs()
			if index < nregs {
				op.addressMode().assignUses(index, v)
			} else if index == nregs {
				opMustBeReg.setReg(v)
			} else {
				panic("BUG")
			}
		case operandKindImm32:
			if index == 0 {
				opMustBeReg.setReg(v)
			} else {
				panic("BUG")
			}
		default:
			panic(fmt.Sprintf("BUG: invalid operand pair: %s", i))
		}
	case useKindOp1:
		op := &i.op1
		switch op.kind {
		case operandKindReg:
			if index != 0 {
				panic("BUG")
			}
			op.setReg(v)
		case operandKindMem:
			op.addressMode().assignUses(index, v)
		default:
			panic(fmt.Sprintf("BUG: invalid operand: %s", i))
		}
	case useKindFcvtToSintSequence:
		switch index {
		case 0:
			i.op1.addressMode().base = v
		case 1:
			i.op1.addressMode().index = v
		case 2:
			i.op2.addressMode().base = v
		case 3:
			i.op2.addressMode().index = v
		case 4:
			i.u1 = uint64(v)
		default:
			panic("BUG")
		}
	case useKindFcvtToUintSequence:
		switch index {
		case 0:
			i.op1.addressMode().base = v
		case 1:
			i.op1.addressMode().index = v
		case 2:
			i.op2.addressMode().base = v
		case 3:
			i.op2.addressMode().index = v
		case 4:
			i.u1 = uint64(v)
		case 5:
			i.u2 = uint64(v)
		default:
			panic("BUG")
		}
	case useKindDivRem:
		switch index {
		case 0:
			if v != raxVReg {
				panic("BUG")
			}
		case 1:
			if v != rdxVReg {
				panic("BUG")
			}
		case 2:
			i.op1.setReg(v)
		case 3:
			i.op2.setReg(v)
		case 4:
			i.u1 = uint64(v)
		default:
			panic("BUG")
		}
	case useKindBlendvpd:
		op, opMustBeReg := &i.op1, &i.op2
		if index == 0 {
			if v.RealReg() != xmm0 {
				panic("BUG")
			}
		} else {
			switch op.kind {
			case operandKindReg:
				switch index {
				case 1:
					op.setReg(v)
				case 2:
					opMustBeReg.setReg(v)
				default:
					panic("BUG")
				}
			case operandKindMem:
				nregs := op.addressMode().nregs()
				index--
				if index < nregs {
					op.addressMode().assignUses(index, v)
				} else if index == nregs {
					opMustBeReg.setReg(v)
				} else {
					panic("BUG")
				}
			default:
				panic(fmt.Sprintf("BUG: invalid operand pair: %s", i))
			}
		}

	case useKindRaxOp1RegOp2:
		switch index {
		case 0:
			if v.RealReg() != rax {
				panic("BUG")
			}
		case 1:
			i.op1.setReg(v)
		default:
			op := &i.op2
			switch op.kind {
			case operandKindReg:
				switch index {
				case 1:
					op.setReg(v)
				case 2:
					op.setReg(v)
				default:
					panic("BUG")
				}
			case operandKindMem:
				nregs := op.addressMode().nregs()
				index -= 2
				if index < nregs {
					op.addressMode().assignUses(index, v)
				} else if index == nregs {
					op.setReg(v)
				} else {
					panic("BUG")
				}
			default:
				panic(fmt.Sprintf("BUG: invalid operand pair: %s", i))
			}
		}
	default:
		panic(fmt.Sprintf("BUG: invalid useKind %s for %s", uk, i))
	}
}

// AssignDef implements regalloc.Instr.
func (i *instruction) AssignDef(reg regalloc.VReg) {
	switch dk := defKinds[i.kind]; dk {
	case defKindNone:
	case defKindOp2:
		i.op2.setReg(reg)
	default:
		panic(fmt.Sprintf("BUG: invalid defKind \"%s\" for %s", dk, i))
	}
}

// IsCopy implements regalloc.Instr.
func (i *instruction) IsCopy() bool {
	k := i.kind
	if k == movRR {
		return true
	}
	if k == xmmUnaryRmR {
		if i.op1.kind == operandKindReg {
			sse := sseOpcode(i.u1)
			return sse == sseOpcodeMovss || sse == sseOpcodeMovsd || sse == sseOpcodeMovdqu
		}
	}
	return false
}

func resetInstruction(i *instruction) {
	*i = instruction{}
}

func (i *instruction) asNop0WithLabel(label label) *instruction { //nolint
	i.kind = nop0
	i.u1 = uint64(label)
	return i
}

func (i *instruction) nop0Label() label {
	return label(i.u1)
}

type instructionKind byte

const (
	nop0 instructionKind = iota + 1

	// Integer arithmetic/bit-twiddling: (add sub and or xor mul, etc.) (32 64) (reg addr imm) reg
	aluRmiR

	// Instructions on GPR that only read src and defines dst (dst is not modified): bsr, etc.
	unaryRmR

	// Bitwise not
	not

	// Integer negation
	neg

	// Integer quotient and remainder: (div idiv) $rax $rdx (reg addr)
	div

	// The high bits (RDX) of a (un)signed multiply: RDX:RAX := RAX * rhs.
	mulHi

	// Do a sign-extend based on the sign of the value in rax into rdx: (cwd cdq cqo)
	// or al into ah: (cbw)
	signExtendData

	// Constant materialization: (imm32 imm64) reg.
	// Either: movl $imm32, %reg32 or movabsq $imm64, %reg64.
	imm

	// GPR to GPR move: mov (64 32) reg reg.
	movRR

	// movzxRmR is zero-extended loads or move (R to R), except for 64 bits: movz (bl bq wl wq lq) addr reg.
	// Note that the lq variant doesn't really exist since the default zero-extend rule makes it
	// unnecessary. For that case we emit the equivalent "movl AM, reg32".
	movzxRmR

	// mov64MR is a plain 64-bit integer load, since movzxRmR can't represent that.
	mov64MR

	// Loads the memory address of addr into dst.
	lea

	// Sign-extended loads and moves: movs (bl bq wl wq lq) addr reg.
	movsxRmR

	// Integer stores: mov (b w l q) reg addr.
	movRM

	// Arithmetic shifts: (shl shr sar) (b w l q) imm reg.
	shiftR

	// Arithmetic SIMD shifts.
	xmmRmiReg

	// Integer comparisons/tests: cmp or test (b w l q) (reg addr imm) reg.
	cmpRmiR

	// Materializes the requested condition code in the destination reg.
	setcc

	// Integer conditional move.
	// Overwrites the destination register.
	cmove

	// pushq (reg addr imm)
	push64

	// popq reg
	pop64

	// XMM (scalar or vector) binary op: (add sub and or xor mul adc? sbb?) (32 64) (reg addr) reg
	xmmRmR

	// XMM (scalar or vector) unary op: mov between XMM registers (32 64) (reg addr) reg.
	//
	// This differs from xmmRmR in that the dst register of xmmUnaryRmR is not used in the
	// computation of the instruction dst value and so does not have to be a previously valid
	// value. This is characteristic of mov instructions.
	xmmUnaryRmR

	// XMM (scalar or vector) unary op with immediate: roundss, roundsd, etc.
	//
	// This differs from XMM_RM_R_IMM in that the dst register of
	// XmmUnaryRmRImm is not used in the computation of the instruction dst
	// value and so does not have to be a previously valid value.
	xmmUnaryRmRImm

	// XMM (scalar or vector) unary op (from xmm to mem): stores, movd, movq
	xmmMovRM

	// XMM (vector) unary op (to move a constant value into an xmm register): movups
	xmmLoadConst

	// XMM (scalar) unary op (from xmm to integer reg): movd, movq, cvtts{s,d}2si
	xmmToGpr

	// XMM (scalar) unary op (from integer to float reg): movd, movq, cvtsi2s{s,d}
	gprToXmm

	// Converts an unsigned int64 to a float32/float64.
	cvtUint64ToFloatSeq

	// Converts a scalar xmm to a signed int32/int64.
	cvtFloatToSintSeq

	// Converts a scalar xmm to an unsigned int32/int64.
	cvtFloatToUintSeq

	// A sequence to compute min/max with the proper NaN semantics for xmm registers.
	xmmMinMaxSeq

	// Float comparisons/tests: cmp (b w l q) (reg addr imm) reg.
	xmmCmpRmR

	// A binary XMM instruction with an 8-bit immediate: e.g. cmp (ps pd) imm (reg addr) reg
	xmmRmRImm

	// Direct call: call simm32.
	// Note that the offset is the relative to the *current RIP*, which points to the first byte of the next instruction.
	call

	// Indirect call: callq (reg mem).
	callIndirect

	// Return.
	ret

	// Jump: jmp (reg, mem, imm32 or label)
	jmp

	// Jump conditionally: jcond cond label.
	jmpIf

	// jmpTableIsland is to emit the jump table.
	jmpTableIsland

	// exitSequence exits the execution and go back to the Go world.
	exitSequence

	// An instruction that will always trigger the illegal instruction exception.
	ud2

	// xchg is described in https://www.felixcloutier.com/x86/xchg.
	// This instruction uses two operands, where one of them can be a memory address, and swaps their values.
	// If the dst is a memory address, the execution is atomic.
	xchg

	// lockcmpxchg is the cmpxchg instruction https://www.felixcloutier.com/x86/cmpxchg with a lock prefix.
	lockcmpxchg

	// zeros puts zeros into the destination register. This is implemented as xor reg, reg for
	// either integer or XMM registers. The reason why we have this instruction instead of using aluRmiR
	// is that it requires the already-defined registers. From reg alloc's perspective, this defines
	// the destination register and takes no inputs.
	zeros

	// sourceOffsetInfo is a dummy instruction to emit source offset info.
	// The existence of this instruction does not affect the execution.
	sourceOffsetInfo

	// defineUninitializedReg is a no-op instruction that defines a register without a defining instruction.
	defineUninitializedReg

	// fcvtToSintSequence is a sequence of instructions to convert a float to a signed integer.
	fcvtToSintSequence

	// fcvtToUintSequence is a sequence of instructions to convert a float to an unsigned integer.
	fcvtToUintSequence

	// xmmCMov is a conditional move instruction for XMM registers. Lowered after register allocation.
	xmmCMov

	// idivRemSequence is a sequence of instructions to compute both the quotient and remainder of a division.
	idivRemSequence

	// blendvpd is https://www.felixcloutier.com/x86/blendvpd.
	blendvpd

	// mfence is https://www.felixcloutier.com/x86/mfence
	mfence

	// lockxadd is xadd https://www.felixcloutier.com/x86/xadd with a lock prefix.
	lockxadd

	// nopUseReg is a meta instruction that uses one register and does nothing.
	nopUseReg

	instrMax
)

func (i *instruction) asMFence() *instruction {
	i.kind = mfence
	return i
}

func (i *instruction) asNopUseReg(r regalloc.VReg) *instruction {
	i.kind = nopUseReg
	i.op1 = newOperandReg(r)
	return i
}

func (i *instruction) asIdivRemSequence(execCtx, divisor, tmpGp regalloc.VReg, isDiv, signed, _64 bool) *instruction {
	i.kind = idivRemSequence
	i.op1 = newOperandReg(execCtx)
	i.op2 = newOperandReg(divisor)
	i.u1 = uint64(tmpGp)
	if isDiv {
		i.u2 |= 1
	}
	if signed {
		i.u2 |= 2
	}
	if _64 {
		i.u2 |= 4
	}
	return i
}

func (i *instruction) idivRemSequenceData() (
	execCtx, divisor, tmpGp regalloc.VReg, isDiv, signed, _64 bool,
) {
	if i.kind != idivRemSequence {
		panic("BUG")
	}
	return i.op1.reg(), i.op2.reg(), regalloc.VReg(i.u1), i.u2&1 != 0, i.u2&2 != 0, i.u2&4 != 0
}

func (i *instruction) asXmmCMov(cc cond, x operand, rd regalloc.VReg, size byte) *instruction {
	i.kind = xmmCMov
	i.op1 = x
	i.op2 = newOperandReg(rd)
	i.u1 = uint64(cc)
	i.u2 = uint64(size)
	return i
}

func (i *instruction) asDefineUninitializedReg(r regalloc.VReg) *instruction {
	i.kind = defineUninitializedReg
	i.op2 = newOperandReg(r)
	return i
}

func (m *machine) allocateFcvtToUintSequence(
	execCtx, src, tmpGp, tmpGp2, tmpXmm, tmpXmm2 regalloc.VReg,
	src64, dst64, sat bool,
) *instruction {
	i := m.allocateInstr()
	i.kind = fcvtToUintSequence
	op1a := m.amodePool.Allocate()
	op2a := m.amodePool.Allocate()
	i.op1 = newOperandMem(op1a)
	i.op2 = newOperandMem(op2a)
	if src64 {
		op1a.imm32 = 1
	} else {
		op1a.imm32 = 0
	}
	if dst64 {
		op1a.imm32 |= 2
	}
	if sat {
		op1a.imm32 |= 4
	}

	op1a.base = execCtx
	op1a.index = src
	op2a.base = tmpGp
	op2a.index = tmpGp2
	i.u1 = uint64(tmpXmm)
	i.u2 = uint64(tmpXmm2)
	return i
}

func (i *instruction) fcvtToUintSequenceData() (
	execCtx, src, tmpGp, tmpGp2, tmpXmm, tmpXmm2 regalloc.VReg, src64, dst64, sat bool,
) {
	if i.kind != fcvtToUintSequence {
		panic("BUG")
	}
	op1a := i.op1.addressMode()
	op2a := i.op2.addressMode()
	return op1a.base, op1a.index, op2a.base, op2a.index, regalloc.VReg(i.u1), regalloc.VReg(i.u2),
		op1a.imm32&1 != 0, op1a.imm32&2 != 0, op1a.imm32&4 != 0
}

func (m *machine) allocateFcvtToSintSequence(
	execCtx, src, tmpGp, tmpGp2, tmpXmm regalloc.VReg,
	src64, dst64, sat bool,
) *instruction {
	i := m.allocateInstr()
	i.kind = fcvtToSintSequence
	op1a := m.amodePool.Allocate()
	op2a := m.amodePool.Allocate()
	i.op1 = newOperandMem(op1a)
	i.op2 = newOperandMem(op2a)
	op1a.base = execCtx
	op1a.index = src
	op2a.base = tmpGp
	op2a.index = tmpGp2
	i.u1 = uint64(tmpXmm)
	if src64 {
		i.u2 = 1
	} else {
		i.u2 = 0
	}
	if dst64 {
		i.u2 |= 2
	}
	if sat {
		i.u2 |= 4
	}
	return i
}

func (i *instruction) fcvtToSintSequenceData() (
	execCtx, src, tmpGp, tmpGp2, tmpXmm regalloc.VReg, src64, dst64, sat bool,
) {
	if i.kind != fcvtToSintSequence {
		panic("BUG")
	}
	op1a := i.op1.addressMode()
	op2a := i.op2.addressMode()
	return op1a.base, op1a.index, op2a.base, op2a.index, regalloc.VReg(i.u1),
		i.u2&1 != 0, i.u2&2 != 0, i.u2&4 != 0
}

func (k instructionKind) String() string {
	switch k {
	case nop0:
		return "nop"
	case ret:
		return "ret"
	case imm:
		return "imm"
	case aluRmiR:
		return "aluRmiR"
	case movRR:
		return "movRR"
	case xmmRmR:
		return "xmmRmR"
	case gprToXmm:
		return "gprToXmm"
	case xmmUnaryRmR:
		return "xmmUnaryRmR"
	case xmmUnaryRmRImm:
		return "xmmUnaryRmRImm"
	case unaryRmR:
		return "unaryRmR"
	case not:
		return "not"
	case neg:
		return "neg"
	case div:
		return "div"
	case mulHi:
		return "mulHi"
	case signExtendData:
		return "signExtendData"
	case movzxRmR:
		return "movzxRmR"
	case mov64MR:
		return "mov64MR"
	case lea:
		return "lea"
	case movsxRmR:
		return "movsxRmR"
	case movRM:
		return "movRM"
	case shiftR:
		return "shiftR"
	case xmmRmiReg:
		return "xmmRmiReg"
	case cmpRmiR:
		return "cmpRmiR"
	case setcc:
		return "setcc"
	case cmove:
		return "cmove"
	case push64:
		return "push64"
	case pop64:
		return "pop64"
	case xmmMovRM:
		return "xmmMovRM"
	case xmmLoadConst:
		return "xmmLoadConst"
	case xmmToGpr:
		return "xmmToGpr"
	case cvtUint64ToFloatSeq:
		return "cvtUint64ToFloatSeq"
	case cvtFloatToSintSeq:
		return "cvtFloatToSintSeq"
	case cvtFloatToUintSeq:
		return "cvtFloatToUintSeq"
	case xmmMinMaxSeq:
		return "xmmMinMaxSeq"
	case xmmCmpRmR:
		return "xmmCmpRmR"
	case xmmRmRImm:
		return "xmmRmRImm"
	case jmpIf:
		return "jmpIf"
	case jmp:
		return "jmp"
	case jmpTableIsland:
		return "jmpTableIsland"
	case exitSequence:
		return "exit_sequence"
	case ud2:
		return "ud2"
	case xchg:
		return "xchg"
	case zeros:
		return "zeros"
	case fcvtToSintSequence:
		return "fcvtToSintSequence"
	case fcvtToUintSequence:
		return "fcvtToUintSequence"
	case xmmCMov:
		return "xmmCMov"
	case idivRemSequence:
		return "idivRemSequence"
	case mfence:
		return "mfence"
	case lockcmpxchg:
		return "lockcmpxchg"
	case lockxadd:
		return "lockxadd"
	default:
		panic("BUG")
	}
}

type aluRmiROpcode byte

const (
	aluRmiROpcodeAdd aluRmiROpcode = iota + 1
	aluRmiROpcodeSub
	aluRmiROpcodeAnd
	aluRmiROpcodeOr
	aluRmiROpcodeXor
	aluRmiROpcodeMul
)

func (a aluRmiROpcode) String() string {
	switch a {
	case aluRmiROpcodeAdd:
		return "add"
	case aluRmiROpcodeSub:
		return "sub"
	case aluRmiROpcodeAnd:
		return "and"
	case aluRmiROpcodeOr:
		return "or"
	case aluRmiROpcodeXor:
		return "xor"
	case aluRmiROpcodeMul:
		return "imul"
	default:
		panic("BUG")
	}
}

func (i *instruction) asJmpIf(cond cond, target operand) *instruction {
	i.kind = jmpIf
	i.u1 = uint64(cond)
	i.op1 = target
	return i
}

// asJmpTableSequence is used to emit the jump table.
// targetSliceIndex is the index of the target slice in machine.jmpTableTargets.
func (i *instruction) asJmpTableSequence(targetSliceIndex int, targetCount int) *instruction {
	i.kind = jmpTableIsland
	i.u1 = uint64(targetSliceIndex)
	i.u2 = uint64(targetCount)
	return i
}

func (i *instruction) asJmp(target operand) *instruction {
	i.kind = jmp
	i.op1 = target
	return i
}

func (i *instruction) jmpLabel() label {
	switch i.kind {
	case jmp, jmpIf, lea, xmmUnaryRmR:
		return i.op1.label()
	default:
		panic("BUG")
	}
}

func (i *instruction) asLEA(target operand, rd regalloc.VReg) *instruction {
	i.kind = lea
	i.op1 = target
	i.op2 = newOperandReg(rd)
	return i
}

func (i *instruction) asCall(ref ssa.FuncRef, abi *backend.FunctionABI) *instruction {
	i.kind = call
	i.u1 = uint64(ref)
	if abi != nil {
		i.u2 = abi.ABIInfoAsUint64()
	}
	return i
}

func (i *instruction) asCallIndirect(ptr operand, abi *backend.FunctionABI) *instruction {
	if ptr.kind != operandKindReg && ptr.kind != operandKindMem {
		panic("BUG")
	}
	i.kind = callIndirect
	i.op1 = ptr
	if abi != nil {
		i.u2 = abi.ABIInfoAsUint64()
	}
	return i
}

func (i *instruction) asRet() *instruction {
	i.kind = ret
	return i
}

func (i *instruction) asImm(dst regalloc.VReg, value uint64, _64 bool) *instruction {
	i.kind = imm
	i.op2 = newOperandReg(dst)
	i.u1 = value
	i.b1 = _64
	return i
}

func (i *instruction) asAluRmiR(op aluRmiROpcode, rm operand, rd regalloc.VReg, _64 bool) *instruction {
	if rm.kind != operandKindReg && rm.kind != operandKindMem && rm.kind != operandKindImm32 {
		panic("BUG")
	}
	i.kind = aluRmiR
	i.op1 = rm
	i.op2 = newOperandReg(rd)
	i.u1 = uint64(op)
	i.b1 = _64
	return i
}

func (i *instruction) asZeros(dst regalloc.VReg) *instruction {
	i.kind = zeros
	i.op2 = newOperandReg(dst)
	return i
}

func (i *instruction) asBlendvpd(rm operand, rd regalloc.VReg) *instruction {
	if rm.kind != operandKindReg && rm.kind != operandKindMem {
		panic("BUG")
	}
	i.kind = blendvpd
	i.op1 = rm
	i.op2 = newOperandReg(rd)
	return i
}

func (i *instruction) asXmmRmR(op sseOpcode, rm operand, rd regalloc.VReg) *instruction {
	if rm.kind != operandKindReg && rm.kind != operandKindMem {
		panic("BUG")
	}
	i.kind = xmmRmR
	i.op1 = rm
	i.op2 = newOperandReg(rd)
	i.u1 = uint64(op)
	return i
}

func (i *instruction) asXmmRmRImm(op sseOpcode, imm uint8, rm operand, rd regalloc.VReg) *instruction {
	if rm.kind != operandKindReg && rm.kind != operandKindMem {
		panic("BUG")
	}
	i.kind = xmmRmRImm
	i.op1 = rm
	i.op2 = newOperandReg(rd)
	i.u1 = uint64(op)
	i.u2 = uint64(imm)
	return i
}

func (i *instruction) asGprToXmm(op sseOpcode, rm operand, rd regalloc.VReg, _64 bool) *instruction {
	if rm.kind != operandKindReg && rm.kind != operandKindMem {
		panic("BUG")
	}
	i.kind = gprToXmm
	i.op1 = rm
	i.op2 = newOperandReg(rd)
	i.u1 = uint64(op)
	i.b1 = _64
	return i
}

func (i *instruction) asEmitSourceOffsetInfo(l ssa.SourceOffset) *instruction {
	i.kind = sourceOffsetInfo
	i.u1 = uint64(l)
	return i
}

func (i *instruction) sourceOffsetInfo() ssa.SourceOffset {
	return ssa.SourceOffset(i.u1)
}

func (i *instruction) asXmmToGpr(op sseOpcode, rm, rd regalloc.VReg, _64 bool) *instruction {
	i.kind = xmmToGpr
	i.op1 = newOperandReg(rm)
	i.op2 = newOperandReg(rd)
	i.u1 = uint64(op)
	i.b1 = _64
	return i
}

func (i *instruction) asMovRM(rm regalloc.VReg, rd operand, size byte) *instruction {
	if rd.kind != operandKindMem {
		panic("BUG")
	}
	i.kind = movRM
	i.op1 = newOperandReg(rm)
	i.op2 = rd
	i.u1 = uint64(size)
	return i
}

func (i *instruction) asMovsxRmR(ext extMode, src operand, rd regalloc.VReg) *instruction {
	if src.kind != operandKindReg && src.kind != operandKindMem {
		panic("BUG")
	}
	i.kind = movsxRmR
	i.op1 = src
	i.op2 = newOperandReg(rd)
	i.u1 = uint64(ext)
	return i
}

func (i *instruction) asMovzxRmR(ext extMode, src operand, rd regalloc.VReg) *instruction {
	if src.kind != operandKindReg && src.kind != operandKindMem {
		panic("BUG")
	}
	i.kind = movzxRmR
	i.op1 = src
	i.op2 = newOperandReg(rd)
	i.u1 = uint64(ext)
	return i
}

func (i *instruction) asSignExtendData(_64 bool) *instruction {
	i.kind = signExtendData
	i.b1 = _64
	return i
}

func (i *instruction) asUD2() *instruction {
	i.kind = ud2
	return i
}

func (i *instruction) asDiv(rn operand, signed bool, _64 bool) *instruction {
	i.kind = div
	i.op1 = rn
	i.b1 = _64
	if signed {
		i.u1 = 1
	}
	return i
}

func (i *instruction) asMov64MR(rm operand, rd regalloc.VReg) *instruction {
	if rm.kind != operandKindMem {
		panic("BUG")
	}
	i.kind = mov64MR
	i.op1 = rm
	i.op2 = newOperandReg(rd)
	return i
}

func (i *instruction) asMovRR(rm, rd regalloc.VReg, _64 bool) *instruction {
	i.kind = movRR
	i.op1 = newOperandReg(rm)
	i.op2 = newOperandReg(rd)
	i.b1 = _64
	return i
}

func (i *instruction) asNot(rm operand, _64 bool) *instruction {
	if rm.kind != operandKindReg && rm.kind != operandKindMem {
		panic("BUG")
	}
	i.kind = not
	i.op1 = rm
	i.b1 = _64
	return i
}

func (i *instruction) asNeg(rm operand, _64 bool) *instruction {
	if rm.kind != operandKindReg && rm.kind != operandKindMem {
		panic("BUG")
	}
	i.kind = neg
	i.op1 = rm
	i.b1 = _64
	return i
}

func (i *instruction) asMulHi(rm operand, signed, _64 bool) *instruction {
	if rm.kind != operandKindReg && (rm.kind != operandKindMem) {
		panic("BUG")
	}
	i.kind = mulHi
	i.op1 = rm
	i.b1 = _64
	if signed {
		i.u1 = 1
	}
	return i
}

func (i *instruction) asUnaryRmR(op unaryRmROpcode, rm operand, rd regalloc.VReg, _64 bool) *instruction {
	if rm.kind != operandKindReg && rm.kind != operandKindMem {
		panic("BUG")
	}
	i.kind = unaryRmR
	i.op1 = rm
	i.op2 = newOperandReg(rd)
	i.u1 = uint64(op)
	i.b1 = _64
	return i
}

func (i *instruction) asShiftR(op shiftROp, amount operand, rd regalloc.VReg, _64 bool) *instruction {
	if amount.kind != operandKindReg && amount.kind != operandKindImm32 {
		panic("BUG")
	}
	i.kind = shiftR
	i.op1 = amount
	i.op2 = newOperandReg(rd)
	i.u1 = uint64(op)
	i.b1 = _64
	return i
}

func (i *instruction) asXmmRmiReg(op sseOpcode, rm operand, rd regalloc.VReg) *instruction {
	if rm.kind != operandKindReg && rm.kind != operandKindImm32 && rm.kind != operandKindMem {
		panic("BUG")
	}
	i.kind = xmmRmiReg
	i.op1 = rm
	i.op2 = newOperandReg(rd)
	i.u1 = uint64(op)
	return i
}

func (i *instruction) asCmpRmiR(cmp bool, rm operand, rn regalloc.VReg, _64 bool) *instruction {
	if rm.kind != operandKindReg && rm.kind != operandKindImm32 && rm.kind != operandKindMem {
		panic("BUG")
	}
	i.kind = cmpRmiR
	i.op1 = rm
	i.op2 = newOperandReg(rn)
	if cmp {
		i.u1 = 1
	}
	i.b1 = _64
	return i
}

func (i *instruction) asSetcc(c cond, rd regalloc.VReg) *instruction {
	i.kind = setcc
	i.op2 = newOperandReg(rd)
	i.u1 = uint64(c)
	return i
}

func (i *instruction) asCmove(c cond, rm operand, rd regalloc.VReg, _64 bool) *instruction {
	i.kind = cmove
	i.op1 = rm
	i.op2 = newOperandReg(rd)
	i.u1 = uint64(c)
	i.b1 = _64
	return i
}

func (m *machine) allocateExitSeq(execCtx regalloc.VReg) *instruction {
	i := m.allocateInstr()
	i.kind = exitSequence
	i.op1 = newOperandReg(execCtx)
	// Allocate the address mode that will be used in encoding the exit sequence.
	i.op2 = newOperandMem(m.amodePool.Allocate())
	return i
}

func (i *instruction) asXmmUnaryRmR(op sseOpcode, rm operand, rd regalloc.VReg) *instruction {
	if rm.kind != operandKindReg && rm.kind != operandKindMem {
		panic("BUG")
	}
	i.kind = xmmUnaryRmR
	i.op1 = rm
	i.op2 = newOperandReg(rd)
	i.u1 = uint64(op)
	return i
}

func (i *instruction) asXmmUnaryRmRImm(op sseOpcode, imm byte, rm operand, rd regalloc.VReg) *instruction {
	if rm.kind != operandKindReg && rm.kind != operandKindMem {
		panic("BUG")
	}
	i.kind = xmmUnaryRmRImm
	i.op1 = rm
	i.op2 = newOperandReg(rd)
	i.u1 = uint64(op)
	i.u2 = uint64(imm)
	return i
}

func (i *instruction) asXmmCmpRmR(op sseOpcode, rm operand, rd regalloc.VReg) *instruction {
	if rm.kind != operandKindReg && rm.kind != operandKindMem {
		panic("BUG")
	}
	i.kind = xmmCmpRmR
	i.op1 = rm
	i.op2 = newOperandReg(rd)
	i.u1 = uint64(op)
	return i
}

func (i *instruction) asXmmMovRM(op sseOpcode, rm regalloc.VReg, rd operand) *instruction {
	if rd.kind != operandKindMem {
		panic("BUG")
	}
	i.kind = xmmMovRM
	i.op1 = newOperandReg(rm)
	i.op2 = rd
	i.u1 = uint64(op)
	return i
}

func (i *instruction) asPop64(rm regalloc.VReg) *instruction {
	i.kind = pop64
	i.op1 = newOperandReg(rm)
	return i
}

func (i *instruction) asPush64(op operand) *instruction {
	if op.kind != operandKindReg && op.kind != operandKindMem && op.kind != operandKindImm32 {
		panic("BUG")
	}
	i.kind = push64
	i.op1 = op
	return i
}

func (i *instruction) asXCHG(rm regalloc.VReg, rd operand, size byte) *instruction {
	i.kind = xchg
	i.op1 = newOperandReg(rm)
	i.op2 = rd
	i.u1 = uint64(size)
	return i
}

func (i *instruction) asLockCmpXCHG(rm regalloc.VReg, rd *amode, size byte) *instruction {
	i.kind = lockcmpxchg
	i.op1 = newOperandReg(rm)
	i.op2 = newOperandMem(rd)
	i.u1 = uint64(size)
	return i
}

func (i *instruction) asLockXAdd(rm regalloc.VReg, rd *amode, size byte) *instruction {
	i.kind = lockxadd
	i.op1 = newOperandReg(rm)
	i.op2 = newOperandMem(rd)
	i.u1 = uint64(size)
	return i
}

type unaryRmROpcode byte

const (
	unaryRmROpcodeBsr unaryRmROpcode = iota
	unaryRmROpcodeBsf
	unaryRmROpcodeLzcnt
	unaryRmROpcodeTzcnt
	unaryRmROpcodePopcnt
)

func (u unaryRmROpcode) String() string {
	switch u {
	case unaryRmROpcodeBsr:
		return "bsr"
	case unaryRmROpcodeBsf:
		return "bsf"
	case unaryRmROpcodeLzcnt:
		return "lzcnt"
	case unaryRmROpcodeTzcnt:
		return "tzcnt"
	case unaryRmROpcodePopcnt:
		return "popcnt"
	default:
		panic("BUG")
	}
}

type shiftROp byte

const (
	shiftROpRotateLeft           shiftROp = 0
	shiftROpRotateRight          shiftROp = 1
	shiftROpShiftLeft            shiftROp = 4
	shiftROpShiftRightLogical    shiftROp = 5
	shiftROpShiftRightArithmetic shiftROp = 7
)

func (s shiftROp) String() string {
	switch s {
	case shiftROpRotateLeft:
		return "rol"
	case shiftROpRotateRight:
		return "ror"
	case shiftROpShiftLeft:
		return "shl"
	case shiftROpShiftRightLogical:
		return "shr"
	case shiftROpShiftRightArithmetic:
		return "sar"
	default:
		panic("BUG")
	}
}

type sseOpcode byte

const (
	sseOpcodeInvalid sseOpcode = iota
	sseOpcodeAddps
	sseOpcodeAddpd
	sseOpcodeAddss
	sseOpcodeAddsd
	sseOpcodeAndps
	sseOpcodeAndpd
	sseOpcodeAndnps
	sseOpcodeAndnpd
	sseOpcodeBlendvps
	sseOpcodeBlendvpd
	sseOpcodeComiss
	sseOpcodeComisd
	sseOpcodeCmpps
	sseOpcodeCmppd
	sseOpcodeCmpss
	sseOpcodeCmpsd
	sseOpcodeCvtdq2ps
	sseOpcodeCvtdq2pd
	sseOpcodeCvtsd2ss
	sseOpcodeCvtsd2si
	sseOpcodeCvtsi2ss
	sseOpcodeCvtsi2sd
	sseOpcodeCvtss2si
	sseOpcodeCvtss2sd
	sseOpcodeCvttps2dq
	sseOpcodeCvttss2si
	sseOpcodeCvttsd2si
	sseOpcodeDivps
	sseOpcodeDivpd
	sseOpcodeDivss
	sseOpcodeDivsd
	sseOpcodeInsertps
	sseOpcodeMaxps
	sseOpcodeMaxpd
	sseOpcodeMaxss
	sseOpcodeMaxsd
	sseOpcodeMinps
	sseOpcodeMinpd
	sseOpcodeMinss
	sseOpcodeMinsd
	sseOpcodeMovaps
	sseOpcodeMovapd
	sseOpcodeMovd
	sseOpcodeMovdqa
	sseOpcodeMovdqu
	sseOpcodeMovlhps
	sseOpcodeMovmskps
	sseOpcodeMovmskpd
	sseOpcodeMovq
	sseOpcodeMovss
	sseOpcodeMovsd
	sseOpcodeMovups
	sseOpcodeMovupd
	sseOpcodeMulps
	sseOpcodeMulpd
	sseOpcodeMulss
	sseOpcodeMulsd
	sseOpcodeOrps
	sseOpcodeOrpd
	sseOpcodePabsb
	sseOpcodePabsw
	sseOpcodePabsd
	sseOpcodePackssdw
	sseOpcodePacksswb
	sseOpcodePackusdw
	sseOpcodePackuswb
	sseOpcodePaddb
	sseOpcodePaddd
	sseOpcodePaddq
	sseOpcodePaddw
	sseOpcodePaddsb
	sseOpcodePaddsw
	sseOpcodePaddusb
	sseOpcodePaddusw
	sseOpcodePalignr
	sseOpcodePand
	sseOpcodePandn
	sseOpcodePavgb
	sseOpcodePavgw
	sseOpcodePcmpeqb
	sseOpcodePcmpeqw
	sseOpcodePcmpeqd
	sseOpcodePcmpeqq
	sseOpcodePcmpgtb
	sseOpcodePcmpgtw
	sseOpcodePcmpgtd
	sseOpcodePcmpgtq
	sseOpcodePextrb
	sseOpcodePextrw
	sseOpcodePextrd
	sseOpcodePextrq
	sseOpcodePinsrb
	sseOpcodePinsrw
	sseOpcodePinsrd
	sseOpcodePinsrq
	sseOpcodePmaddwd
	sseOpcodePmaxsb
	sseOpcodePmaxsw
	sseOpcodePmaxsd
	sseOpcodePmaxub
	sseOpcodePmaxuw
	sseOpcodePmaxud
	sseOpcodePminsb
	sseOpcodePminsw
	sseOpcodePminsd
	sseOpcodePminub
	sseOpcodePminuw
	sseOpcodePminud
	sseOpcodePmovmskb
	sseOpcodePmovsxbd
	sseOpcodePmovsxbw
	sseOpcodePmovsxbq
	sseOpcodePmovsxwd
	sseOpcodePmovsxwq
	sseOpcodePmovsxdq
	sseOpcodePmovzxbd
	sseOpcodePmovzxbw
	sseOpcodePmovzxbq
	sseOpcodePmovzxwd
	sseOpcodePmovzxwq
	sseOpcodePmovzxdq
	sseOpcodePmulld
	sseOpcodePmullw
	sseOpcodePmuludq
	sseOpcodePor
	sseOpcodePshufb
	sseOpcodePshufd
	sseOpcodePsllw
	sseOpcodePslld
	sseOpcodePsllq
	sseOpcodePsraw
	sseOpcodePsrad
	sseOpcodePsrlw
	sseOpcodePsrld
	sseOpcodePsrlq
	sseOpcodePsubb
	sseOpcodePsubd
	sseOpcodePsubq
	sseOpcodePsubw
	sseOpcodePsubsb
	sseOpcodePsubsw
	sseOpcodePsubusb
	sseOpcodePsubusw
	sseOpcodePtest
	sseOpcodePunpckhbw
	sseOpcodePunpcklbw
	sseOpcodePxor
	sseOpcodeRcpss
	sseOpcodeRoundps
	sseOpcodeRoundpd
	sseOpcodeRoundss
	sseOpcodeRoundsd
	sseOpcodeRsqrtss
	sseOpcodeSqrtps
	sseOpcodeSqrtpd
	sseOpcodeSqrtss
	sseOpcodeSqrtsd
	sseOpcodeSubps
	sseOpcodeSubpd
	sseOpcodeSubss
	sseOpcodeSubsd
	sseOpcodeUcomiss
	sseOpcodeUcomisd
	sseOpcodeXorps
	sseOpcodeXorpd
	sseOpcodePmulhrsw
	sseOpcodeUnpcklps
	sseOpcodeCvtps2pd
	sseOpcodeCvtpd2ps
	sseOpcodeCvttpd2dq
	sseOpcodeShufps
	sseOpcodePmaddubsw
)

func (s sseOpcode) String() string {
	switch s {
	case sseOpcodeInvalid:
		return "invalid"
	case sseOpcodeAddps:
		return "addps"
	case sseOpcodeAddpd:
		return "addpd"
	case sseOpcodeAddss:
		return "addss"
	case sseOpcodeAddsd:
		return "addsd"
	case sseOpcodeAndps:
		return "andps"
	case sseOpcodeAndpd:
		return "andpd"
	case sseOpcodeAndnps:
		return "andnps"
	case sseOpcodeAndnpd:
		return "andnpd"
	case sseOpcodeBlendvps:
		return "blendvps"
	case sseOpcodeBlendvpd:
		return "blendvpd"
	case sseOpcodeComiss:
		return "comiss"
	case sseOpcodeComisd:
		return "comisd"
	case sseOpcodeCmpps:
		return "cmpps"
	case sseOpcodeCmppd:
		return "cmppd"
	case sseOpcodeCmpss:
		return "cmpss"
	case sseOpcodeCmpsd:
		return "cmpsd"
	case sseOpcodeCvtdq2ps:
		return "cvtdq2ps"
	case sseOpcodeCvtdq2pd:
		return "cvtdq2pd"
	case sseOpcodeCvtsd2ss:
		return "cvtsd2ss"
	case sseOpcodeCvtsd2si:
		return "cvtsd2si"
	case sseOpcodeCvtsi2ss:
		return "cvtsi2ss"
	case sseOpcodeCvtsi2sd:
		return "cvtsi2sd"
	case sseOpcodeCvtss2si:
		return "cvtss2si"
	case sseOpcodeCvtss2sd:
		return "cvtss2sd"
	case sseOpcodeCvttps2dq:
		return "cvttps2dq"
	case sseOpcodeCvttss2si:
		return "cvttss2si"
	case sseOpcodeCvttsd2si:
		return "cvttsd2si"
	case sseOpcodeDivps:
		return "divps"
	case sseOpcodeDivpd:
		return "divpd"
	case sseOpcodeDivss:
		return "divss"
	case sseOpcodeDivsd:
		return "divsd"
	case sseOpcodeInsertps:
		return "insertps"
	case sseOpcodeMaxps:
		return "maxps"
	case sseOpcodeMaxpd:
		return "maxpd"
	case sseOpcodeMaxss:
		return "maxss"
	case sseOpcodeMaxsd:
		return "maxsd"
	case sseOpcodeMinps:
		return "minps"
	case sseOpcodeMinpd:
		return "minpd"
	case sseOpcodeMinss:
		return "minss"
	case sseOpcodeMinsd:
		return "minsd"
	case sseOpcodeMovaps:
		return "movaps"
	case sseOpcodeMovapd:
		return "movapd"
	case sseOpcodeMovd:
		return "movd"
	case sseOpcodeMovdqa:
		return "movdqa"
	case sseOpcodeMovdqu:
		return "movdqu"
	case sseOpcodeMovlhps:
		return "movlhps"
	case sseOpcodeMovmskps:
		return "movmskps"
	case sseOpcodeMovmskpd:
		return "movmskpd"
	case sseOpcodeMovq:
		return "movq"
	case sseOpcodeMovss:
		return "movss"
	case sseOpcodeMovsd:
		return "movsd"
	case sseOpcodeMovups:
		return "movups"
	case sseOpcodeMovupd:
		return "movupd"
	case sseOpcodeMulps:
		return "mulps"
	case sseOpcodeMulpd:
		return "mulpd"
	case sseOpcodeMulss:
		return "mulss"
	case sseOpcodeMulsd:
		return "mulsd"
	case sseOpcodeOrps:
		return "orps"
	case sseOpcodeOrpd:
		return "orpd"
	case sseOpcodePabsb:
		return "pabsb"
	case sseOpcodePabsw:
		return "pabsw"
	case sseOpcodePabsd:
		return "pabsd"
	case sseOpcodePackssdw:
		return "packssdw"
	case sseOpcodePacksswb:
		return "packsswb"
	case sseOpcodePackusdw:
		return "packusdw"
	case sseOpcodePackuswb:
		return "packuswb"
	case sseOpcodePaddb:
		return "paddb"
	case sseOpcodePaddd:
		return "paddd"
	case sseOpcodePaddq:
		return "paddq"
	case sseOpcodePaddw:
		return "paddw"
	case sseOpcodePaddsb:
		return "paddsb"
	case sseOpcodePaddsw:
		return "paddsw"
	case sseOpcodePaddusb:
		return "paddusb"
	case sseOpcodePaddusw:
		return "paddusw"
	case sseOpcodePalignr:
		return "palignr"
	case sseOpcodePand:
		return "pand"
	case sseOpcodePandn:
		return "pandn"
	case sseOpcodePavgb:
		return "pavgb"
	case sseOpcodePavgw:
		return "pavgw"
	case sseOpcodePcmpeqb:
		return "pcmpeqb"
	case sseOpcodePcmpeqw:
		return "pcmpeqw"
	case sseOpcodePcmpeqd:
		return "pcmpeqd"
	case sseOpcodePcmpeqq:
		return "pcmpeqq"
	case sseOpcodePcmpgtb:
		return "pcmpgtb"
	case sseOpcodePcmpgtw:
		return "pcmpgtw"
	case sseOpcodePcmpgtd:
		return "pcmpgtd"
	case sseOpcodePcmpgtq:
		return "pcmpgtq"
	case sseOpcodePextrb:
		return "pextrb"
	case sseOpcodePextrw:
		return "pextrw"
	case sseOpcodePextrd:
		return "pextrd"
	case sseOpcodePextrq:
		return "pextrq"
	case sseOpcodePinsrb:
		return "pinsrb"
	case sseOpcodePinsrw:
		return "pinsrw"
	case sseOpcodePinsrd:
		return "pinsrd"
	case sseOpcodePinsrq:
		return "pinsrq"
	case sseOpcodePmaddwd:
		return "pmaddwd"
	case sseOpcodePmaxsb:
		return "pmaxsb"
	case sseOpcodePmaxsw:
		return "pmaxsw"
	case sseOpcodePmaxsd:
		return "pmaxsd"
	case sseOpcodePmaxub:
		return "pmaxub"
	case sseOpcodePmaxuw:
		return "pmaxuw"
	case sseOpcodePmaxud:
		return "pmaxud"
	case sseOpcodePminsb:
		return "pminsb"
	case sseOpcodePminsw:
		return "pminsw"
	case sseOpcodePminsd:
		return "pminsd"
	case sseOpcodePminub:
		return "pminub"
	case sseOpcodePminuw:
		return "pminuw"
	case sseOpcodePminud:
		return "pminud"
	case sseOpcodePmovmskb:
		return "pmovmskb"
	case sseOpcodePmovsxbd:
		return "pmovsxbd"
	case sseOpcodePmovsxbw:
		return "pmovsxbw"
	case sseOpcodePmovsxbq:
		return "pmovsxbq"
	case sseOpcodePmovsxwd:
		return "pmovsxwd"
	case sseOpcodePmovsxwq:
		return "pmovsxwq"
	case sseOpcodePmovsxdq:
		return "pmovsxdq"
	case sseOpcodePmovzxbd:
		return "pmovzxbd"
	case sseOpcodePmovzxbw:
		return "pmovzxbw"
	case sseOpcodePmovzxbq:
		return "pmovzxbq"
	case sseOpcodePmovzxwd:
		return "pmovzxwd"
	case sseOpcodePmovzxwq:
		return "pmovzxwq"
	case sseOpcodePmovzxdq:
		return "pmovzxdq"
	case sseOpcodePmulld:
		return "pmulld"
	case sseOpcodePmullw:
		return "pmullw"
	case sseOpcodePmuludq:
		return "pmuludq"
	case sseOpcodePor:
		return "por"
	case sseOpcodePshufb:
		return "pshufb"
	case sseOpcodePshufd:
		return "pshufd"
	case sseOpcodePsllw:
		return "psllw"
	case sseOpcodePslld:
		return "pslld"
	case sseOpcodePsllq:
		return "psllq"
	case sseOpcodePsraw:
		return "psraw"
	case sseOpcodePsrad:
		return "psrad"
	case sseOpcodePsrlw:
		return "psrlw"
	case sseOpcodePsrld:
		return "psrld"
	case sseOpcodePsrlq:
		return "psrlq"
	case sseOpcodePsubb:
		return "psubb"
	case sseOpcodePsubd:
		return "psubd"
	case sseOpcodePsubq:
		return "psubq"
	case sseOpcodePsubw:
		return "psubw"
	case sseOpcodePsubsb:
		return "psubsb"
	case sseOpcodePsubsw:
		return "psubsw"
	case sseOpcodePsubusb:
		return "psubusb"
	case sseOpcodePsubusw:
		return "psubusw"
	case sseOpcodePtest:
		return "ptest"
	case sseOpcodePunpckhbw:
		return "punpckhbw"
	case sseOpcodePunpcklbw:
		return "punpcklbw"
	case sseOpcodePxor:
		return "pxor"
	case sseOpcodeRcpss:
		return "rcpss"
	case sseOpcodeRoundps:
		return "roundps"
	case sseOpcodeRoundpd:
		return "roundpd"
	case sseOpcodeRoundss:
		return "roundss"
	case sseOpcodeRoundsd:
		return "roundsd"
	case sseOpcodeRsqrtss:
		return "rsqrtss"
	case sseOpcodeSqrtps:
		return "sqrtps"
	case sseOpcodeSqrtpd:
		return "sqrtpd"
	case sseOpcodeSqrtss:
		return "sqrtss"
	case sseOpcodeSqrtsd:
		return "sqrtsd"
	case sseOpcodeSubps:
		return "subps"
	case sseOpcodeSubpd:
		return "subpd"
	case sseOpcodeSubss:
		return "subss"
	case sseOpcodeSubsd:
		return "subsd"
	case sseOpcodeUcomiss:
		return "ucomiss"
	case sseOpcodeUcomisd:
		return "ucomisd"
	case sseOpcodeXorps:
		return "xorps"
	case sseOpcodeXorpd:
		return "xorpd"
	case sseOpcodePmulhrsw:
		return "pmulhrsw"
	case sseOpcodeUnpcklps:
		return "unpcklps"
	case sseOpcodeCvtps2pd:
		return "cvtps2pd"
	case sseOpcodeCvtpd2ps:
		return "cvtpd2ps"
	case sseOpcodeCvttpd2dq:
		return "cvttpd2dq"
	case sseOpcodeShufps:
		return "shufps"
	case sseOpcodePmaddubsw:
		return "pmaddubsw"
	default:
		panic("BUG")
	}
}

type roundingMode uint8

const (
	roundingModeNearest roundingMode = iota
	roundingModeDown
	roundingModeUp
	roundingModeZero
)

func (r roundingMode) String() string {
	switch r {
	case roundingModeNearest:
		return "nearest"
	case roundingModeDown:
		return "down"
	case roundingModeUp:
		return "up"
	case roundingModeZero:
		return "zero"
	default:
		panic("BUG")
	}
}

// cmpPred is the immediate value for a comparison operation in xmmRmRImm.
type cmpPred uint8

const (
	// cmpPredEQ_OQ is Equal (ordered, non-signaling)
	cmpPredEQ_OQ cmpPred = iota
	// cmpPredLT_OS is Less-than (ordered, signaling)
	cmpPredLT_OS
	// cmpPredLE_OS is Less-than-or-equal (ordered, signaling)
	cmpPredLE_OS
	// cmpPredUNORD_Q is Unordered (non-signaling)
	cmpPredUNORD_Q
	// cmpPredNEQ_UQ is Not-equal (unordered, non-signaling)
	cmpPredNEQ_UQ
	// cmpPredNLT_US is Not-less-than (unordered, signaling)
	cmpPredNLT_US
	// cmpPredNLE_US is Not-less-than-or-equal (unordered, signaling)
	cmpPredNLE_US
	// cmpPredORD_Q is Ordered (non-signaling)
	cmpPredORD_Q
	// cmpPredEQ_UQ is Equal (unordered, non-signaling)
	cmpPredEQ_UQ
	// cmpPredNGE_US is Not-greater-than-or-equal (unordered, signaling)
	cmpPredNGE_US
	// cmpPredNGT_US is Not-greater-than (unordered, signaling)
	cmpPredNGT_US
	// cmpPredFALSE_OQ is False (ordered, non-signaling)
	cmpPredFALSE_OQ
	// cmpPredNEQ_OQ is Not-equal (ordered, non-signaling)
	cmpPredNEQ_OQ
	// cmpPredGE_OS is Greater-than-or-equal (ordered, signaling)
	cmpPredGE_OS
	// cmpPredGT_OS is Greater-than (ordered, signaling)
	cmpPredGT_OS
	// cmpPredTRUE_UQ is True (unordered, non-signaling)
	cmpPredTRUE_UQ
	// Equal (ordered, signaling)
	cmpPredEQ_OS
	// Less-than (ordered, nonsignaling)
	cmpPredLT_OQ
	// Less-than-or-equal (ordered, nonsignaling)
	cmpPredLE_OQ
	// Unordered (signaling)
	cmpPredUNORD_S
	// Not-equal (unordered, signaling)
	cmpPredNEQ_US
	// Not-less-than (unordered, nonsignaling)
	cmpPredNLT_UQ
	// Not-less-than-or-equal (unordered, nonsignaling)
	cmpPredNLE_UQ
	// Ordered (signaling)
	cmpPredORD_S
	// Equal (unordered, signaling)
	cmpPredEQ_US
	// Not-greater-than-or-equal (unordered, non-signaling)
	cmpPredNGE_UQ
	// Not-greater-than (unordered, nonsignaling)
	cmpPredNGT_UQ
	// False (ordered, signaling)
	cmpPredFALSE_OS
	// Not-equal (ordered, signaling)
	cmpPredNEQ_OS
	// Greater-than-or-equal (ordered, nonsignaling)
	cmpPredGE_OQ
	// Greater-than (ordered, nonsignaling)
	cmpPredGT_OQ
	// True (unordered, signaling)
	cmpPredTRUE_US
)

func (r cmpPred) String() string {
	switch r {
	case cmpPredEQ_OQ:
		return "eq_oq"
	case cmpPredLT_OS:
		return "lt_os"
	case cmpPredLE_OS:
		return "le_os"
	case cmpPredUNORD_Q:
		return "unord_q"
	case cmpPredNEQ_UQ:
		return "neq_uq"
	case cmpPredNLT_US:
		return "nlt_us"
	case cmpPredNLE_US:
		return "nle_us"
	case cmpPredORD_Q:
		return "ord_q"
	case cmpPredEQ_UQ:
		return "eq_uq"
	case cmpPredNGE_US:
		return "nge_us"
	case cmpPredNGT_US:
		return "ngt_us"
	case cmpPredFALSE_OQ:
		return "false_oq"
	case cmpPredNEQ_OQ:
		return "neq_oq"
	case cmpPredGE_OS:
		return "ge_os"
	case cmpPredGT_OS:
		return "gt_os"
	case cmpPredTRUE_UQ:
		return "true_uq"
	case cmpPredEQ_OS:
		return "eq_os"
	case cmpPredLT_OQ:
		return "lt_oq"
	case cmpPredLE_OQ:
		return "le_oq"
	case cmpPredUNORD_S:
		return "unord_s"
	case cmpPredNEQ_US:
		return "neq_us"
	case cmpPredNLT_UQ:
		return "nlt_uq"
	case cmpPredNLE_UQ:
		return "nle_uq"
	case cmpPredORD_S:
		return "ord_s"
	case cmpPredEQ_US:
		return "eq_us"
	case cmpPredNGE_UQ:
		return "nge_uq"
	case cmpPredNGT_UQ:
		return "ngt_uq"
	case cmpPredFALSE_OS:
		return "false_os"
	case cmpPredNEQ_OS:
		return "neq_os"
	case cmpPredGE_OQ:
		return "ge_oq"
	case cmpPredGT_OQ:
		return "gt_oq"
	case cmpPredTRUE_US:
		return "true_us"
	default:
		panic("BUG")
	}
}

func linkInstr(prev, next *instruction) *instruction {
	prev.next = next
	next.prev = prev
	return next
}

type defKind byte

const (
	defKindNone defKind = iota + 1
	defKindOp2
	defKindCall
	defKindDivRem
)

var defKinds = [instrMax]defKind{
	nop0:                   defKindNone,
	ret:                    defKindNone,
	movRR:                  defKindOp2,
	movRM:                  defKindNone,
	xmmMovRM:               defKindNone,
	aluRmiR:                defKindNone,
	shiftR:                 defKindNone,
	imm:                    defKindOp2,
	unaryRmR:               defKindOp2,
	xmmRmiReg:              defKindNone,
	xmmUnaryRmR:            defKindOp2,
	xmmUnaryRmRImm:         defKindOp2,
	xmmCmpRmR:              defKindNone,
	xmmRmR:                 defKindNone,
	xmmRmRImm:              defKindNone,
	mov64MR:                defKindOp2,
	movsxRmR:               defKindOp2,
	movzxRmR:               defKindOp2,
	gprToXmm:               defKindOp2,
	xmmToGpr:               defKindOp2,
	cmove:                  defKindNone,
	call:                   defKindCall,
	callIndirect:           defKindCall,
	ud2:                    defKindNone,
	jmp:                    defKindNone,
	jmpIf:                  defKindNone,
	jmpTableIsland:         defKindNone,
	cmpRmiR:                defKindNone,
	exitSequence:           defKindNone,
	lea:                    defKindOp2,
	setcc:                  defKindOp2,
	zeros:                  defKindOp2,
	sourceOffsetInfo:       defKindNone,
	fcvtToSintSequence:     defKindNone,
	defineUninitializedReg: defKindOp2,
	fcvtToUintSequence:     defKindNone,
	xmmCMov:                defKindOp2,
	idivRemSequence:        defKindDivRem,
	blendvpd:               defKindNone,
	mfence:                 defKindNone,
	xchg:                   defKindNone,
	lockcmpxchg:            defKindNone,
	lockxadd:               defKindNone,
	neg:                    defKindNone,
	nopUseReg:              defKindNone,
}

// String implements fmt.Stringer.
func (d defKind) String() string {
	switch d {
	case defKindNone:
		return "none"
	case defKindOp2:
		return "op2"
	case defKindCall:
		return "call"
	case defKindDivRem:
		return "divrem"
	default:
		return "invalid"
	}
}

type useKind byte

const (
	useKindNone useKind = iota + 1
	useKindOp1
	// useKindOp1Op2Reg is Op1 can be any operand, Op2 must be a register.
	useKindOp1Op2Reg
	// useKindOp1RegOp2 is Op1 must be a register, Op2 can be any operand.
	useKindOp1RegOp2
	// useKindRaxOp1RegOp2 is Op1 must be a register, Op2 can be any operand, and RAX is used.
	useKindRaxOp1RegOp2
	useKindDivRem
	useKindBlendvpd
	useKindCall
	useKindCallInd
	useKindFcvtToSintSequence
	useKindFcvtToUintSequence
)

var useKinds = [instrMax]useKind{
	nop0:                   useKindNone,
	ret:                    useKindNone,
	movRR:                  useKindOp1,
	movRM:                  useKindOp1RegOp2,
	xmmMovRM:               useKindOp1RegOp2,
	cmove:                  useKindOp1Op2Reg,
	aluRmiR:                useKindOp1Op2Reg,
	shiftR:                 useKindOp1Op2Reg,
	imm:                    useKindNone,
	unaryRmR:               useKindOp1,
	xmmRmiReg:              useKindOp1Op2Reg,
	xmmUnaryRmR:            useKindOp1,
	xmmUnaryRmRImm:         useKindOp1,
	xmmCmpRmR:              useKindOp1Op2Reg,
	xmmRmR:                 useKindOp1Op2Reg,
	xmmRmRImm:              useKindOp1Op2Reg,
	mov64MR:                useKindOp1,
	movzxRmR:               useKindOp1,
	movsxRmR:               useKindOp1,
	gprToXmm:               useKindOp1,
	xmmToGpr:               useKindOp1,
	call:                   useKindCall,
	callIndirect:           useKindCallInd,
	ud2:                    useKindNone,
	jmpIf:                  useKindOp1,
	jmp:                    useKindOp1,
	cmpRmiR:                useKindOp1Op2Reg,
	exitSequence:           useKindOp1,
	lea:                    useKindOp1,
	jmpTableIsland:         useKindNone,
	setcc:                  useKindNone,
	zeros:                  useKindNone,
	sourceOffsetInfo:       useKindNone,
	fcvtToSintSequence:     useKindFcvtToSintSequence,
	defineUninitializedReg: useKindNone,
	fcvtToUintSequence:     useKindFcvtToUintSequence,
	xmmCMov:                useKindOp1,
	idivRemSequence:        useKindDivRem,
	blendvpd:               useKindBlendvpd,
	mfence:                 useKindNone,
	xchg:                   useKindOp1RegOp2,
	lockcmpxchg:            useKindRaxOp1RegOp2,
	lockxadd:               useKindOp1RegOp2,
	neg:                    useKindOp1,
	nopUseReg:              useKindOp1,
}

func (u useKind) String() string {
	switch u {
	case useKindNone:
		return "none"
	case useKindOp1:
		return "op1"
	case useKindOp1Op2Reg:
		return "op1op2Reg"
	case useKindOp1RegOp2:
		return "op1RegOp2"
	case useKindCall:
		return "call"
	case useKindCallInd:
		return "callInd"
	default:
		return "invalid"
	}
}
