package arm64

// Files prefixed as lower_instr** do the instruction selection, meaning that lowering SSA level instructions
// into machine specific instructions.
//
// Importantly, what the lower** functions does includes tree-matching; find the pattern from the given instruction tree,
// and merge the multiple instructions if possible. It can be considered as "N:1" instruction selection.

import (
	"fmt"
	"math"

	"github.com/tetratelabs/wazero/internal/engine/wazevo/backend/regalloc"
	"github.com/tetratelabs/wazero/internal/engine/wazevo/ssa"
	"github.com/tetratelabs/wazero/internal/engine/wazevo/wazevoapi"
)

// LowerSingleBranch implements backend.Machine.
func (m *machine) LowerSingleBranch(br *ssa.Instruction) {
	switch br.Opcode() {
	case ssa.OpcodeJump:
		_, _, targetBlkID := br.BranchData()
		if br.IsFallthroughJump() {
			return
		}
		b := m.allocateInstr()
		targetBlk := m.compiler.SSABuilder().BasicBlock(targetBlkID)
		if targetBlk.ReturnBlock() {
			b.asRet()
		} else {
			b.asBr(ssaBlockLabel(targetBlk))
		}
		m.insert(b)
	case ssa.OpcodeBrTable:
		m.lowerBrTable(br)
	default:
		panic("BUG: unexpected branch opcode" + br.Opcode().String())
	}
}

func (m *machine) lowerBrTable(i *ssa.Instruction) {
	index, targetBlockIDs := i.BrTableData()
	targetBlockCount := len(targetBlockIDs.View())
	indexOperand := m.getOperand_NR(m.compiler.ValueDefinition(index), extModeNone)

	// Firstly, we have to do the bounds check of the index, and
	// set it to the default target (sitting at the end of the list) if it's out of bounds.

	// mov  maxIndexReg #maximum_index
	// subs wzr, index, maxIndexReg
	// csel adjustedIndex, maxIndexReg, index, hs ;; if index is higher or equal than maxIndexReg.
	maxIndexReg := m.compiler.AllocateVReg(ssa.TypeI32)
	m.lowerConstantI32(maxIndexReg, int32(targetBlockCount-1))
	subs := m.allocateInstr()
	subs.asALU(aluOpSubS, xzrVReg, indexOperand, operandNR(maxIndexReg), false)
	m.insert(subs)
	csel := m.allocateInstr()
	adjustedIndex := m.compiler.AllocateVReg(ssa.TypeI32)
	csel.asCSel(adjustedIndex, operandNR(maxIndexReg), indexOperand, hs, false)
	m.insert(csel)

	brSequence := m.allocateInstr()

	tableIndex := m.addJmpTableTarget(targetBlockIDs)
	brSequence.asBrTableSequence(adjustedIndex, tableIndex, targetBlockCount)
	m.insert(brSequence)
}

// LowerConditionalBranch implements backend.Machine.
func (m *machine) LowerConditionalBranch(b *ssa.Instruction) {
	cval, args, targetBlkID := b.BranchData()
	if len(args) > 0 {
		panic(fmt.Sprintf(
			"conditional branch shouldn't have args; likely a bug in critical edge splitting: from %s to %s",
			m.currentLabelPos.sb,
			targetBlkID,
		))
	}

	targetBlk := m.compiler.SSABuilder().BasicBlock(targetBlkID)
	target := ssaBlockLabel(targetBlk)
	cvalDef := m.compiler.ValueDefinition(cval)

	switch {
	case m.compiler.MatchInstr(cvalDef, ssa.OpcodeIcmp): // This case, we can use the ALU flag set by SUBS instruction.
		cvalInstr := cvalDef.Instr
		x, y, c := cvalInstr.IcmpData()
		cc, signed := condFlagFromSSAIntegerCmpCond(c), c.Signed()
		if b.Opcode() == ssa.OpcodeBrz {
			cc = cc.invert()
		}

		if !m.tryLowerBandToFlag(x, y) {
			m.lowerIcmpToFlag(x, y, signed)
		}
		cbr := m.allocateInstr()
		cbr.asCondBr(cc.asCond(), target, false /* ignored */)
		m.insert(cbr)
		cvalDef.Instr.MarkLowered()
	case m.compiler.MatchInstr(cvalDef, ssa.OpcodeFcmp): // This case we can use the Fpu flag directly.
		cvalInstr := cvalDef.Instr
		x, y, c := cvalInstr.FcmpData()
		cc := condFlagFromSSAFloatCmpCond(c)
		if b.Opcode() == ssa.OpcodeBrz {
			cc = cc.invert()
		}
		m.lowerFcmpToFlag(x, y)
		cbr := m.allocateInstr()
		cbr.asCondBr(cc.asCond(), target, false /* ignored */)
		m.insert(cbr)
		cvalDef.Instr.MarkLowered()
	default:
		rn := m.getOperand_NR(cvalDef, extModeNone)
		var c cond
		if b.Opcode() == ssa.OpcodeBrz {
			c = registerAsRegZeroCond(rn.nr())
		} else {
			c = registerAsRegNotZeroCond(rn.nr())
		}
		cbr := m.allocateInstr()
		cbr.asCondBr(c, target, false)
		m.insert(cbr)
	}
}

func (m *machine) tryLowerBandToFlag(x, y ssa.Value) (ok bool) {
	xx := m.compiler.ValueDefinition(x)
	yy := m.compiler.ValueDefinition(y)
	if xx.IsFromInstr() && xx.Instr.Constant() && xx.Instr.ConstantVal() == 0 {
		if m.compiler.MatchInstr(yy, ssa.OpcodeBand) {
			bandInstr := yy.Instr
			m.lowerBitwiseAluOp(bandInstr, aluOpAnds, true)
			ok = true
			bandInstr.MarkLowered()
			return
		}
	}

	if yy.IsFromInstr() && yy.Instr.Constant() && yy.Instr.ConstantVal() == 0 {
		if m.compiler.MatchInstr(xx, ssa.OpcodeBand) {
			bandInstr := xx.Instr
			m.lowerBitwiseAluOp(bandInstr, aluOpAnds, true)
			ok = true
			bandInstr.MarkLowered()
			return
		}
	}
	return
}

// LowerInstr implements backend.Machine.
func (m *machine) LowerInstr(instr *ssa.Instruction) {
	if l := instr.SourceOffset(); l.Valid() {
		info := m.allocateInstr().asEmitSourceOffsetInfo(l)
		m.insert(info)
	}

	switch op := instr.Opcode(); op {
	case ssa.OpcodeBrz, ssa.OpcodeBrnz, ssa.OpcodeJump, ssa.OpcodeBrTable:
		panic("BUG: branching instructions are handled by LowerBranches")
	case ssa.OpcodeReturn:
		panic("BUG: return must be handled by backend.Compiler")
	case ssa.OpcodeIadd, ssa.OpcodeIsub:
		m.lowerSubOrAdd(instr, op == ssa.OpcodeIadd)
	case ssa.OpcodeFadd, ssa.OpcodeFsub, ssa.OpcodeFmul, ssa.OpcodeFdiv, ssa.OpcodeFmax, ssa.OpcodeFmin:
		m.lowerFpuBinOp(instr)
	case ssa.OpcodeIconst, ssa.OpcodeF32const, ssa.OpcodeF64const: // Constant instructions are inlined.
	case ssa.OpcodeExitWithCode:
		execCtx, code := instr.ExitWithCodeData()
		m.lowerExitWithCode(m.compiler.VRegOf(execCtx), code)
	case ssa.OpcodeExitIfTrueWithCode:
		execCtx, c, code := instr.ExitIfTrueWithCodeData()
		m.lowerExitIfTrueWithCode(m.compiler.VRegOf(execCtx), c, code)
	case ssa.OpcodeStore, ssa.OpcodeIstore8, ssa.OpcodeIstore16, ssa.OpcodeIstore32:
		m.lowerStore(instr)
	case ssa.OpcodeLoad:
		dst := instr.Return()
		ptr, offset, typ := instr.LoadData()
		m.lowerLoad(ptr, offset, typ, dst)
	case ssa.OpcodeVZeroExtLoad:
		dst := instr.Return()
		ptr, offset, typ := instr.VZeroExtLoadData()
		m.lowerLoad(ptr, offset, typ, dst)
	case ssa.OpcodeUload8, ssa.OpcodeUload16, ssa.OpcodeUload32, ssa.OpcodeSload8, ssa.OpcodeSload16, ssa.OpcodeSload32:
		ptr, offset, _ := instr.LoadData()
		ret := m.compiler.VRegOf(instr.Return())
		m.lowerExtLoad(op, ptr, offset, ret)
	case ssa.OpcodeCall, ssa.OpcodeCallIndirect:
		m.lowerCall(instr)
	case ssa.OpcodeIcmp:
		m.lowerIcmp(instr)
	case ssa.OpcodeVIcmp:
		m.lowerVIcmp(instr)
	case ssa.OpcodeVFcmp:
		m.lowerVFcmp(instr)
	case ssa.OpcodeVCeil:
		m.lowerVecMisc(vecOpFrintp, instr)
	case ssa.OpcodeVFloor:
		m.lowerVecMisc(vecOpFrintm, instr)
	case ssa.OpcodeVTrunc:
		m.lowerVecMisc(vecOpFrintz, instr)
	case ssa.OpcodeVNearest:
		m.lowerVecMisc(vecOpFrintn, instr)
	case ssa.OpcodeVMaxPseudo:
		m.lowerVMinMaxPseudo(instr, true)
	case ssa.OpcodeVMinPseudo:
		m.lowerVMinMaxPseudo(instr, false)
	case ssa.OpcodeBand:
		m.lowerBitwiseAluOp(instr, aluOpAnd, false)
	case ssa.OpcodeBor:
		m.lowerBitwiseAluOp(instr, aluOpOrr, false)
	case ssa.OpcodeBxor:
		m.lowerBitwiseAluOp(instr, aluOpEor, false)
	case ssa.OpcodeIshl:
		m.lowerShifts(instr, extModeNone, aluOpLsl)
	case ssa.OpcodeSshr:
		if instr.Return().Type().Bits() == 64 {
			m.lowerShifts(instr, extModeSignExtend64, aluOpAsr)
		} else {
			m.lowerShifts(instr, extModeSignExtend32, aluOpAsr)
		}
	case ssa.OpcodeUshr:
		if instr.Return().Type().Bits() == 64 {
			m.lowerShifts(instr, extModeZeroExtend64, aluOpLsr)
		} else {
			m.lowerShifts(instr, extModeZeroExtend32, aluOpLsr)
		}
	case ssa.OpcodeRotl:
		m.lowerRotl(instr)
	case ssa.OpcodeRotr:
		m.lowerRotr(instr)
	case ssa.OpcodeSExtend, ssa.OpcodeUExtend:
		from, to, signed := instr.ExtendData()
		m.lowerExtend(instr.Arg(), instr.Return(), from, to, signed)
	case ssa.OpcodeFcmp:
		x, y, c := instr.FcmpData()
		m.lowerFcmp(x, y, instr.Return(), c)
	case ssa.OpcodeImul:
		x, y := instr.Arg2()
		result := instr.Return()
		m.lowerImul(x, y, result)
	case ssa.OpcodeUndefined:
		undef := m.allocateInstr()
		undef.asUDF()
		m.insert(undef)
	case ssa.OpcodeSelect:
		c, x, y := instr.SelectData()
		if x.Type() == ssa.TypeV128 {
			rc := m.getOperand_NR(m.compiler.ValueDefinition(c), extModeNone)
			rn := m.getOperand_NR(m.compiler.ValueDefinition(x), extModeNone)
			rm := m.getOperand_NR(m.compiler.ValueDefinition(y), extModeNone)
			rd := m.compiler.VRegOf(instr.Return())
			m.lowerSelectVec(rc, rn, rm, rd)
		} else {
			m.lowerSelect(c, x, y, instr.Return())
		}
	case ssa.OpcodeClz:
		x := instr.Arg()
		result := instr.Return()
		m.lowerClz(x, result)
	case ssa.OpcodeCtz:
		x := instr.Arg()
		result := instr.Return()
		m.lowerCtz(x, result)
	case ssa.OpcodePopcnt:
		x := instr.Arg()
		result := instr.Return()
		m.lowerPopcnt(x, result)
	case ssa.OpcodeFcvtToSint, ssa.OpcodeFcvtToSintSat:
		x, ctx := instr.Arg2()
		result := instr.Return()
		rn := m.getOperand_NR(m.compiler.ValueDefinition(x), extModeNone)
		rd := m.compiler.VRegOf(result)
		ctxVReg := m.compiler.VRegOf(ctx)
		m.lowerFpuToInt(rd, rn, ctxVReg, true, x.Type() == ssa.TypeF64,
			result.Type().Bits() == 64, op == ssa.OpcodeFcvtToSintSat)
	case ssa.OpcodeFcvtToUint, ssa.OpcodeFcvtToUintSat:
		x, ctx := instr.Arg2()
		result := instr.Return()
		rn := m.getOperand_NR(m.compiler.ValueDefinition(x), extModeNone)
		rd := m.compiler.VRegOf(result)
		ctxVReg := m.compiler.VRegOf(ctx)
		m.lowerFpuToInt(rd, rn, ctxVReg, false, x.Type() == ssa.TypeF64,
			result.Type().Bits() == 64, op == ssa.OpcodeFcvtToUintSat)
	case ssa.OpcodeFcvtFromSint:
		x := instr.Arg()
		result := instr.Return()
		rn := m.getOperand_NR(m.compiler.ValueDefinition(x), extModeNone)
		rd := m.compiler.VRegOf(result)
		m.lowerIntToFpu(rd, rn, true, x.Type() == ssa.TypeI64, result.Type().Bits() == 64)
	case ssa.OpcodeFcvtFromUint:
		x := instr.Arg()
		result := instr.Return()
		rn := m.getOperand_NR(m.compiler.ValueDefinition(x), extModeNone)
		rd := m.compiler.VRegOf(result)
		m.lowerIntToFpu(rd, rn, false, x.Type() == ssa.TypeI64, result.Type().Bits() == 64)
	case ssa.OpcodeFdemote:
		v := instr.Arg()
		rn := m.getOperand_NR(m.compiler.ValueDefinition(v), extModeNone)
		rd := m.compiler.VRegOf(instr.Return())
		cnt := m.allocateInstr()
		cnt.asFpuRR(fpuUniOpCvt64To32, rd, rn, false)
		m.insert(cnt)
	case ssa.OpcodeFpromote:
		v := instr.Arg()
		rn := m.getOperand_NR(m.compiler.ValueDefinition(v), extModeNone)
		rd := m.compiler.VRegOf(instr.Return())
		cnt := m.allocateInstr()
		cnt.asFpuRR(fpuUniOpCvt32To64, rd, rn, true)
		m.insert(cnt)
	case ssa.OpcodeIreduce:
		rn := m.getOperand_NR(m.compiler.ValueDefinition(instr.Arg()), extModeNone)
		retVal := instr.Return()
		rd := m.compiler.VRegOf(retVal)

		if retVal.Type() != ssa.TypeI32 {
			panic("TODO?: Ireduce to non-i32")
		}
		mov := m.allocateInstr()
		mov.asMove32(rd, rn.reg())
		m.insert(mov)
	case ssa.OpcodeFneg:
		m.lowerFpuUniOp(fpuUniOpNeg, instr.Arg(), instr.Return())
	case ssa.OpcodeSqrt:
		m.lowerFpuUniOp(fpuUniOpSqrt, instr.Arg(), instr.Return())
	case ssa.OpcodeCeil:
		m.lowerFpuUniOp(fpuUniOpRoundPlus, instr.Arg(), instr.Return())
	case ssa.OpcodeFloor:
		m.lowerFpuUniOp(fpuUniOpRoundMinus, instr.Arg(), instr.Return())
	case ssa.OpcodeTrunc:
		m.lowerFpuUniOp(fpuUniOpRoundZero, instr.Arg(), instr.Return())
	case ssa.OpcodeNearest:
		m.lowerFpuUniOp(fpuUniOpRoundNearest, instr.Arg(), instr.Return())
	case ssa.OpcodeFabs:
		m.lowerFpuUniOp(fpuUniOpAbs, instr.Arg(), instr.Return())
	case ssa.OpcodeBitcast:
		m.lowerBitcast(instr)
	case ssa.OpcodeFcopysign:
		x, y := instr.Arg2()
		m.lowerFcopysign(x, y, instr.Return())
	case ssa.OpcodeSdiv, ssa.OpcodeUdiv:
		x, y, ctx := instr.Arg3()
		ctxVReg := m.compiler.VRegOf(ctx)
		rn := m.getOperand_NR(m.compiler.ValueDefinition(x), extModeNone)
		rm := m.getOperand_NR(m.compiler.ValueDefinition(y), extModeNone)
		rd := m.compiler.VRegOf(instr.Return())
		m.lowerIDiv(ctxVReg, rd, rn, rm, x.Type() == ssa.TypeI64, op == ssa.OpcodeSdiv)
	case ssa.OpcodeSrem, ssa.OpcodeUrem:
		x, y, ctx := instr.Arg3()
		ctxVReg := m.compiler.VRegOf(ctx)
		rn := m.getOperand_NR(m.compiler.ValueDefinition(x), extModeNone)
		rm := m.getOperand_NR(m.compiler.ValueDefinition(y), extModeNone)
		rd := m.compiler.VRegOf(instr.Return())
		m.lowerIRem(ctxVReg, rd, rn.nr(), rm, x.Type() == ssa.TypeI64, op == ssa.OpcodeSrem)
	case ssa.OpcodeVconst:
		result := m.compiler.VRegOf(instr.Return())
		lo, hi := instr.VconstData()
		v := m.allocateInstr()
		v.asLoadFpuConst128(result, lo, hi)
		m.insert(v)
	case ssa.OpcodeVbnot:
		x := instr.Arg()
		ins := m.allocateInstr()
		rn := m.getOperand_NR(m.compiler.ValueDefinition(x), extModeNone)
		rd := m.compiler.VRegOf(instr.Return())
		ins.asVecMisc(vecOpNot, rd, rn, vecArrangement16B)
		m.insert(ins)
	case ssa.OpcodeVbxor:
		x, y := instr.Arg2()
		m.lowerVecRRR(vecOpEOR, x, y, instr.Return(), vecArrangement16B)
	case ssa.OpcodeVbor:
		x, y := instr.Arg2()
		m.lowerVecRRR(vecOpOrr, x, y, instr.Return(), vecArrangement16B)
	case ssa.OpcodeVband:
		x, y := instr.Arg2()
		m.lowerVecRRR(vecOpAnd, x, y, instr.Return(), vecArrangement16B)
	case ssa.OpcodeVbandnot:
		x, y := instr.Arg2()
		m.lowerVecRRR(vecOpBic, x, y, instr.Return(), vecArrangement16B)
	case ssa.OpcodeVbitselect:
		c, x, y := instr.SelectData()
		rn := m.getOperand_NR(m.compiler.ValueDefinition(x), extModeNone)
		rm := m.getOperand_NR(m.compiler.ValueDefinition(y), extModeNone)
		creg := m.getOperand_NR(m.compiler.ValueDefinition(c), extModeNone)
		tmp := m.compiler.AllocateVReg(ssa.TypeV128)

		// creg is overwritten by BSL, so we need to move it to the result register before the instruction
		// in case when it is used somewhere else.
		mov := m.allocateInstr()
		mov.asFpuMov128(tmp, creg.nr())
		m.insert(mov)

		ins := m.allocateInstr()
		ins.asVecRRRRewrite(vecOpBsl, tmp, rn, rm, vecArrangement16B)
		m.insert(ins)

		mov2 := m.allocateInstr()
		rd := m.compiler.VRegOf(instr.Return())
		mov2.asFpuMov128(rd, tmp)
		m.insert(mov2)
	case ssa.OpcodeVanyTrue, ssa.OpcodeVallTrue:
		x, lane := instr.ArgWithLane()
		var arr vecArrangement
		if op == ssa.OpcodeVallTrue {
			arr = ssaLaneToArrangement(lane)
		}
		rm := m.getOperand_NR(m.compiler.ValueDefinition(x), extModeNone)
		rd := m.compiler.VRegOf(instr.Return())
		m.lowerVcheckTrue(op, rm, rd, arr)
	case ssa.OpcodeVhighBits:
		x, lane := instr.ArgWithLane()
		rm := m.getOperand_NR(m.compiler.ValueDefinition(x), extModeNone)
		rd := m.compiler.VRegOf(instr.Return())
		arr := ssaLaneToArrangement(lane)
		m.lowerVhighBits(rm, rd, arr)
	case ssa.OpcodeVIadd:
		x, y, lane := instr.Arg2WithLane()
		arr := ssaLaneToArrangement(lane)
		m.lowerVecRRR(vecOpAdd, x, y, instr.Return(), arr)
	case ssa.OpcodeExtIaddPairwise:
		v, lane, signed := instr.ExtIaddPairwiseData()
		vv := m.getOperand_NR(m.compiler.ValueDefinition(v), extModeNone)

		tmpLo, tmpHi := operandNR(m.compiler.AllocateVReg(ssa.TypeV128)), operandNR(m.compiler.AllocateVReg(ssa.TypeV128))
		var widen vecOp
		if signed {
			widen = vecOpSshll
		} else {
			widen = vecOpUshll
		}

		var loArr, hiArr, dstArr vecArrangement
		switch lane {
		case ssa.VecLaneI8x16:
			loArr, hiArr, dstArr = vecArrangement8B, vecArrangement16B, vecArrangement8H
		case ssa.VecLaneI16x8:
			loArr, hiArr, dstArr = vecArrangement4H, vecArrangement8H, vecArrangement4S
		case ssa.VecLaneI32x4:
			loArr, hiArr, dstArr = vecArrangement2S, vecArrangement4S, vecArrangement2D
		default:
			panic("unsupported lane " + lane.String())
		}

		widenLo := m.allocateInstr().asVecShiftImm(widen, tmpLo.nr(), vv, operandShiftImm(0), loArr)
		widenHi := m.allocateInstr().asVecShiftImm(widen, tmpHi.nr(), vv, operandShiftImm(0), hiArr)
		addp := m.allocateInstr().asVecRRR(vecOpAddp, m.compiler.VRegOf(instr.Return()), tmpLo, tmpHi, dstArr)
		m.insert(widenLo)
		m.insert(widenHi)
		m.insert(addp)

	case ssa.OpcodeVSaddSat:
		x, y, lane := instr.Arg2WithLane()
		arr := ssaLaneToArrangement(lane)
		m.lowerVecRRR(vecOpSqadd, x, y, instr.Return(), arr)
	case ssa.OpcodeVUaddSat:
		x, y, lane := instr.Arg2WithLane()
		arr := ssaLaneToArrangement(lane)
		m.lowerVecRRR(vecOpUqadd, x, y, instr.Return(), arr)
	case ssa.OpcodeVIsub:
		x, y, lane := instr.Arg2WithLane()
		arr := ssaLaneToArrangement(lane)
		m.lowerVecRRR(vecOpSub, x, y, instr.Return(), arr)
	case ssa.OpcodeVSsubSat:
		x, y, lane := instr.Arg2WithLane()
		arr := ssaLaneToArrangement(lane)
		m.lowerVecRRR(vecOpSqsub, x, y, instr.Return(), arr)
	case ssa.OpcodeVUsubSat:
		x, y, lane := instr.Arg2WithLane()
		arr := ssaLaneToArrangement(lane)
		m.lowerVecRRR(vecOpUqsub, x, y, instr.Return(), arr)
	case ssa.OpcodeVImin:
		x, y, lane := instr.Arg2WithLane()
		arr := ssaLaneToArrangement(lane)
		m.lowerVecRRR(vecOpSmin, x, y, instr.Return(), arr)
	case ssa.OpcodeVUmin:
		x, y, lane := instr.Arg2WithLane()
		arr := ssaLaneToArrangement(lane)
		m.lowerVecRRR(vecOpUmin, x, y, instr.Return(), arr)
	case ssa.OpcodeVImax:
		x, y, lane := instr.Arg2WithLane()
		arr := ssaLaneToArrangement(lane)
		m.lowerVecRRR(vecOpSmax, x, y, instr.Return(), arr)
	case ssa.OpcodeVUmax:
		x, y, lane := instr.Arg2WithLane()
		arr := ssaLaneToArrangement(lane)
		m.lowerVecRRR(vecOpUmax, x, y, instr.Return(), arr)
	case ssa.OpcodeVAvgRound:
		x, y, lane := instr.Arg2WithLane()
		arr := ssaLaneToArrangement(lane)
		m.lowerVecRRR(vecOpUrhadd, x, y, instr.Return(), arr)
	case ssa.OpcodeVImul:
		x, y, lane := instr.Arg2WithLane()
		arr := ssaLaneToArrangement(lane)
		rn := m.getOperand_NR(m.compiler.ValueDefinition(x), extModeNone)
		rm := m.getOperand_NR(m.compiler.ValueDefinition(y), extModeNone)
		rd := m.compiler.VRegOf(instr.Return())
		m.lowerVIMul(rd, rn, rm, arr)
	case ssa.OpcodeVIabs:
		m.lowerVecMisc(vecOpAbs, instr)
	case ssa.OpcodeVIneg:
		m.lowerVecMisc(vecOpNeg, instr)
	case ssa.OpcodeVIpopcnt:
		m.lowerVecMisc(vecOpCnt, instr)
	case ssa.OpcodeVIshl,
		ssa.OpcodeVSshr, ssa.OpcodeVUshr:
		x, y, lane := instr.Arg2WithLane()
		arr := ssaLaneToArrangement(lane)
		rn := m.getOperand_NR(m.compiler.ValueDefinition(x), extModeNone)
		rm := m.getOperand_NR(m.compiler.ValueDefinition(y), extModeNone)
		rd := m.compiler.VRegOf(instr.Return())
		m.lowerVShift(op, rd, rn, rm, arr)
	case ssa.OpcodeVSqrt:
		m.lowerVecMisc(vecOpFsqrt, instr)
	case ssa.OpcodeVFabs:
		m.lowerVecMisc(vecOpFabs, instr)
	case ssa.OpcodeVFneg:
		m.lowerVecMisc(vecOpFneg, instr)
	case ssa.OpcodeVFmin:
		x, y, lane := instr.Arg2WithLane()
		arr := ssaLaneToArrangement(lane)
		m.lowerVecRRR(vecOpFmin, x, y, instr.Return(), arr)
	case ssa.OpcodeVFmax:
		x, y, lane := instr.Arg2WithLane()
		arr := ssaLaneToArrangement(lane)
		m.lowerVecRRR(vecOpFmax, x, y, instr.Return(), arr)
	case ssa.OpcodeVFadd:
		x, y, lane := instr.Arg2WithLane()
		arr := ssaLaneToArrangement(lane)
		m.lowerVecRRR(vecOpFadd, x, y, instr.Return(), arr)
	case ssa.OpcodeVFsub:
		x, y, lane := instr.Arg2WithLane()
		arr := ssaLaneToArrangement(lane)
		m.lowerVecRRR(vecOpFsub, x, y, instr.Return(), arr)
	case ssa.OpcodeVFmul:
		x, y, lane := instr.Arg2WithLane()
		arr := ssaLaneToArrangement(lane)
		m.lowerVecRRR(vecOpFmul, x, y, instr.Return(), arr)
	case ssa.OpcodeSqmulRoundSat:
		x, y, lane := instr.Arg2WithLane()
		arr := ssaLaneToArrangement(lane)
		m.lowerVecRRR(vecOpSqrdmulh, x, y, instr.Return(), arr)
	case ssa.OpcodeVFdiv:
		x, y, lane := instr.Arg2WithLane()
		arr := ssaLaneToArrangement(lane)
		m.lowerVecRRR(vecOpFdiv, x, y, instr.Return(), arr)
	case ssa.OpcodeVFcvtToSintSat, ssa.OpcodeVFcvtToUintSat:
		x, lane := instr.ArgWithLane()
		arr := ssaLaneToArrangement(lane)
		rn := m.getOperand_NR(m.compiler.ValueDefinition(x), extModeNone)
		rd := m.compiler.VRegOf(instr.Return())
		m.lowerVfpuToInt(rd, rn, arr, op == ssa.OpcodeVFcvtToSintSat)
	case ssa.OpcodeVFcvtFromSint, ssa.OpcodeVFcvtFromUint:
		x, lane := instr.ArgWithLane()
		arr := ssaLaneToArrangement(lane)
		rn := m.getOperand_NR(m.compiler.ValueDefinition(x), extModeNone)
		rd := m.compiler.VRegOf(instr.Return())
		m.lowerVfpuFromInt(rd, rn, arr, op == ssa.OpcodeVFcvtFromSint)
	case ssa.OpcodeSwidenLow, ssa.OpcodeUwidenLow:
		x, lane := instr.ArgWithLane()
		rn := m.getOperand_NR(m.compiler.ValueDefinition(x), extModeNone)
		rd := m.compiler.VRegOf(instr.Return())

		var arr vecArrangement
		switch lane {
		case ssa.VecLaneI8x16:
			arr = vecArrangement8B
		case ssa.VecLaneI16x8:
			arr = vecArrangement4H
		case ssa.VecLaneI32x4:
			arr = vecArrangement2S
		}

		shll := m.allocateInstr()
		if signed := op == ssa.OpcodeSwidenLow; signed {
			shll.asVecShiftImm(vecOpSshll, rd, rn, operandShiftImm(0), arr)
		} else {
			shll.asVecShiftImm(vecOpUshll, rd, rn, operandShiftImm(0), arr)
		}
		m.insert(shll)
	case ssa.OpcodeSwidenHigh, ssa.OpcodeUwidenHigh:
		x, lane := instr.ArgWithLane()
		rn := m.getOperand_NR(m.compiler.ValueDefinition(x), extModeNone)
		rd := m.compiler.VRegOf(instr.Return())

		arr := ssaLaneToArrangement(lane)

		shll := m.allocateInstr()
		if signed := op == ssa.OpcodeSwidenHigh; signed {
			shll.asVecShiftImm(vecOpSshll, rd, rn, operandShiftImm(0), arr)
		} else {
			shll.asVecShiftImm(vecOpUshll, rd, rn, operandShiftImm(0), arr)
		}
		m.insert(shll)

	case ssa.OpcodeSnarrow, ssa.OpcodeUnarrow:
		x, y, lane := instr.Arg2WithLane()
		var arr, arr2 vecArrangement
		switch lane {
		case ssa.VecLaneI16x8: // I16x8
			arr = vecArrangement8B
			arr2 = vecArrangement16B // Implies sqxtn2.
		case ssa.VecLaneI32x4:
			arr = vecArrangement4H
			arr2 = vecArrangement8H // Implies sqxtn2.
		default:
			panic("unsupported lane " + lane.String())
		}
		rn := m.getOperand_NR(m.compiler.ValueDefinition(x), extModeNone)
		rm := m.getOperand_NR(m.compiler.ValueDefinition(y), extModeNone)
		rd := m.compiler.VRegOf(instr.Return())

		tmp := m.compiler.AllocateVReg(ssa.TypeV128)

		loQxtn := m.allocateInstr()
		hiQxtn := m.allocateInstr()
		if signed := op == ssa.OpcodeSnarrow; signed {
			// Narrow lanes on rn and write them into lower-half of rd.
			loQxtn.asVecMisc(vecOpSqxtn, tmp, rn, arr) // low
			// Narrow lanes on rm and write them into higher-half of rd.
			hiQxtn.asVecMisc(vecOpSqxtn, tmp, rm, arr2) // high (sqxtn2)
		} else {
			// Narrow lanes on rn and write them into lower-half of rd.
			loQxtn.asVecMisc(vecOpSqxtun, tmp, rn, arr) // low
			// Narrow lanes on rm and write them into higher-half of rd.
			hiQxtn.asVecMisc(vecOpSqxtun, tmp, rm, arr2) // high (sqxtn2)
		}
		m.insert(loQxtn)
		m.insert(hiQxtn)

		mov := m.allocateInstr()
		mov.asFpuMov128(rd, tmp)
		m.insert(mov)
	case ssa.OpcodeFvpromoteLow:
		x, lane := instr.ArgWithLane()
		if lane != ssa.VecLaneF32x4 {
			panic("unsupported lane type " + lane.String())
		}
		ins := m.allocateInstr()
		rn := m.getOperand_NR(m.compiler.ValueDefinition(x), extModeNone)
		rd := m.compiler.VRegOf(instr.Return())
		ins.asVecMisc(vecOpFcvtl, rd, rn, vecArrangement2S)
		m.insert(ins)
	case ssa.OpcodeFvdemote:
		x, lane := instr.ArgWithLane()
		if lane != ssa.VecLaneF64x2 {
			panic("unsupported lane type " + lane.String())
		}
		ins := m.allocateInstr()
		rn := m.getOperand_NR(m.compiler.ValueDefinition(x), extModeNone)
		rd := m.compiler.VRegOf(instr.Return())
		ins.asVecMisc(vecOpFcvtn, rd, rn, vecArrangement2S)
		m.insert(ins)
	case ssa.OpcodeExtractlane:
		x, index, signed, lane := instr.ExtractlaneData()

		rn := m.getOperand_NR(m.compiler.ValueDefinition(x), extModeNone)
		rd := m.compiler.VRegOf(instr.Return())

		mov := m.allocateInstr()
		switch lane {
		case ssa.VecLaneI8x16:
			mov.asMovFromVec(rd, rn, vecArrangementB, vecIndex(index), signed)
		case ssa.VecLaneI16x8:
			mov.asMovFromVec(rd, rn, vecArrangementH, vecIndex(index), signed)
		case ssa.VecLaneI32x4:
			mov.asMovFromVec(rd, rn, vecArrangementS, vecIndex(index), signed)
		case ssa.VecLaneI64x2:
			mov.asMovFromVec(rd, rn, vecArrangementD, vecIndex(index), signed)
		case ssa.VecLaneF32x4:
			mov.asVecMovElement(rd, rn, vecArrangementS, vecIndex(0), vecIndex(index))
		case ssa.VecLaneF64x2:
			mov.asVecMovElement(rd, rn, vecArrangementD, vecIndex(0), vecIndex(index))
		default:
			panic("unsupported lane: " + lane.String())
		}

		m.insert(mov)

	case ssa.OpcodeInsertlane:
		x, y, index, lane := instr.InsertlaneData()
		rn := m.getOperand_NR(m.compiler.ValueDefinition(x), extModeNone)
		rm := m.getOperand_NR(m.compiler.ValueDefinition(y), extModeNone)
		rd := m.compiler.VRegOf(instr.Return())
		tmpReg := m.compiler.AllocateVReg(ssa.TypeV128)

		// Initially mov rn to tmp.
		mov1 := m.allocateInstr()
		mov1.asFpuMov128(tmpReg, rn.nr())
		m.insert(mov1)

		// movToVec and vecMovElement do not clear the remaining bits to zero,
		// thus, we can mov rm in-place to tmp.
		mov2 := m.allocateInstr()
		switch lane {
		case ssa.VecLaneI8x16:
			mov2.asMovToVec(tmpReg, rm, vecArrangementB, vecIndex(index))
		case ssa.VecLaneI16x8:
			mov2.asMovToVec(tmpReg, rm, vecArrangementH, vecIndex(index))
		case ssa.VecLaneI32x4:
			mov2.asMovToVec(tmpReg, rm, vecArrangementS, vecIndex(index))
		case ssa.VecLaneI64x2:
			mov2.asMovToVec(tmpReg, rm, vecArrangementD, vecIndex(index))
		case ssa.VecLaneF32x4:
			mov2.asVecMovElement(tmpReg, rm, vecArrangementS, vecIndex(index), vecIndex(0))
		case ssa.VecLaneF64x2:
			mov2.asVecMovElement(tmpReg, rm, vecArrangementD, vecIndex(index), vecIndex(0))
		}
		m.insert(mov2)

		// Finally mov tmp to rd.
		mov3 := m.allocateInstr()
		mov3.asFpuMov128(rd, tmpReg)
		m.insert(mov3)

	case ssa.OpcodeSwizzle:
		x, y, lane := instr.Arg2WithLane()
		rn := m.getOperand_NR(m.compiler.ValueDefinition(x), extModeNone)
		rm := m.getOperand_NR(m.compiler.ValueDefinition(y), extModeNone)
		rd := m.compiler.VRegOf(instr.Return())

		arr := ssaLaneToArrangement(lane)

		// tbl <rd>.<arr>, { <rn>.<arr> }, <rm>.<arr>
		tbl1 := m.allocateInstr()
		tbl1.asVecTbl(1, rd, rn, rm, arr)
		m.insert(tbl1)

	case ssa.OpcodeShuffle:
		x, y, lane1, lane2 := instr.ShuffleData()
		rn := m.getOperand_NR(m.compiler.ValueDefinition(x), extModeNone)
		rm := m.getOperand_NR(m.compiler.ValueDefinition(y), extModeNone)
		rd := m.compiler.VRegOf(instr.Return())

		m.lowerShuffle(rd, rn, rm, lane1, lane2)

	case ssa.OpcodeSplat:
		x, lane := instr.ArgWithLane()
		rn := m.getOperand_NR(m.compiler.ValueDefinition(x), extModeNone)
		rd := m.compiler.VRegOf(instr.Return())

		dup := m.allocateInstr()
		switch lane {
		case ssa.VecLaneI8x16:
			dup.asVecDup(rd, rn, vecArrangement16B)
		case ssa.VecLaneI16x8:
			dup.asVecDup(rd, rn, vecArrangement8H)
		case ssa.VecLaneI32x4:
			dup.asVecDup(rd, rn, vecArrangement4S)
		case ssa.VecLaneI64x2:
			dup.asVecDup(rd, rn, vecArrangement2D)
		case ssa.VecLaneF32x4:
			dup.asVecDupElement(rd, rn, vecArrangementS, vecIndex(0))
		case ssa.VecLaneF64x2:
			dup.asVecDupElement(rd, rn, vecArrangementD, vecIndex(0))
		}
		m.insert(dup)

	case ssa.OpcodeWideningPairwiseDotProductS:
		x, y := instr.Arg2()
		xx, yy := m.getOperand_NR(m.compiler.ValueDefinition(x), extModeNone),
			m.getOperand_NR(m.compiler.ValueDefinition(y), extModeNone)
		tmp, tmp2 := operandNR(m.compiler.AllocateVReg(ssa.TypeV128)), operandNR(m.compiler.AllocateVReg(ssa.TypeV128))
		m.insert(m.allocateInstr().asVecRRR(vecOpSmull, tmp.nr(), xx, yy, vecArrangement8H))
		m.insert(m.allocateInstr().asVecRRR(vecOpSmull2, tmp2.nr(), xx, yy, vecArrangement8H))
		m.insert(m.allocateInstr().asVecRRR(vecOpAddp, tmp.nr(), tmp, tmp2, vecArrangement4S))

		rd := m.compiler.VRegOf(instr.Return())
		m.insert(m.allocateInstr().asFpuMov128(rd, tmp.nr()))

	case ssa.OpcodeLoadSplat:
		ptr, offset, lane := instr.LoadSplatData()
		m.lowerLoadSplat(ptr, offset, lane, instr.Return())

	case ssa.OpcodeAtomicRmw:
		m.lowerAtomicRmw(instr)

	case ssa.OpcodeAtomicCas:
		m.lowerAtomicCas(instr)

	case ssa.OpcodeAtomicLoad:
		m.lowerAtomicLoad(instr)

	case ssa.OpcodeAtomicStore:
		m.lowerAtomicStore(instr)

	case ssa.OpcodeFence:
		instr := m.allocateInstr()
		instr.asDMB()
		m.insert(instr)

	default:
		panic("TODO: lowering " + op.String())
	}
	m.FlushPendingInstructions()
}

func (m *machine) lowerShuffle(rd regalloc.VReg, rn, rm operand, lane1, lane2 uint64) {
	// `tbl2` requires 2 consecutive registers, so we arbitrarily pick v29, v30.
	vReg, wReg := v29VReg, v30VReg

	// Initialize v29, v30 to rn, rm.
	movv := m.allocateInstr()
	movv.asFpuMov128(vReg, rn.nr())
	m.insert(movv)

	movw := m.allocateInstr()
	movw.asFpuMov128(wReg, rm.nr())
	m.insert(movw)

	// `lane1`, `lane2` are already encoded as two u64s with the right layout:
	//     lane1 := lane[7]<<56 | ... | lane[1]<<8 | lane[0]
	//     lane2 := lane[15]<<56 | ... | lane[9]<<8 | lane[8]
	// Thus, we can use loadFpuConst128.
	tmp := operandNR(m.compiler.AllocateVReg(ssa.TypeV128))
	lfc := m.allocateInstr()
	lfc.asLoadFpuConst128(tmp.nr(), lane1, lane2)
	m.insert(lfc)

	// tbl <rd>.16b, { <vReg>.16B, <wReg>.16b }, <tmp>.16b
	tbl2 := m.allocateInstr()
	tbl2.asVecTbl(2, rd, operandNR(vReg), tmp, vecArrangement16B)
	m.insert(tbl2)
}

func (m *machine) lowerVShift(op ssa.Opcode, rd regalloc.VReg, rn, rm operand, arr vecArrangement) {
	var modulo byte
	switch arr {
	case vecArrangement16B:
		modulo = 0x7 // Modulo 8.
	case vecArrangement8H:
		modulo = 0xf // Modulo 16.
	case vecArrangement4S:
		modulo = 0x1f // Modulo 32.
	case vecArrangement2D:
		modulo = 0x3f // Modulo 64.
	default:
		panic("unsupported arrangment " + arr.String())
	}

	rtmp := operandNR(m.compiler.AllocateVReg(ssa.TypeI64))
	vtmp := operandNR(m.compiler.AllocateVReg(ssa.TypeV128))

	and := m.allocateInstr()
	and.asALUBitmaskImm(aluOpAnd, rtmp.nr(), rm.nr(), uint64(modulo), true)
	m.insert(and)

	if op != ssa.OpcodeVIshl {
		// Negate the amount to make this as right shift.
		neg := m.allocateInstr()
		neg.asALU(aluOpSub, rtmp.nr(), operandNR(xzrVReg), rtmp, true)
		m.insert(neg)
	}

	// Copy the shift amount into a vector register as sshl/ushl requires it to be there.
	dup := m.allocateInstr()
	dup.asVecDup(vtmp.nr(), rtmp, arr)
	m.insert(dup)

	if op == ssa.OpcodeVIshl || op == ssa.OpcodeVSshr {
		sshl := m.allocateInstr()
		sshl.asVecRRR(vecOpSshl, rd, rn, vtmp, arr)
		m.insert(sshl)
	} else {
		ushl := m.allocateInstr()
		ushl.asVecRRR(vecOpUshl, rd, rn, vtmp, arr)
		m.insert(ushl)
	}
}

func (m *machine) lowerVcheckTrue(op ssa.Opcode, rm operand, rd regalloc.VReg, arr vecArrangement) {
	tmp := operandNR(m.compiler.AllocateVReg(ssa.TypeV128))

	// Special case VallTrue for i64x2.
	if op == ssa.OpcodeVallTrue && arr == vecArrangement2D {
		// 	cmeq v3?.2d, v2?.2d, #0
		//	addp v3?.2d, v3?.2d, v3?.2d
		//	fcmp v3?, v3?
		//	cset dst, eq

		ins := m.allocateInstr()
		ins.asVecMisc(vecOpCmeq0, tmp.nr(), rm, vecArrangement2D)
		m.insert(ins)

		addp := m.allocateInstr()
		addp.asVecRRR(vecOpAddp, tmp.nr(), tmp, tmp, vecArrangement2D)
		m.insert(addp)

		fcmp := m.allocateInstr()
		fcmp.asFpuCmp(tmp, tmp, true)
		m.insert(fcmp)

		cset := m.allocateInstr()
		cset.asCSet(rd, false, eq)
		m.insert(cset)

		return
	}

	// Create a scalar value with umaxp or uminv, then compare it against zero.
	ins := m.allocateInstr()
	if op == ssa.OpcodeVanyTrue {
		// 	umaxp v4?.16b, v2?.16b, v2?.16b
		ins.asVecRRR(vecOpUmaxp, tmp.nr(), rm, rm, vecArrangement16B)
	} else {
		// 	uminv d4?, v2?.4s
		ins.asVecLanes(vecOpUminv, tmp.nr(), rm, arr)
	}
	m.insert(ins)

	//	mov x3?, v4?.d[0]
	//	ccmp x3?, #0x0, #0x0, al
	//	cset x3?, ne
	//	mov x0, x3?

	movv := m.allocateInstr()
	movv.asMovFromVec(rd, tmp, vecArrangementD, vecIndex(0), false)
	m.insert(movv)

	fc := m.allocateInstr()
	fc.asCCmpImm(operandNR(rd), uint64(0), al, 0, true)
	m.insert(fc)

	cset := m.allocateInstr()
	cset.asCSet(rd, false, ne)
	m.insert(cset)
}

func (m *machine) lowerVhighBits(rm operand, rd regalloc.VReg, arr vecArrangement) {
	r0 := operandNR(m.compiler.AllocateVReg(ssa.TypeI64))
	v0 := operandNR(m.compiler.AllocateVReg(ssa.TypeV128))
	v1 := operandNR(m.compiler.AllocateVReg(ssa.TypeV128))

	switch arr {
	case vecArrangement16B:
		//	sshr v6?.16b, v2?.16b, #7
		//	movz x4?, #0x201, lsl 0
		//	movk x4?, #0x804, lsl 16
		//	movk x4?, #0x2010, lsl 32
		//	movk x4?, #0x8040, lsl 48
		//	dup v5?.2d, x4?
		//	and v6?.16b, v6?.16b, v5?.16b
		//	ext v5?.16b, v6?.16b, v6?.16b, #8
		//	zip1 v5?.16b, v6?.16b, v5?.16b
		//	addv s5?, v5?.8h
		//	umov s3?, v5?.h[0]

		// Right arithmetic shift on the original vector and store the result into v1. So we have:
		// v1[i] = 0xff if vi<0, 0 otherwise.
		sshr := m.allocateInstr()
		sshr.asVecShiftImm(vecOpSshr, v1.nr(), rm, operandShiftImm(7), vecArrangement16B)
		m.insert(sshr)

		// Load the bit mask into r0.
		m.insertMOVZ(r0.nr(), 0x0201, 0, true)
		m.insertMOVK(r0.nr(), 0x0804, 1, true)
		m.insertMOVK(r0.nr(), 0x2010, 2, true)
		m.insertMOVK(r0.nr(), 0x8040, 3, true)

		// dup r0 to v0.
		dup := m.allocateInstr()
		dup.asVecDup(v0.nr(), r0, vecArrangement2D)
		m.insert(dup)

		// Lane-wise logical AND with the bit mask, meaning that we have
		// v[i] = (1 << i) if vi<0, 0 otherwise.
		//
		// Below, we use the following notation:
		// wi := (1 << i) if vi<0, 0 otherwise.
		and := m.allocateInstr()
		and.asVecRRR(vecOpAnd, v1.nr(), v1, v0, vecArrangement16B)
		m.insert(and)

		// Swap the lower and higher 8 byte elements, and write it into v0, meaning that we have
		// v0[i] = w(i+8) if i < 8, w(i-8) otherwise.
		ext := m.allocateInstr()
		ext.asVecExtract(v0.nr(), v1, v1, vecArrangement16B, uint32(8))
		m.insert(ext)

		// v = [w0, w8, ..., w7, w15]
		zip1 := m.allocateInstr()
		zip1.asVecPermute(vecOpZip1, v0.nr(), v1, v0, vecArrangement16B)
		m.insert(zip1)

		// v.h[0] = w0 + ... + w15
		addv := m.allocateInstr()
		addv.asVecLanes(vecOpAddv, v0.nr(), v0, vecArrangement8H)
		m.insert(addv)

		// Extract the v.h[0] as the result.
		movfv := m.allocateInstr()
		movfv.asMovFromVec(rd, v0, vecArrangementH, vecIndex(0), false)
		m.insert(movfv)
	case vecArrangement8H:
		//	sshr v6?.8h, v2?.8h, #15
		//	movz x4?, #0x1, lsl 0
		//	movk x4?, #0x2, lsl 16
		//	movk x4?, #0x4, lsl 32
		//	movk x4?, #0x8, lsl 48
		//	dup v5?.2d, x4?
		//	lsl x4?, x4?, 0x4
		//	ins v5?.d[1], x4?
		//	and v5?.16b, v6?.16b, v5?.16b
		//	addv s5?, v5?.8h
		//	umov s3?, v5?.h[0]

		// Right arithmetic shift on the original vector and store the result into v1. So we have:
		// v[i] = 0xffff if vi<0, 0 otherwise.
		sshr := m.allocateInstr()
		sshr.asVecShiftImm(vecOpSshr, v1.nr(), rm, operandShiftImm(15), vecArrangement8H)
		m.insert(sshr)

		// Load the bit mask into r0.
		m.lowerConstantI64(r0.nr(), 0x0008000400020001)

		// dup r0 to vector v0.
		dup := m.allocateInstr()
		dup.asVecDup(v0.nr(), r0, vecArrangement2D)
		m.insert(dup)

		lsl := m.allocateInstr()
		lsl.asALUShift(aluOpLsl, r0.nr(), r0, operandShiftImm(4), true)
		m.insert(lsl)

		movv := m.allocateInstr()
		movv.asMovToVec(v0.nr(), r0, vecArrangementD, vecIndex(1))
		m.insert(movv)

		// Lane-wise logical AND with the bitmask, meaning that we have
		// v[i] = (1 << i)     if vi<0, 0 otherwise for i=0..3
		//      = (1 << (i+4)) if vi<0, 0 otherwise for i=3..7
		and := m.allocateInstr()
		and.asVecRRR(vecOpAnd, v0.nr(), v1, v0, vecArrangement16B)
		m.insert(and)

		addv := m.allocateInstr()
		addv.asVecLanes(vecOpAddv, v0.nr(), v0, vecArrangement8H)
		m.insert(addv)

		movfv := m.allocateInstr()
		movfv.asMovFromVec(rd, v0, vecArrangementH, vecIndex(0), false)
		m.insert(movfv)
	case vecArrangement4S:
		// 	sshr v6?.8h, v2?.8h, #15
		//	movz x4?, #0x1, lsl 0
		//	movk x4?, #0x2, lsl 16
		//	movk x4?, #0x4, lsl 32
		//	movk x4?, #0x8, lsl 48
		//	dup v5?.2d, x4?
		//	lsl x4?, x4?, 0x4
		//	ins v5?.d[1], x4?
		//	and v5?.16b, v6?.16b, v5?.16b
		//	addv s5?, v5?.8h
		//	umov s3?, v5?.h[0]

		// Right arithmetic shift on the original vector and store the result into v1. So we have:
		// v[i] = 0xffffffff if vi<0, 0 otherwise.
		sshr := m.allocateInstr()
		sshr.asVecShiftImm(vecOpSshr, v1.nr(), rm, operandShiftImm(31), vecArrangement4S)
		m.insert(sshr)

		// Load the bit mask into r0.
		m.lowerConstantI64(r0.nr(), 0x0000000200000001)

		// dup r0 to vector v0.
		dup := m.allocateInstr()
		dup.asVecDup(v0.nr(), r0, vecArrangement2D)
		m.insert(dup)

		lsl := m.allocateInstr()
		lsl.asALUShift(aluOpLsl, r0.nr(), r0, operandShiftImm(2), true)
		m.insert(lsl)

		movv := m.allocateInstr()
		movv.asMovToVec(v0.nr(), r0, vecArrangementD, vecIndex(1))
		m.insert(movv)

		// Lane-wise logical AND with the bitmask, meaning that we have
		// v[i] = (1 << i)     if vi<0, 0 otherwise for i in [0, 1]
		//      = (1 << (i+4)) if vi<0, 0 otherwise for i in [2, 3]
		and := m.allocateInstr()
		and.asVecRRR(vecOpAnd, v0.nr(), v1, v0, vecArrangement16B)
		m.insert(and)

		addv := m.allocateInstr()
		addv.asVecLanes(vecOpAddv, v0.nr(), v0, vecArrangement4S)
		m.insert(addv)

		movfv := m.allocateInstr()
		movfv.asMovFromVec(rd, v0, vecArrangementS, vecIndex(0), false)
		m.insert(movfv)
	case vecArrangement2D:
		// 	mov d3?, v2?.d[0]
		//	mov x4?, v2?.d[1]
		//	lsr x4?, x4?, 0x3f
		//	lsr d3?, d3?, 0x3f
		//	add s3?, s3?, w4?, lsl #1

		// Move the lower 64-bit int into result.
		movv0 := m.allocateInstr()
		movv0.asMovFromVec(rd, rm, vecArrangementD, vecIndex(0), false)
		m.insert(movv0)

		// Move the higher 64-bit int into r0.
		movv1 := m.allocateInstr()
		movv1.asMovFromVec(r0.nr(), rm, vecArrangementD, vecIndex(1), false)
		m.insert(movv1)

		// Move the sign bit into the least significant bit.
		lsr1 := m.allocateInstr()
		lsr1.asALUShift(aluOpLsr, r0.nr(), r0, operandShiftImm(63), true)
		m.insert(lsr1)

		lsr2 := m.allocateInstr()
		lsr2.asALUShift(aluOpLsr, rd, operandNR(rd), operandShiftImm(63), true)
		m.insert(lsr2)

		// rd = (r0<<1) | rd
		lsl := m.allocateInstr()
		lsl.asALU(aluOpAdd, rd, operandNR(rd), operandSR(r0.nr(), 1, shiftOpLSL), false)
		m.insert(lsl)
	default:
		panic("Unsupported " + arr.String())
	}
}

func (m *machine) lowerVecMisc(op vecOp, instr *ssa.Instruction) {
	x, lane := instr.ArgWithLane()
	arr := ssaLaneToArrangement(lane)
	ins := m.allocateInstr()
	rn := m.getOperand_NR(m.compiler.ValueDefinition(x), extModeNone)
	rd := m.compiler.VRegOf(instr.Return())
	ins.asVecMisc(op, rd, rn, arr)
	m.insert(ins)
}

func (m *machine) lowerVecRRR(op vecOp, x, y, ret ssa.Value, arr vecArrangement) {
	ins := m.allocateInstr()
	rn := m.getOperand_NR(m.compiler.ValueDefinition(x), extModeNone)
	rm := m.getOperand_NR(m.compiler.ValueDefinition(y), extModeNone)
	rd := m.compiler.VRegOf(ret)
	ins.asVecRRR(op, rd, rn, rm, arr)
	m.insert(ins)
}

func (m *machine) lowerVIMul(rd regalloc.VReg, rn, rm operand, arr vecArrangement) {
	if arr != vecArrangement2D {
		mul := m.allocateInstr()
		mul.asVecRRR(vecOpMul, rd, rn, rm, arr)
		m.insert(mul)
	} else {
		tmp1 := m.compiler.AllocateVReg(ssa.TypeV128)
		tmp2 := m.compiler.AllocateVReg(ssa.TypeV128)
		tmp3 := m.compiler.AllocateVReg(ssa.TypeV128)

		tmpRes := m.compiler.AllocateVReg(ssa.TypeV128)

		// Following the algorithm in https://chromium-review.googlesource.com/c/v8/v8/+/1781696
		rev64 := m.allocateInstr()
		rev64.asVecMisc(vecOpRev64, tmp2, rm, vecArrangement4S)
		m.insert(rev64)

		mul := m.allocateInstr()
		mul.asVecRRR(vecOpMul, tmp2, operandNR(tmp2), rn, vecArrangement4S)
		m.insert(mul)

		xtn1 := m.allocateInstr()
		xtn1.asVecMisc(vecOpXtn, tmp1, rn, vecArrangement2S)
		m.insert(xtn1)

		addp := m.allocateInstr()
		addp.asVecRRR(vecOpAddp, tmp2, operandNR(tmp2), operandNR(tmp2), vecArrangement4S)
		m.insert(addp)

		xtn2 := m.allocateInstr()
		xtn2.asVecMisc(vecOpXtn, tmp3, rm, vecArrangement2S)
		m.insert(xtn2)

		// Note: do not write the result directly into result yet. This is the same reason as in bsl.
		// In short, in UMLAL instruction, the result register is also one of the source register, and
		// the value on the result register is significant.
		shll := m.allocateInstr()
		shll.asVecMisc(vecOpShll, tmpRes, operandNR(tmp2), vecArrangement2S)
		m.insert(shll)

		umlal := m.allocateInstr()
		umlal.asVecRRRRewrite(vecOpUmlal, tmpRes, operandNR(tmp3), operandNR(tmp1), vecArrangement2S)
		m.insert(umlal)

		mov := m.allocateInstr()
		mov.asFpuMov128(rd, tmpRes)
		m.insert(mov)
	}
}

func (m *machine) lowerVMinMaxPseudo(instr *ssa.Instruction, max bool) {
	x, y, lane := instr.Arg2WithLane()
	arr := ssaLaneToArrangement(lane)

	rn := m.getOperand_NR(m.compiler.ValueDefinition(x), extModeNone)
	rm := m.getOperand_NR(m.compiler.ValueDefinition(y), extModeNone)

	// Note: this usage of tmp is important.
	// BSL modifies the destination register, so we need to use a temporary register so that
	// the actual definition of the destination register happens *after* the BSL instruction.
	// That way, we can force the spill instruction to be inserted after the BSL instruction.
	tmp := m.compiler.AllocateVReg(ssa.TypeV128)

	fcmgt := m.allocateInstr()
	if max {
		fcmgt.asVecRRR(vecOpFcmgt, tmp, rm, rn, arr)
	} else {
		// If min, swap the args.
		fcmgt.asVecRRR(vecOpFcmgt, tmp, rn, rm, arr)
	}
	m.insert(fcmgt)

	bsl := m.allocateInstr()
	bsl.asVecRRRRewrite(vecOpBsl, tmp, rm, rn, vecArrangement16B)
	m.insert(bsl)

	res := operandNR(m.compiler.VRegOf(instr.Return()))
	mov2 := m.allocateInstr()
	mov2.asFpuMov128(res.nr(), tmp)
	m.insert(mov2)
}

func (m *machine) lowerIRem(execCtxVReg regalloc.VReg, rd, rn regalloc.VReg, rm operand, _64bit, signed bool) {
	div := m.allocateInstr()

	if signed {
		div.asALU(aluOpSDiv, rd, operandNR(rn), rm, _64bit)
	} else {
		div.asALU(aluOpUDiv, rd, operandNR(rn), rm, _64bit)
	}
	m.insert(div)

	// Check if rm is zero:
	m.exitIfNot(execCtxVReg, registerAsRegNotZeroCond(rm.nr()), _64bit, wazevoapi.ExitCodeIntegerDivisionByZero)

	// rd = rn-rd*rm by MSUB instruction.
	msub := m.allocateInstr()
	msub.asALURRRR(aluOpMSub, rd, operandNR(rd), rm, rn, _64bit)
	m.insert(msub)
}

func (m *machine) lowerIDiv(execCtxVReg, rd regalloc.VReg, rn, rm operand, _64bit, signed bool) {
	div := m.allocateInstr()

	if signed {
		div.asALU(aluOpSDiv, rd, rn, rm, _64bit)
	} else {
		div.asALU(aluOpUDiv, rd, rn, rm, _64bit)
	}
	m.insert(div)

	// Check if rm is zero:
	m.exitIfNot(execCtxVReg, registerAsRegNotZeroCond(rm.nr()), _64bit, wazevoapi.ExitCodeIntegerDivisionByZero)

	if signed {
		// We need to check the signed overflow which happens iff "math.MinInt{32,64} / -1"
		minusOneCheck := m.allocateInstr()
		// Sets eq condition if rm == -1.
		minusOneCheck.asALU(aluOpAddS, xzrVReg, rm, operandImm12(1, 0), _64bit)
		m.insert(minusOneCheck)

		ccmp := m.allocateInstr()
		// If eq condition is set, sets the flag by the result based on "rn - 1", otherwise clears the flag.
		ccmp.asCCmpImm(rn, 1, eq, 0, _64bit)
		m.insert(ccmp)

		// Check the overflow flag.
		m.exitIfNot(execCtxVReg, vs.invert().asCond(), false, wazevoapi.ExitCodeIntegerOverflow)
	}
}

// exitIfNot emits a conditional branch to exit if the condition is not met.
// If `c` (cond type) is a register, `cond64bit` must be chosen to indicate whether the register is 32-bit or 64-bit.
// Otherwise, `cond64bit` is ignored.
func (m *machine) exitIfNot(execCtxVReg regalloc.VReg, c cond, cond64bit bool, code wazevoapi.ExitCode) {
	execCtxTmp := m.copyToTmp(execCtxVReg)

	cbr := m.allocateInstr()
	m.insert(cbr)
	m.lowerExitWithCode(execCtxTmp, code)
	// Conditional branch target is after exit.
	l := m.insertBrTargetLabel()
	cbr.asCondBr(c, l, cond64bit)
}

func (m *machine) lowerFcopysign(x, y, ret ssa.Value) {
	rn := m.getOperand_NR(m.compiler.ValueDefinition(x), extModeNone)
	rm := m.getOperand_NR(m.compiler.ValueDefinition(y), extModeNone)
	var tmpI, tmpF regalloc.VReg
	_64 := x.Type() == ssa.TypeF64
	if _64 {
		tmpF = m.compiler.AllocateVReg(ssa.TypeF64)
		tmpI = m.compiler.AllocateVReg(ssa.TypeI64)
	} else {
		tmpF = m.compiler.AllocateVReg(ssa.TypeF32)
		tmpI = m.compiler.AllocateVReg(ssa.TypeI32)
	}
	rd := m.compiler.VRegOf(ret)
	m.lowerFcopysignImpl(rd, rn, rm, tmpI, tmpF, _64)
}

func (m *machine) lowerFcopysignImpl(rd regalloc.VReg, rn, rm operand, tmpI, tmpF regalloc.VReg, _64bit bool) {
	// This is exactly the same code emitted by GCC for "__builtin_copysign":
	//
	//    mov     x0, -9223372036854775808
	//    fmov    d2, x0
	//    vbit    v0.8b, v1.8b, v2.8b
	//

	setMSB := m.allocateInstr()
	if _64bit {
		m.lowerConstantI64(tmpI, math.MinInt64)
		setMSB.asMovToVec(tmpF, operandNR(tmpI), vecArrangementD, vecIndex(0))
	} else {
		m.lowerConstantI32(tmpI, math.MinInt32)
		setMSB.asMovToVec(tmpF, operandNR(tmpI), vecArrangementS, vecIndex(0))
	}
	m.insert(setMSB)

	tmpReg := m.compiler.AllocateVReg(ssa.TypeF64)

	mov := m.allocateInstr()
	mov.asFpuMov64(tmpReg, rn.nr())
	m.insert(mov)

	vbit := m.allocateInstr()
	vbit.asVecRRRRewrite(vecOpBit, tmpReg, rm, operandNR(tmpF), vecArrangement8B)
	m.insert(vbit)

	movDst := m.allocateInstr()
	movDst.asFpuMov64(rd, tmpReg)
	m.insert(movDst)
}

func (m *machine) lowerBitcast(instr *ssa.Instruction) {
	v, dstType := instr.BitcastData()
	srcType := v.Type()
	rn := m.getOperand_NR(m.compiler.ValueDefinition(v), extModeNone)
	rd := m.compiler.VRegOf(instr.Return())
	srcInt := srcType.IsInt()
	dstInt := dstType.IsInt()
	switch {
	case srcInt && !dstInt: // Int to Float:
		mov := m.allocateInstr()
		var arr vecArrangement
		if srcType.Bits() == 64 {
			arr = vecArrangementD
		} else {
			arr = vecArrangementS
		}
		mov.asMovToVec(rd, rn, arr, vecIndex(0))
		m.insert(mov)
	case !srcInt && dstInt: // Float to Int:
		mov := m.allocateInstr()
		var arr vecArrangement
		if dstType.Bits() == 64 {
			arr = vecArrangementD
		} else {
			arr = vecArrangementS
		}
		mov.asMovFromVec(rd, rn, arr, vecIndex(0), false)
		m.insert(mov)
	default:
		panic("TODO?BUG?")
	}
}

func (m *machine) lowerFpuUniOp(op fpuUniOp, in, out ssa.Value) {
	rn := m.getOperand_NR(m.compiler.ValueDefinition(in), extModeNone)
	rd := m.compiler.VRegOf(out)

	neg := m.allocateInstr()
	neg.asFpuRR(op, rd, rn, in.Type().Bits() == 64)
	m.insert(neg)
}

func (m *machine) lowerFpuToInt(rd regalloc.VReg, rn operand, ctx regalloc.VReg, signed, src64bit, dst64bit, nonTrapping bool) {
	if !nonTrapping {
		// First of all, we have to clear the FPU flags.
		flagClear := m.allocateInstr()
		flagClear.asMovToFPSR(xzrVReg)
		m.insert(flagClear)
	}

	// Then, do the conversion which doesn't trap inherently.
	cvt := m.allocateInstr()
	cvt.asFpuToInt(rd, rn, signed, src64bit, dst64bit)
	m.insert(cvt)

	if !nonTrapping {
		tmpReg := m.compiler.AllocateVReg(ssa.TypeI64)

		// After the conversion, check the FPU flags.
		getFlag := m.allocateInstr()
		getFlag.asMovFromFPSR(tmpReg)
		m.insert(getFlag)

		execCtx := m.copyToTmp(ctx)
		_rn := operandNR(m.copyToTmp(rn.nr()))

		// Check if the conversion was undefined by comparing the status with 1.
		// See https://developer.arm.com/documentation/ddi0595/2020-12/AArch64-Registers/FPSR--Floating-point-Status-Register
		alu := m.allocateInstr()
		alu.asALU(aluOpSubS, xzrVReg, operandNR(tmpReg), operandImm12(1, 0), true)
		m.insert(alu)

		// If it is not undefined, we can return the result.
		ok := m.allocateInstr()
		m.insert(ok)

		// Otherwise, we have to choose the status depending on it is overflow or NaN conversion.

		// Comparing itself to check if it is a NaN.
		fpuCmp := m.allocateInstr()
		fpuCmp.asFpuCmp(_rn, _rn, src64bit)
		m.insert(fpuCmp)
		// If the VC flag is not set (== VS flag is set), it is a NaN.
		m.exitIfNot(execCtx, vc.asCond(), false, wazevoapi.ExitCodeInvalidConversionToInteger)
		// Otherwise, it is an overflow.
		m.lowerExitWithCode(execCtx, wazevoapi.ExitCodeIntegerOverflow)

		// Conditional branch target is after exit.
		l := m.insertBrTargetLabel()
		ok.asCondBr(ne.asCond(), l, false /* ignored */)
	}
}

func (m *machine) lowerIntToFpu(rd regalloc.VReg, rn operand, signed, src64bit, dst64bit bool) {
	cvt := m.allocateInstr()
	cvt.asIntToFpu(rd, rn, signed, src64bit, dst64bit)
	m.insert(cvt)
}

func (m *machine) lowerFpuBinOp(si *ssa.Instruction) {
	instr := m.allocateInstr()
	var op fpuBinOp
	switch si.Opcode() {
	case ssa.OpcodeFadd:
		op = fpuBinOpAdd
	case ssa.OpcodeFsub:
		op = fpuBinOpSub
	case ssa.OpcodeFmul:
		op = fpuBinOpMul
	case ssa.OpcodeFdiv:
		op = fpuBinOpDiv
	case ssa.OpcodeFmax:
		op = fpuBinOpMax
	case ssa.OpcodeFmin:
		op = fpuBinOpMin
	}
	x, y := si.Arg2()
	xDef, yDef := m.compiler.ValueDefinition(x), m.compiler.ValueDefinition(y)
	rn := m.getOperand_NR(xDef, extModeNone)
	rm := m.getOperand_NR(yDef, extModeNone)
	rd := m.compiler.VRegOf(si.Return())
	instr.asFpuRRR(op, rd, rn, rm, x.Type().Bits() == 64)
	m.insert(instr)
}

func (m *machine) lowerSubOrAdd(si *ssa.Instruction, add bool) {
	x, y := si.Arg2()
	if !x.Type().IsInt() {
		panic("BUG?")
	}

	xDef, yDef := m.compiler.ValueDefinition(x), m.compiler.ValueDefinition(y)
	rn := m.getOperand_NR(xDef, extModeNone)
	rm, yNegated := m.getOperand_MaybeNegatedImm12_ER_SR_NR(yDef, extModeNone)

	var aop aluOp
	switch {
	case add && !yNegated: // rn+rm = x+y
		aop = aluOpAdd
	case add && yNegated: // rn-rm = x-(-y) = x+y
		aop = aluOpSub
	case !add && !yNegated: // rn-rm = x-y
		aop = aluOpSub
	case !add && yNegated: // rn+rm = x-(-y) = x-y
		aop = aluOpAdd
	}
	rd := m.compiler.VRegOf(si.Return())
	alu := m.allocateInstr()
	alu.asALU(aop, rd, rn, rm, x.Type().Bits() == 64)
	m.insert(alu)
}

// InsertMove implements backend.Machine.
func (m *machine) InsertMove(dst, src regalloc.VReg, typ ssa.Type) {
	instr := m.allocateInstr()
	switch typ {
	case ssa.TypeI32, ssa.TypeI64:
		instr.asMove64(dst, src)
	case ssa.TypeF32, ssa.TypeF64:
		instr.asFpuMov64(dst, src)
	case ssa.TypeV128:
		instr.asFpuMov128(dst, src)
	default:
		panic("TODO")
	}
	m.insert(instr)
}

func (m *machine) lowerIcmp(si *ssa.Instruction) {
	x, y, c := si.IcmpData()
	flag := condFlagFromSSAIntegerCmpCond(c)

	in64bit := x.Type().Bits() == 64
	var ext extMode
	if in64bit {
		if c.Signed() {
			ext = extModeSignExtend64
		} else {
			ext = extModeZeroExtend64
		}
	} else {
		if c.Signed() {
			ext = extModeSignExtend32
		} else {
			ext = extModeZeroExtend32
		}
	}

	rn := m.getOperand_NR(m.compiler.ValueDefinition(x), ext)
	rm := m.getOperand_Imm12_ER_SR_NR(m.compiler.ValueDefinition(y), ext)
	alu := m.allocateInstr()
	alu.asALU(aluOpSubS, xzrVReg, rn, rm, in64bit)
	m.insert(alu)

	cset := m.allocateInstr()
	cset.asCSet(m.compiler.VRegOf(si.Return()), false, flag)
	m.insert(cset)
}

func (m *machine) lowerVIcmp(si *ssa.Instruction) {
	x, y, c, lane := si.VIcmpData()
	flag := condFlagFromSSAIntegerCmpCond(c)
	arr := ssaLaneToArrangement(lane)

	rn := m.getOperand_NR(m.compiler.ValueDefinition(x), extModeNone)
	rm := m.getOperand_NR(m.compiler.ValueDefinition(y), extModeNone)
	rd := m.compiler.VRegOf(si.Return())

	switch flag {
	case eq:
		cmp := m.allocateInstr()
		cmp.asVecRRR(vecOpCmeq, rd, rn, rm, arr)
		m.insert(cmp)
	case ne:
		cmp := m.allocateInstr()
		cmp.asVecRRR(vecOpCmeq, rd, rn, rm, arr)
		m.insert(cmp)
		not := m.allocateInstr()
		not.asVecMisc(vecOpNot, rd, operandNR(rd), vecArrangement16B)
		m.insert(not)
	case ge:
		cmp := m.allocateInstr()
		cmp.asVecRRR(vecOpCmge, rd, rn, rm, arr)
		m.insert(cmp)
	case gt:
		cmp := m.allocateInstr()
		cmp.asVecRRR(vecOpCmgt, rd, rn, rm, arr)
		m.insert(cmp)
	case le:
		cmp := m.allocateInstr()
		cmp.asVecRRR(vecOpCmge, rd, rm, rn, arr) // rm, rn are swapped
		m.insert(cmp)
	case lt:
		cmp := m.allocateInstr()
		cmp.asVecRRR(vecOpCmgt, rd, rm, rn, arr) // rm, rn are swapped
		m.insert(cmp)
	case hs:
		cmp := m.allocateInstr()
		cmp.asVecRRR(vecOpCmhs, rd, rn, rm, arr)
		m.insert(cmp)
	case hi:
		cmp := m.allocateInstr()
		cmp.asVecRRR(vecOpCmhi, rd, rn, rm, arr)
		m.insert(cmp)
	case ls:
		cmp := m.allocateInstr()
		cmp.asVecRRR(vecOpCmhs, rd, rm, rn, arr) // rm, rn are swapped
		m.insert(cmp)
	case lo:
		cmp := m.allocateInstr()
		cmp.asVecRRR(vecOpCmhi, rd, rm, rn, arr) // rm, rn are swapped
		m.insert(cmp)
	}
}

func (m *machine) lowerVFcmp(si *ssa.Instruction) {
	x, y, c, lane := si.VFcmpData()
	flag := condFlagFromSSAFloatCmpCond(c)
	arr := ssaLaneToArrangement(lane)

	rn := m.getOperand_NR(m.compiler.ValueDefinition(x), extModeNone)
	rm := m.getOperand_NR(m.compiler.ValueDefinition(y), extModeNone)
	rd := m.compiler.VRegOf(si.Return())

	switch flag {
	case eq:
		cmp := m.allocateInstr()
		cmp.asVecRRR(vecOpFcmeq, rd, rn, rm, arr)
		m.insert(cmp)
	case ne:
		cmp := m.allocateInstr()
		cmp.asVecRRR(vecOpFcmeq, rd, rn, rm, arr)
		m.insert(cmp)
		not := m.allocateInstr()
		not.asVecMisc(vecOpNot, rd, operandNR(rd), vecArrangement16B)
		m.insert(not)
	case ge:
		cmp := m.allocateInstr()
		cmp.asVecRRR(vecOpFcmge, rd, rn, rm, arr)
		m.insert(cmp)
	case gt:
		cmp := m.allocateInstr()
		cmp.asVecRRR(vecOpFcmgt, rd, rn, rm, arr)
		m.insert(cmp)
	case mi:
		cmp := m.allocateInstr()
		cmp.asVecRRR(vecOpFcmgt, rd, rm, rn, arr) // rm, rn are swapped
		m.insert(cmp)
	case ls:
		cmp := m.allocateInstr()
		cmp.asVecRRR(vecOpFcmge, rd, rm, rn, arr) // rm, rn are swapped
		m.insert(cmp)
	}
}

func (m *machine) lowerVfpuToInt(rd regalloc.VReg, rn operand, arr vecArrangement, signed bool) {
	cvt := m.allocateInstr()
	if signed {
		cvt.asVecMisc(vecOpFcvtzs, rd, rn, arr)
	} else {
		cvt.asVecMisc(vecOpFcvtzu, rd, rn, arr)
	}
	m.insert(cvt)

	if arr == vecArrangement2D {
		narrow := m.allocateInstr()
		if signed {
			narrow.asVecMisc(vecOpSqxtn, rd, operandNR(rd), vecArrangement2S)
		} else {
			narrow.asVecMisc(vecOpUqxtn, rd, operandNR(rd), vecArrangement2S)
		}
		m.insert(narrow)
	}
}

func (m *machine) lowerVfpuFromInt(rd regalloc.VReg, rn operand, arr vecArrangement, signed bool) {
	cvt := m.allocateInstr()
	if signed {
		cvt.asVecMisc(vecOpScvtf, rd, rn, arr)
	} else {
		cvt.asVecMisc(vecOpUcvtf, rd, rn, arr)
	}
	m.insert(cvt)
}

func (m *machine) lowerShifts(si *ssa.Instruction, ext extMode, aluOp aluOp) {
	x, amount := si.Arg2()
	rn := m.getOperand_NR(m.compiler.ValueDefinition(x), ext)
	rm := m.getOperand_ShiftImm_NR(m.compiler.ValueDefinition(amount), ext, x.Type().Bits())
	rd := m.compiler.VRegOf(si.Return())

	alu := m.allocateInstr()
	alu.asALUShift(aluOp, rd, rn, rm, x.Type().Bits() == 64)
	m.insert(alu)
}

func (m *machine) lowerBitwiseAluOp(si *ssa.Instruction, op aluOp, ignoreResult bool) {
	x, y := si.Arg2()

	xDef, yDef := m.compiler.ValueDefinition(x), m.compiler.ValueDefinition(y)
	rn := m.getOperand_NR(xDef, extModeNone)

	var rd regalloc.VReg
	if ignoreResult {
		rd = xzrVReg
	} else {
		rd = m.compiler.VRegOf(si.Return())
	}

	_64 := x.Type().Bits() == 64
	alu := m.allocateInstr()
	if instr := yDef.Instr; instr != nil && instr.Constant() {
		c := instr.ConstantVal()
		if isBitMaskImmediate(c, _64) {
			// Constant bit wise operations can be lowered to a single instruction.
			alu.asALUBitmaskImm(op, rd, rn.nr(), c, _64)
			m.insert(alu)
			return
		}
	}

	rm := m.getOperand_SR_NR(yDef, extModeNone)
	alu.asALU(op, rd, rn, rm, _64)
	m.insert(alu)
}

func (m *machine) lowerRotl(si *ssa.Instruction) {
	x, y := si.Arg2()
	r := si.Return()
	_64 := r.Type().Bits() == 64

	rn := m.getOperand_NR(m.compiler.ValueDefinition(x), extModeNone)
	rm := m.getOperand_NR(m.compiler.ValueDefinition(y), extModeNone)
	var tmp regalloc.VReg
	if _64 {
		tmp = m.compiler.AllocateVReg(ssa.TypeI64)
	} else {
		tmp = m.compiler.AllocateVReg(ssa.TypeI32)
	}
	rd := m.compiler.VRegOf(r)

	// Encode rotl as neg + rotr: neg is a sub against the zero-reg.
	m.lowerRotlImpl(rd, rn, rm, tmp, _64)
}

func (m *machine) lowerRotlImpl(rd regalloc.VReg, rn, rm operand, tmp regalloc.VReg, is64bit bool) {
	// Encode rotl as neg + rotr: neg is a sub against the zero-reg.
	neg := m.allocateInstr()
	neg.asALU(aluOpSub, tmp, operandNR(xzrVReg), rm, is64bit)
	m.insert(neg)
	alu := m.allocateInstr()
	alu.asALU(aluOpRotR, rd, rn, operandNR(tmp), is64bit)
	m.insert(alu)
}

func (m *machine) lowerRotr(si *ssa.Instruction) {
	x, y := si.Arg2()

	xDef, yDef := m.compiler.ValueDefinition(x), m.compiler.ValueDefinition(y)
	rn := m.getOperand_NR(xDef, extModeNone)
	rm := m.getOperand_NR(yDef, extModeNone)
	rd := m.compiler.VRegOf(si.Return())

	alu := m.allocateInstr()
	alu.asALU(aluOpRotR, rd, rn, rm, si.Return().Type().Bits() == 64)
	m.insert(alu)
}

func (m *machine) lowerExtend(arg, ret ssa.Value, from, to byte, signed bool) {
	rd := m.compiler.VRegOf(ret)
	def := m.compiler.ValueDefinition(arg)

	if instr := def.Instr; !signed && from == 32 && instr != nil {
		// We can optimize out the unsigned extend because:
		// 	Writes to the W register set bits [63:32] of the X register to zero
		//  https://developer.arm.com/documentation/den0024/a/An-Introduction-to-the-ARMv8-Instruction-Sets/The-ARMv8-instruction-sets/Distinguishing-between-32-bit-and-64-bit-A64-instructions
		switch instr.Opcode() {
		case
			ssa.OpcodeIadd, ssa.OpcodeIsub, ssa.OpcodeLoad,
			ssa.OpcodeBand, ssa.OpcodeBor, ssa.OpcodeBnot,
			ssa.OpcodeIshl, ssa.OpcodeUshr, ssa.OpcodeSshr,
			ssa.OpcodeRotl, ssa.OpcodeRotr,
			ssa.OpcodeUload8, ssa.OpcodeUload16, ssa.OpcodeUload32:
			// So, if the argument is the result of a 32-bit operation, we can just copy the register.
			// It is highly likely that this copy will be optimized out after register allocation.
			rn := m.compiler.VRegOf(arg)
			mov := m.allocateInstr()
			// Note: do not use move32 as it will be lowered to a 32-bit move, which is not copy (that is actually the impl of UExtend).
			mov.asMove64(rd, rn)
			m.insert(mov)
			return
		default:
		}
	}
	rn := m.getOperand_NR(def, extModeNone)

	ext := m.allocateInstr()
	ext.asExtend(rd, rn.nr(), from, to, signed)
	m.insert(ext)
}

func (m *machine) lowerFcmp(x, y, result ssa.Value, c ssa.FloatCmpCond) {
	rn, rm := m.getOperand_NR(m.compiler.ValueDefinition(x), extModeNone), m.getOperand_NR(m.compiler.ValueDefinition(y), extModeNone)

	fc := m.allocateInstr()
	fc.asFpuCmp(rn, rm, x.Type().Bits() == 64)
	m.insert(fc)

	cset := m.allocateInstr()
	cset.asCSet(m.compiler.VRegOf(result), false, condFlagFromSSAFloatCmpCond(c))
	m.insert(cset)
}

func (m *machine) lowerImul(x, y, result ssa.Value) {
	rd := m.compiler.VRegOf(result)
	rn := m.getOperand_NR(m.compiler.ValueDefinition(x), extModeNone)
	rm := m.getOperand_NR(m.compiler.ValueDefinition(y), extModeNone)

	// TODO: if this comes before Add/Sub, we could merge it by putting it into the place of xzrVReg.

	mul := m.allocateInstr()
	mul.asALURRRR(aluOpMAdd, rd, rn, rm, xzrVReg, x.Type().Bits() == 64)
	m.insert(mul)
}

func (m *machine) lowerClz(x, result ssa.Value) {
	rd := m.compiler.VRegOf(result)
	rn := m.getOperand_NR(m.compiler.ValueDefinition(x), extModeNone)
	clz := m.allocateInstr()
	clz.asBitRR(bitOpClz, rd, rn.nr(), x.Type().Bits() == 64)
	m.insert(clz)
}

func (m *machine) lowerCtz(x, result ssa.Value) {
	rd := m.compiler.VRegOf(result)
	rn := m.getOperand_NR(m.compiler.ValueDefinition(x), extModeNone)
	rbit := m.allocateInstr()
	_64 := x.Type().Bits() == 64
	var tmpReg regalloc.VReg
	if _64 {
		tmpReg = m.compiler.AllocateVReg(ssa.TypeI64)
	} else {
		tmpReg = m.compiler.AllocateVReg(ssa.TypeI32)
	}
	rbit.asBitRR(bitOpRbit, tmpReg, rn.nr(), _64)
	m.insert(rbit)

	clz := m.allocateInstr()
	clz.asBitRR(bitOpClz, rd, tmpReg, _64)
	m.insert(clz)
}

func (m *machine) lowerPopcnt(x, result ssa.Value) {
	// arm64 doesn't have an instruction for population count on scalar register,
	// so we use the vector instruction `cnt`.
	// This is exactly what the official Go implements bits.OneCount.
	// For example, "func () int { return bits.OneCount(10) }" is compiled as
	//
	//    MOVD    $10, R0 ;; Load 10.
	//    FMOVD   R0, F0
	//    VCNT    V0.B8, V0.B8
	//    UADDLV  V0.B8, V0
	//
	// In aarch64 asm, FMOVD is encoded as `ins`, VCNT is `cnt`,
	// and the registers may use different names. In our encoding we use the following
	// instructions:
	//
	//    ins v0.d[0], x0     ;; mov from GPR to vec (FMOV above) is encoded as INS
	//    cnt v0.16b, v0.16b  ;; we use vec arrangement 16b
	//    uaddlv h0, v0.8b    ;; h0 is still v0 with the dest width specifier 'H', implied when src arrangement is 8b
	//    mov x5, v0.d[0]     ;; finally we mov the result back to a GPR
	//

	rd := m.compiler.VRegOf(result)
	rn := m.getOperand_NR(m.compiler.ValueDefinition(x), extModeNone)

	rf1 := operandNR(m.compiler.AllocateVReg(ssa.TypeF64))
	ins := m.allocateInstr()
	ins.asMovToVec(rf1.nr(), rn, vecArrangementD, vecIndex(0))
	m.insert(ins)

	rf2 := operandNR(m.compiler.AllocateVReg(ssa.TypeF64))
	cnt := m.allocateInstr()
	cnt.asVecMisc(vecOpCnt, rf2.nr(), rf1, vecArrangement16B)
	m.insert(cnt)

	rf3 := operandNR(m.compiler.AllocateVReg(ssa.TypeF64))
	uaddlv := m.allocateInstr()
	uaddlv.asVecLanes(vecOpUaddlv, rf3.nr(), rf2, vecArrangement8B)
	m.insert(uaddlv)

	mov := m.allocateInstr()
	mov.asMovFromVec(rd, rf3, vecArrangementD, vecIndex(0), false)
	m.insert(mov)
}

// lowerExitWithCode lowers the lowerExitWithCode takes a context pointer as argument.
func (m *machine) lowerExitWithCode(execCtxVReg regalloc.VReg, code wazevoapi.ExitCode) {
	tmpReg1 := m.compiler.AllocateVReg(ssa.TypeI32)
	loadExitCodeConst := m.allocateInstr()
	loadExitCodeConst.asMOVZ(tmpReg1, uint64(code), 0, true)

	setExitCode := m.allocateInstr()
	mode := m.amodePool.Allocate()
	*mode = addressMode{
		kind: addressModeKindRegUnsignedImm12,
		rn:   execCtxVReg, imm: wazevoapi.ExecutionContextOffsetExitCodeOffset.I64(),
	}
	setExitCode.asStore(operandNR(tmpReg1), mode, 32)

	// In order to unwind the stack, we also need to push the current stack pointer:
	tmp2 := m.compiler.AllocateVReg(ssa.TypeI64)
	movSpToTmp := m.allocateInstr()
	movSpToTmp.asMove64(tmp2, spVReg)
	strSpToExecCtx := m.allocateInstr()
	mode2 := m.amodePool.Allocate()
	*mode2 = addressMode{
		kind: addressModeKindRegUnsignedImm12,
		rn:   execCtxVReg, imm: wazevoapi.ExecutionContextOffsetStackPointerBeforeGoCall.I64(),
	}
	strSpToExecCtx.asStore(operandNR(tmp2), mode2, 64)
	// Also the address of this exit.
	tmp3 := m.compiler.AllocateVReg(ssa.TypeI64)
	currentAddrToTmp := m.allocateInstr()
	currentAddrToTmp.asAdr(tmp3, 0)
	storeCurrentAddrToExecCtx := m.allocateInstr()
	mode3 := m.amodePool.Allocate()
	*mode3 = addressMode{
		kind: addressModeKindRegUnsignedImm12,
		rn:   execCtxVReg, imm: wazevoapi.ExecutionContextOffsetGoCallReturnAddress.I64(),
	}
	storeCurrentAddrToExecCtx.asStore(operandNR(tmp3), mode3, 64)

	exitSeq := m.allocateInstr()
	exitSeq.asExitSequence(execCtxVReg)

	m.insert(loadExitCodeConst)
	m.insert(setExitCode)
	m.insert(movSpToTmp)
	m.insert(strSpToExecCtx)
	m.insert(currentAddrToTmp)
	m.insert(storeCurrentAddrToExecCtx)
	m.insert(exitSeq)
}

func (m *machine) lowerIcmpToFlag(x, y ssa.Value, signed bool) {
	if x.Type() != y.Type() {
		panic(
			fmt.Sprintf("TODO(maybe): support icmp with different types: v%d=%s != v%d=%s",
				x.ID(), x.Type(), y.ID(), y.Type()))
	}

	extMod := extModeOf(x.Type(), signed)

	// First operand must be in pure register form.
	rn := m.getOperand_NR(m.compiler.ValueDefinition(x), extMod)
	// Second operand can be in any of Imm12, ER, SR, or NR form supported by the SUBS instructions.
	rm := m.getOperand_Imm12_ER_SR_NR(m.compiler.ValueDefinition(y), extMod)

	alu := m.allocateInstr()
	// subs zr, rn, rm
	alu.asALU(
		aluOpSubS,
		// We don't need the result, just need to set flags.
		xzrVReg,
		rn,
		rm,
		x.Type().Bits() == 64,
	)
	m.insert(alu)
}

func (m *machine) lowerFcmpToFlag(x, y ssa.Value) {
	if x.Type() != y.Type() {
		panic("TODO(maybe): support icmp with different types")
	}

	rn := m.getOperand_NR(m.compiler.ValueDefinition(x), extModeNone)
	rm := m.getOperand_NR(m.compiler.ValueDefinition(y), extModeNone)
	cmp := m.allocateInstr()
	cmp.asFpuCmp(rn, rm, x.Type().Bits() == 64)
	m.insert(cmp)
}

func (m *machine) lowerExitIfTrueWithCode(execCtxVReg regalloc.VReg, cond ssa.Value, code wazevoapi.ExitCode) {
	condDef := m.compiler.ValueDefinition(cond)
	if !m.compiler.MatchInstr(condDef, ssa.OpcodeIcmp) {
		panic("TODO: OpcodeExitIfTrueWithCode must come after Icmp at the moment: " + condDef.Instr.Opcode().String())
	}
	condDef.Instr.MarkLowered()

	cvalInstr := condDef.Instr
	x, y, c := cvalInstr.IcmpData()
	signed := c.Signed()

	if !m.tryLowerBandToFlag(x, y) {
		m.lowerIcmpToFlag(x, y, signed)
	}

	// We need to copy the execution context to a temp register, because if it's spilled,
	// it might end up being reloaded inside the exiting branch.
	execCtxTmp := m.copyToTmp(execCtxVReg)

	// We have to skip the entire exit sequence if the condition is false.
	cbr := m.allocateInstr()
	m.insert(cbr)
	m.lowerExitWithCode(execCtxTmp, code)
	// conditional branch target is after exit.
	l := m.insertBrTargetLabel()
	cbr.asCondBr(condFlagFromSSAIntegerCmpCond(c).invert().asCond(), l, false /* ignored */)
}

func (m *machine) lowerSelect(c, x, y, result ssa.Value) {
	cvalDef := m.compiler.ValueDefinition(c)

	var cc condFlag
	switch {
	case m.compiler.MatchInstr(cvalDef, ssa.OpcodeIcmp): // This case, we can use the ALU flag set by SUBS instruction.
		cvalInstr := cvalDef.Instr
		x, y, c := cvalInstr.IcmpData()
		cc = condFlagFromSSAIntegerCmpCond(c)
		m.lowerIcmpToFlag(x, y, c.Signed())
		cvalDef.Instr.MarkLowered()
	case m.compiler.MatchInstr(cvalDef, ssa.OpcodeFcmp): // This case we can use the Fpu flag directly.
		cvalInstr := cvalDef.Instr
		x, y, c := cvalInstr.FcmpData()
		cc = condFlagFromSSAFloatCmpCond(c)
		m.lowerFcmpToFlag(x, y)
		cvalDef.Instr.MarkLowered()
	default:
		rn := m.getOperand_NR(cvalDef, extModeNone)
		if c.Type() != ssa.TypeI32 && c.Type() != ssa.TypeI64 {
			panic("TODO?BUG?: support select with non-integer condition")
		}
		alu := m.allocateInstr()
		// subs zr, rn, zr
		alu.asALU(
			aluOpSubS,
			// We don't need the result, just need to set flags.
			xzrVReg,
			rn,
			operandNR(xzrVReg),
			c.Type().Bits() == 64,
		)
		m.insert(alu)
		cc = ne
	}

	rn := m.getOperand_NR(m.compiler.ValueDefinition(x), extModeNone)
	rm := m.getOperand_NR(m.compiler.ValueDefinition(y), extModeNone)

	rd := m.compiler.VRegOf(result)
	switch x.Type() {
	case ssa.TypeI32, ssa.TypeI64:
		// csel rd, rn, rm, cc
		csel := m.allocateInstr()
		csel.asCSel(rd, rn, rm, cc, x.Type().Bits() == 64)
		m.insert(csel)
	case ssa.TypeF32, ssa.TypeF64:
		// fcsel rd, rn, rm, cc
		fcsel := m.allocateInstr()
		fcsel.asFpuCSel(rd, rn, rm, cc, x.Type().Bits() == 64)
		m.insert(fcsel)
	default:
		panic("BUG")
	}
}

func (m *machine) lowerSelectVec(rc, rn, rm operand, rd regalloc.VReg) {
	// First check if `rc` is zero or not.
	checkZero := m.allocateInstr()
	checkZero.asALU(aluOpSubS, xzrVReg, rc, operandNR(xzrVReg), false)
	m.insert(checkZero)

	// Then use CSETM to set all bits to one if `rc` is zero.
	allOnesOrZero := m.compiler.AllocateVReg(ssa.TypeI64)
	cset := m.allocateInstr()
	cset.asCSet(allOnesOrZero, true, ne)
	m.insert(cset)

	// Then move the bits to the result vector register.
	tmp2 := m.compiler.AllocateVReg(ssa.TypeV128)
	dup := m.allocateInstr()
	dup.asVecDup(tmp2, operandNR(allOnesOrZero), vecArrangement2D)
	m.insert(dup)

	// Now that `tmp2` has either all bits one or zero depending on `rc`,
	// we can use bsl to select between `rn` and `rm`.
	ins := m.allocateInstr()
	ins.asVecRRRRewrite(vecOpBsl, tmp2, rn, rm, vecArrangement16B)
	m.insert(ins)

	// Finally, move the result to the destination register.
	mov2 := m.allocateInstr()
	mov2.asFpuMov128(rd, tmp2)
	m.insert(mov2)
}

func (m *machine) lowerAtomicRmw(si *ssa.Instruction) {
	ssaOp, size := si.AtomicRmwData()

	var op atomicRmwOp
	var negateArg bool
	var flipArg bool
	switch ssaOp {
	case ssa.AtomicRmwOpAdd:
		op = atomicRmwOpAdd
	case ssa.AtomicRmwOpSub:
		op = atomicRmwOpAdd
		negateArg = true
	case ssa.AtomicRmwOpAnd:
		op = atomicRmwOpClr
		flipArg = true
	case ssa.AtomicRmwOpOr:
		op = atomicRmwOpSet
	case ssa.AtomicRmwOpXor:
		op = atomicRmwOpEor
	case ssa.AtomicRmwOpXchg:
		op = atomicRmwOpSwp
	default:
		panic(fmt.Sprintf("unknown ssa atomic rmw op: %s", ssaOp))
	}

	addr, val := si.Arg2()
	addrDef, valDef := m.compiler.ValueDefinition(addr), m.compiler.ValueDefinition(val)
	rn := m.getOperand_NR(addrDef, extModeNone)
	rt := m.compiler.VRegOf(si.Return())
	rs := m.getOperand_NR(valDef, extModeNone)

	_64 := si.Return().Type().Bits() == 64
	var tmp regalloc.VReg
	if _64 {
		tmp = m.compiler.AllocateVReg(ssa.TypeI64)
	} else {
		tmp = m.compiler.AllocateVReg(ssa.TypeI32)
	}
	m.lowerAtomicRmwImpl(op, rn.nr(), rs.nr(), rt, tmp, size, negateArg, flipArg, _64)
}

func (m *machine) lowerAtomicRmwImpl(op atomicRmwOp, rn, rs, rt, tmp regalloc.VReg, size uint64, negateArg, flipArg, dst64bit bool) {
	switch {
	case negateArg:
		neg := m.allocateInstr()
		neg.asALU(aluOpSub, tmp, operandNR(xzrVReg), operandNR(rs), dst64bit)
		m.insert(neg)
	case flipArg:
		flip := m.allocateInstr()
		flip.asALU(aluOpOrn, tmp, operandNR(xzrVReg), operandNR(rs), dst64bit)
		m.insert(flip)
	default:
		tmp = rs
	}

	rmw := m.allocateInstr()
	rmw.asAtomicRmw(op, rn, tmp, rt, size)
	m.insert(rmw)
}

func (m *machine) lowerAtomicCas(si *ssa.Instruction) {
	addr, exp, repl := si.Arg3()
	size := si.AtomicTargetSize()

	addrDef, expDef, replDef := m.compiler.ValueDefinition(addr), m.compiler.ValueDefinition(exp), m.compiler.ValueDefinition(repl)
	rn := m.getOperand_NR(addrDef, extModeNone)
	rt := m.getOperand_NR(replDef, extModeNone)
	rs := m.getOperand_NR(expDef, extModeNone)
	tmp := m.compiler.AllocateVReg(si.Return().Type())

	_64 := si.Return().Type().Bits() == 64
	// rs is overwritten by CAS, so we need to move it to the result register before the instruction
	// in case when it is used somewhere else.
	mov := m.allocateInstr()
	if _64 {
		mov.asMove64(tmp, rs.nr())
	} else {
		mov.asMove32(tmp, rs.nr())
	}
	m.insert(mov)

	m.lowerAtomicCasImpl(rn.nr(), tmp, rt.nr(), size)

	mov2 := m.allocateInstr()
	rd := m.compiler.VRegOf(si.Return())
	if _64 {
		mov2.asMove64(rd, tmp)
	} else {
		mov2.asMove32(rd, tmp)
	}
	m.insert(mov2)
}

func (m *machine) lowerAtomicCasImpl(rn, rs, rt regalloc.VReg, size uint64) {
	cas := m.allocateInstr()
	cas.asAtomicCas(rn, rs, rt, size)
	m.insert(cas)
}

func (m *machine) lowerAtomicLoad(si *ssa.Instruction) {
	addr := si.Arg()
	size := si.AtomicTargetSize()

	addrDef := m.compiler.ValueDefinition(addr)
	rn := m.getOperand_NR(addrDef, extModeNone)
	rt := m.compiler.VRegOf(si.Return())

	m.lowerAtomicLoadImpl(rn.nr(), rt, size)
}

func (m *machine) lowerAtomicLoadImpl(rn, rt regalloc.VReg, size uint64) {
	ld := m.allocateInstr()
	ld.asAtomicLoad(rn, rt, size)
	m.insert(ld)
}

func (m *machine) lowerAtomicStore(si *ssa.Instruction) {
	addr, val := si.Arg2()
	size := si.AtomicTargetSize()

	addrDef := m.compiler.ValueDefinition(addr)
	valDef := m.compiler.ValueDefinition(val)
	rn := m.getOperand_NR(addrDef, extModeNone)
	rt := m.getOperand_NR(valDef, extModeNone)

	m.lowerAtomicStoreImpl(rn, rt, size)
}

func (m *machine) lowerAtomicStoreImpl(rn, rt operand, size uint64) {
	ld := m.allocateInstr()
	ld.asAtomicStore(rn, rt, size)
	m.insert(ld)
}

// copyToTmp copies the given regalloc.VReg to a temporary register. This is called before cbr to avoid the regalloc issue
// e.g. reload happening in the middle of the exit sequence which is not the path the normal path executes
func (m *machine) copyToTmp(v regalloc.VReg) regalloc.VReg {
	typ := m.compiler.TypeOf(v)
	mov := m.allocateInstr()
	tmp := m.compiler.AllocateVReg(typ)
	if typ.IsInt() {
		mov.asMove64(tmp, v)
	} else {
		mov.asFpuMov128(tmp, v)
	}
	m.insert(mov)
	return tmp
}
