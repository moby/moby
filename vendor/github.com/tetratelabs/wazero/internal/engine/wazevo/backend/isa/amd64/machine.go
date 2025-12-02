package amd64

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"strings"

	"github.com/tetratelabs/wazero/internal/engine/wazevo/backend"
	"github.com/tetratelabs/wazero/internal/engine/wazevo/backend/regalloc"
	"github.com/tetratelabs/wazero/internal/engine/wazevo/ssa"
	"github.com/tetratelabs/wazero/internal/engine/wazevo/wazevoapi"
	"github.com/tetratelabs/wazero/internal/platform"
)

// NewBackend returns a new backend for arm64.
func NewBackend() backend.Machine {
	m := &machine{
		cpuFeatures:                         platform.CpuFeatures,
		regAlloc:                            regalloc.NewAllocator[*instruction, *labelPosition, *regAllocFn](regInfo),
		spillSlots:                          map[regalloc.VRegID]int64{},
		amodePool:                           wazevoapi.NewPool[amode](nil),
		labelPositionPool:                   wazevoapi.NewIDedPool[labelPosition](resetLabelPosition),
		instrPool:                           wazevoapi.NewPool[instruction](resetInstruction),
		constSwizzleMaskConstIndex:          -1,
		constSqmulRoundSatIndex:             -1,
		constI8x16SHLMaskTableIndex:         -1,
		constI8x16LogicalSHRMaskTableIndex:  -1,
		constF64x2CvtFromIMaskIndex:         -1,
		constTwop52Index:                    -1,
		constI32sMaxOnF64x2Index:            -1,
		constI32uMaxOnF64x2Index:            -1,
		constAllOnesI8x16Index:              -1,
		constAllOnesI16x8Index:              -1,
		constExtAddPairwiseI16x8uMask1Index: -1,
		constExtAddPairwiseI16x8uMask2Index: -1,
	}
	m.regAllocFn.m = m
	return m
}

type (
	// machine implements backend.Machine for amd64.
	machine struct {
		c                        backend.Compiler
		stackBoundsCheckDisabled bool

		instrPool wazevoapi.Pool[instruction]
		amodePool wazevoapi.Pool[amode]

		cpuFeatures platform.CpuFeatureFlags

		regAlloc        regalloc.Allocator[*instruction, *labelPosition, *regAllocFn]
		regAllocFn      regAllocFn
		regAllocStarted bool

		// labelPositionPool is the pool of labelPosition. The id is the label where
		// if the label is less than the maxSSABlockID, it's the ssa.BasicBlockID.
		labelPositionPool wazevoapi.IDedPool[labelPosition]
		// nextLabel is the next label to be allocated. The first free label comes after maxSSABlockID
		// so that we can have an identical label for the SSA block ID, which is useful for debugging.
		nextLabel label
		// rootInstr is the first instruction of the function.
		rootInstr *instruction
		// currentLabelPos is the currently-compiled ssa.BasicBlock's labelPosition.
		currentLabelPos *labelPosition
		// orderedSSABlockLabelPos is the ordered list of labelPosition in the generated code for each ssa.BasicBlock.
		orderedSSABlockLabelPos []*labelPosition
		// returnLabelPos is the labelPosition for the return block.
		returnLabelPos labelPosition
		// perBlockHead and perBlockEnd are the head and tail of the instruction list per currently-compiled ssa.BasicBlock.
		perBlockHead, perBlockEnd *instruction
		// pendingInstructions are the instructions which are not yet emitted into the instruction list.
		pendingInstructions []*instruction
		// maxSSABlockID is the maximum ssa.BasicBlockID in the current function.
		maxSSABlockID label

		spillSlotSize int64
		spillSlots    map[regalloc.VRegID]int64
		currentABI    *backend.FunctionABI
		clobberedRegs []regalloc.VReg

		maxRequiredStackSizeForCalls int64

		labelResolutionPends []labelResolutionPend

		// jmpTableTargets holds the labels of the jump table targets.
		jmpTableTargets [][]uint32
		// jmpTableTargetNext is the index to the jmpTableTargets slice to be used for the next jump table.
		jmpTableTargetsNext int
		consts              []_const

		constSwizzleMaskConstIndex, constSqmulRoundSatIndex,
		constI8x16SHLMaskTableIndex, constI8x16LogicalSHRMaskTableIndex,
		constF64x2CvtFromIMaskIndex, constTwop52Index,
		constI32sMaxOnF64x2Index, constI32uMaxOnF64x2Index,
		constAllOnesI8x16Index, constAllOnesI16x8Index,
		constExtAddPairwiseI16x8uMask1Index, constExtAddPairwiseI16x8uMask2Index int
	}

	_const struct {
		lo, hi   uint64
		_var     []byte
		label    label
		labelPos *labelPosition
	}

	labelResolutionPend struct {
		instr       *instruction
		instrOffset int64
		// imm32Offset is the offset of the last 4 bytes of the instruction.
		imm32Offset int64
	}
)

type (
	// label represents a position in the generated code which is either
	// a real instruction or the constant InstructionPool (e.g. jump tables).
	//
	// This is exactly the same as the traditional "label" in assembly code.
	label uint32

	// labelPosition represents the regions of the generated code which the label represents.
	// This implements regalloc.Block.
	labelPosition struct {
		// sb is not nil if this corresponds to a ssa.BasicBlock.
		sb ssa.BasicBlock
		// cur is used to walk through the instructions in the block during the register allocation.
		cur,
		// begin and end are the first and last instructions of the block.
		begin, end *instruction
		// binaryOffset is the offset in the binary where the label is located.
		binaryOffset int64
	}
)

// String implements backend.Machine.
func (l label) String() string {
	return fmt.Sprintf("L%d", l)
}

func resetLabelPosition(l *labelPosition) {
	*l = labelPosition{}
}

const labelReturn = math.MaxUint32

func ssaBlockLabel(sb ssa.BasicBlock) label {
	if sb.ReturnBlock() {
		return labelReturn
	}
	return label(sb.ID())
}

// getOrAllocateSSABlockLabelPosition returns the labelPosition for the given basic block.
func (m *machine) getOrAllocateSSABlockLabelPosition(sb ssa.BasicBlock) *labelPosition {
	if sb.ReturnBlock() {
		m.returnLabelPos.sb = sb
		return &m.returnLabelPos
	}

	l := ssaBlockLabel(sb)
	pos := m.labelPositionPool.GetOrAllocate(int(l))
	pos.sb = sb
	return pos
}

func (m *machine) getOrAllocateConstLabel(i *int, _var []byte) label {
	index := *i
	if index == -1 {
		l, pos := m.allocateLabel()
		index = len(m.consts)
		m.consts = append(m.consts, _const{
			_var:     _var,
			label:    l,
			labelPos: pos,
		})
		*i = index
	}
	return m.consts[index].label
}

// Reset implements backend.Machine.
func (m *machine) Reset() {
	m.consts = m.consts[:0]
	m.clobberedRegs = m.clobberedRegs[:0]
	for key := range m.spillSlots {
		m.clobberedRegs = append(m.clobberedRegs, regalloc.VReg(key))
	}
	for _, key := range m.clobberedRegs {
		delete(m.spillSlots, regalloc.VRegID(key))
	}

	m.stackBoundsCheckDisabled = false
	m.regAlloc.Reset()
	m.labelPositionPool.Reset()
	m.instrPool.Reset()
	m.regAllocStarted = false
	m.clobberedRegs = m.clobberedRegs[:0]

	m.spillSlotSize = 0
	m.maxRequiredStackSizeForCalls = 0
	m.perBlockHead, m.perBlockEnd, m.rootInstr = nil, nil, nil
	m.pendingInstructions = m.pendingInstructions[:0]
	m.orderedSSABlockLabelPos = m.orderedSSABlockLabelPos[:0]

	m.amodePool.Reset()
	m.jmpTableTargetsNext = 0
	m.constSwizzleMaskConstIndex = -1
	m.constSqmulRoundSatIndex = -1
	m.constI8x16SHLMaskTableIndex = -1
	m.constI8x16LogicalSHRMaskTableIndex = -1
	m.constF64x2CvtFromIMaskIndex = -1
	m.constTwop52Index = -1
	m.constI32sMaxOnF64x2Index = -1
	m.constI32uMaxOnF64x2Index = -1
	m.constAllOnesI8x16Index = -1
	m.constAllOnesI16x8Index = -1
	m.constExtAddPairwiseI16x8uMask1Index = -1
	m.constExtAddPairwiseI16x8uMask2Index = -1
}

// StartLoweringFunction implements backend.Machine StartLoweringFunction.
func (m *machine) StartLoweringFunction(maxBlockID ssa.BasicBlockID) {
	m.maxSSABlockID = label(maxBlockID)
	m.nextLabel = label(maxBlockID) + 1
}

// LinkAdjacentBlocks implements backend.Machine.
func (m *machine) LinkAdjacentBlocks(prev, next ssa.BasicBlock) {
	prevPos, nextPos := m.getOrAllocateSSABlockLabelPosition(prev), m.getOrAllocateSSABlockLabelPosition(next)
	prevPos.end.next = nextPos.begin
}

// StartBlock implements backend.Machine.
func (m *machine) StartBlock(blk ssa.BasicBlock) {
	m.currentLabelPos = m.getOrAllocateSSABlockLabelPosition(blk)
	labelPos := m.currentLabelPos
	end := m.allocateNop()
	m.perBlockHead, m.perBlockEnd = end, end
	labelPos.begin, labelPos.end = end, end
	m.orderedSSABlockLabelPos = append(m.orderedSSABlockLabelPos, labelPos)
}

// EndBlock implements ExecutableContext.
func (m *machine) EndBlock() {
	// Insert nop0 as the head of the block for convenience to simplify the logic of inserting instructions.
	m.insertAtPerBlockHead(m.allocateNop())

	m.currentLabelPos.begin = m.perBlockHead

	if m.currentLabelPos.sb.EntryBlock() {
		m.rootInstr = m.perBlockHead
	}
}

func (m *machine) insertAtPerBlockHead(i *instruction) {
	if m.perBlockHead == nil {
		m.perBlockHead = i
		m.perBlockEnd = i
		return
	}

	i.next = m.perBlockHead
	m.perBlockHead.prev = i
	m.perBlockHead = i
}

// FlushPendingInstructions implements backend.Machine.
func (m *machine) FlushPendingInstructions() {
	l := len(m.pendingInstructions)
	if l == 0 {
		return
	}
	for i := l - 1; i >= 0; i-- { // reverse because we lower instructions in reverse order.
		m.insertAtPerBlockHead(m.pendingInstructions[i])
	}
	m.pendingInstructions = m.pendingInstructions[:0]
}

// DisableStackCheck implements backend.Machine.
func (m *machine) DisableStackCheck() { m.stackBoundsCheckDisabled = true }

// SetCompiler implements backend.Machine.
func (m *machine) SetCompiler(c backend.Compiler) {
	m.c = c
	m.regAllocFn.ssaB = c.SSABuilder()
}

// SetCurrentABI implements backend.Machine.
func (m *machine) SetCurrentABI(abi *backend.FunctionABI) { m.currentABI = abi }

// RegAlloc implements backend.Machine.
func (m *machine) RegAlloc() {
	rf := m.regAllocFn
	m.regAllocStarted = true
	m.regAlloc.DoAllocation(&rf)
	// Now that we know the final spill slot size, we must align spillSlotSize to 16 bytes.
	m.spillSlotSize = (m.spillSlotSize + 15) &^ 15
}

// InsertReturn implements backend.Machine.
func (m *machine) InsertReturn() {
	i := m.allocateInstr().asRet()
	m.insert(i)
}

// LowerSingleBranch implements backend.Machine.
func (m *machine) LowerSingleBranch(b *ssa.Instruction) {
	switch b.Opcode() {
	case ssa.OpcodeJump:
		_, _, targetBlkID := b.BranchData()
		if b.IsFallthroughJump() {
			return
		}
		jmp := m.allocateInstr()
		target := ssaBlockLabel(m.c.SSABuilder().BasicBlock(targetBlkID))
		if target == labelReturn {
			jmp.asRet()
		} else {
			jmp.asJmp(newOperandLabel(target))
		}
		m.insert(jmp)
	case ssa.OpcodeBrTable:
		index, targetBlkIDs := b.BrTableData()
		m.lowerBrTable(index, targetBlkIDs)
	default:
		panic("BUG: unexpected branch opcode" + b.Opcode().String())
	}
}

func (m *machine) addJmpTableTarget(targets ssa.Values) (index int) {
	if m.jmpTableTargetsNext == len(m.jmpTableTargets) {
		m.jmpTableTargets = append(m.jmpTableTargets, make([]uint32, 0, len(targets.View())))
	}

	index = m.jmpTableTargetsNext
	m.jmpTableTargetsNext++
	m.jmpTableTargets[index] = m.jmpTableTargets[index][:0]
	for _, targetBlockID := range targets.View() {
		target := m.c.SSABuilder().BasicBlock(ssa.BasicBlockID(targetBlockID))
		m.jmpTableTargets[index] = append(m.jmpTableTargets[index], uint32(ssaBlockLabel(target)))
	}
	return
}

var condBranchMatches = [...]ssa.Opcode{ssa.OpcodeIcmp, ssa.OpcodeFcmp}

func (m *machine) lowerBrTable(index ssa.Value, targets ssa.Values) {
	_v := m.getOperand_Reg(m.c.ValueDefinition(index))
	v := m.copyToTmp(_v.reg())

	targetCount := len(targets.View())

	// First, we need to do the bounds check.
	maxIndex := m.c.AllocateVReg(ssa.TypeI32)
	m.lowerIconst(maxIndex, uint64(targetCount-1), false)
	cmp := m.allocateInstr().asCmpRmiR(true, newOperandReg(maxIndex), v, false)
	m.insert(cmp)

	// Then do the conditional move maxIndex to v if v > maxIndex.
	cmov := m.allocateInstr().asCmove(condNB, newOperandReg(maxIndex), v, false)
	m.insert(cmov)

	// Now that v has the correct index. Load the address of the jump table into the addr.
	addr := m.c.AllocateVReg(ssa.TypeI64)
	leaJmpTableAddr := m.allocateInstr()
	m.insert(leaJmpTableAddr)

	// Then add the target's offset into jmpTableAddr.
	loadTargetOffsetFromJmpTable := m.allocateInstr().asAluRmiR(aluRmiROpcodeAdd,
		// Shift by 3 because each entry is 8 bytes.
		newOperandMem(m.newAmodeRegRegShift(0, addr, v, 3)), addr, true)
	m.insert(loadTargetOffsetFromJmpTable)

	// Now ready to jump.
	jmp := m.allocateInstr().asJmp(newOperandReg(addr))
	m.insert(jmp)

	jmpTableBegin, jmpTableBeginLabel := m.allocateBrTarget()
	m.insert(jmpTableBegin)
	leaJmpTableAddr.asLEA(newOperandLabel(jmpTableBeginLabel), addr)

	jmpTable := m.allocateInstr()
	targetSliceIndex := m.addJmpTableTarget(targets)
	jmpTable.asJmpTableSequence(targetSliceIndex, targetCount)
	m.insert(jmpTable)
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

	target := ssaBlockLabel(m.c.SSABuilder().BasicBlock(targetBlkID))
	cvalDef := m.c.ValueDefinition(cval)

	switch m.c.MatchInstrOneOf(cvalDef, condBranchMatches[:]) {
	case ssa.OpcodeIcmp:
		cvalInstr := cvalDef.Instr
		x, y, c := cvalInstr.IcmpData()

		cc := condFromSSAIntCmpCond(c)
		if b.Opcode() == ssa.OpcodeBrz {
			cc = cc.invert()
		}

		// First, perform the comparison and set the flag.
		xd, yd := m.c.ValueDefinition(x), m.c.ValueDefinition(y)
		if !m.tryLowerBandToFlag(xd, yd) {
			m.lowerIcmpToFlag(xd, yd, x.Type() == ssa.TypeI64)
		}

		// Then perform the conditional branch.
		m.insert(m.allocateInstr().asJmpIf(cc, newOperandLabel(target)))
		cvalDef.Instr.MarkLowered()
	case ssa.OpcodeFcmp:
		cvalInstr := cvalDef.Instr

		f1, f2, and := m.lowerFcmpToFlags(cvalInstr)
		isBrz := b.Opcode() == ssa.OpcodeBrz
		if isBrz {
			f1 = f1.invert()
		}
		if f2 == condInvalid {
			m.insert(m.allocateInstr().asJmpIf(f1, newOperandLabel(target)))
		} else {
			if isBrz {
				f2 = f2.invert()
				and = !and
			}
			jmp1, jmp2 := m.allocateInstr(), m.allocateInstr()
			m.insert(jmp1)
			m.insert(jmp2)
			notTaken, notTakenLabel := m.allocateBrTarget()
			m.insert(notTaken)
			if and {
				jmp1.asJmpIf(f1.invert(), newOperandLabel(notTakenLabel))
				jmp2.asJmpIf(f2, newOperandLabel(target))
			} else {
				jmp1.asJmpIf(f1, newOperandLabel(target))
				jmp2.asJmpIf(f2, newOperandLabel(target))
			}
		}

		cvalDef.Instr.MarkLowered()
	default:
		v := m.getOperand_Reg(cvalDef)

		var cc cond
		if b.Opcode() == ssa.OpcodeBrz {
			cc = condZ
		} else {
			cc = condNZ
		}

		// Perform test %v, %v to set the flag.
		cmp := m.allocateInstr().asCmpRmiR(false, v, v.reg(), false)
		m.insert(cmp)
		m.insert(m.allocateInstr().asJmpIf(cc, newOperandLabel(target)))
	}
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
	case ssa.OpcodeIconst, ssa.OpcodeF32const, ssa.OpcodeF64const: // Constant instructions are inlined.
	case ssa.OpcodeCall, ssa.OpcodeCallIndirect:
		m.lowerCall(instr)
	case ssa.OpcodeStore, ssa.OpcodeIstore8, ssa.OpcodeIstore16, ssa.OpcodeIstore32:
		m.lowerStore(instr)
	case ssa.OpcodeIadd:
		m.lowerAluRmiROp(instr, aluRmiROpcodeAdd)
	case ssa.OpcodeIsub:
		m.lowerAluRmiROp(instr, aluRmiROpcodeSub)
	case ssa.OpcodeImul:
		m.lowerAluRmiROp(instr, aluRmiROpcodeMul)
	case ssa.OpcodeSdiv, ssa.OpcodeUdiv, ssa.OpcodeSrem, ssa.OpcodeUrem:
		isDiv := op == ssa.OpcodeSdiv || op == ssa.OpcodeUdiv
		isSigned := op == ssa.OpcodeSdiv || op == ssa.OpcodeSrem
		m.lowerIDivRem(instr, isDiv, isSigned)
	case ssa.OpcodeBand:
		m.lowerAluRmiROp(instr, aluRmiROpcodeAnd)
	case ssa.OpcodeBor:
		m.lowerAluRmiROp(instr, aluRmiROpcodeOr)
	case ssa.OpcodeBxor:
		m.lowerAluRmiROp(instr, aluRmiROpcodeXor)
	case ssa.OpcodeIshl:
		m.lowerShiftR(instr, shiftROpShiftLeft)
	case ssa.OpcodeSshr:
		m.lowerShiftR(instr, shiftROpShiftRightArithmetic)
	case ssa.OpcodeUshr:
		m.lowerShiftR(instr, shiftROpShiftRightLogical)
	case ssa.OpcodeRotl:
		m.lowerShiftR(instr, shiftROpRotateLeft)
	case ssa.OpcodeRotr:
		m.lowerShiftR(instr, shiftROpRotateRight)
	case ssa.OpcodeClz:
		m.lowerClz(instr)
	case ssa.OpcodeCtz:
		m.lowerCtz(instr)
	case ssa.OpcodePopcnt:
		m.lowerUnaryRmR(instr, unaryRmROpcodePopcnt)
	case ssa.OpcodeFadd, ssa.OpcodeFsub, ssa.OpcodeFmul, ssa.OpcodeFdiv:
		m.lowerXmmRmR(instr)
	case ssa.OpcodeFabs:
		m.lowerFabsFneg(instr)
	case ssa.OpcodeFneg:
		m.lowerFabsFneg(instr)
	case ssa.OpcodeCeil:
		m.lowerRound(instr, roundingModeUp)
	case ssa.OpcodeFloor:
		m.lowerRound(instr, roundingModeDown)
	case ssa.OpcodeTrunc:
		m.lowerRound(instr, roundingModeZero)
	case ssa.OpcodeNearest:
		m.lowerRound(instr, roundingModeNearest)
	case ssa.OpcodeFmin, ssa.OpcodeFmax:
		m.lowerFminFmax(instr)
	case ssa.OpcodeFcopysign:
		m.lowerFcopysign(instr)
	case ssa.OpcodeBitcast:
		m.lowerBitcast(instr)
	case ssa.OpcodeSqrt:
		m.lowerSqrt(instr)
	case ssa.OpcodeFpromote:
		v := instr.Arg()
		rn := m.getOperand_Reg(m.c.ValueDefinition(v))
		rd := m.c.VRegOf(instr.Return())
		cnt := m.allocateInstr()
		cnt.asXmmUnaryRmR(sseOpcodeCvtss2sd, rn, rd)
		m.insert(cnt)
	case ssa.OpcodeFdemote:
		v := instr.Arg()
		rn := m.getOperand_Reg(m.c.ValueDefinition(v))
		rd := m.c.VRegOf(instr.Return())
		cnt := m.allocateInstr()
		cnt.asXmmUnaryRmR(sseOpcodeCvtsd2ss, rn, rd)
		m.insert(cnt)
	case ssa.OpcodeFcvtToSint, ssa.OpcodeFcvtToSintSat:
		x, ctx := instr.Arg2()
		rn := m.getOperand_Reg(m.c.ValueDefinition(x))
		rd := m.c.VRegOf(instr.Return())
		ctxVReg := m.c.VRegOf(ctx)
		m.lowerFcvtToSint(ctxVReg, rn.reg(), rd, x.Type() == ssa.TypeF64,
			instr.Return().Type().Bits() == 64, op == ssa.OpcodeFcvtToSintSat)
	case ssa.OpcodeFcvtToUint, ssa.OpcodeFcvtToUintSat:
		x, ctx := instr.Arg2()
		rn := m.getOperand_Reg(m.c.ValueDefinition(x))
		rd := m.c.VRegOf(instr.Return())
		ctxVReg := m.c.VRegOf(ctx)
		m.lowerFcvtToUint(ctxVReg, rn.reg(), rd, x.Type() == ssa.TypeF64,
			instr.Return().Type().Bits() == 64, op == ssa.OpcodeFcvtToUintSat)
	case ssa.OpcodeFcvtFromSint:
		x := instr.Arg()
		rn := m.getOperand_Reg(m.c.ValueDefinition(x))
		rd := newOperandReg(m.c.VRegOf(instr.Return()))
		m.lowerFcvtFromSint(rn, rd,
			x.Type() == ssa.TypeI64, instr.Return().Type().Bits() == 64)
	case ssa.OpcodeFcvtFromUint:
		x := instr.Arg()
		rn := m.getOperand_Reg(m.c.ValueDefinition(x))
		rd := newOperandReg(m.c.VRegOf(instr.Return()))
		m.lowerFcvtFromUint(rn, rd, x.Type() == ssa.TypeI64,
			instr.Return().Type().Bits() == 64)
	case ssa.OpcodeVanyTrue:
		m.lowerVanyTrue(instr)
	case ssa.OpcodeVallTrue:
		m.lowerVallTrue(instr)
	case ssa.OpcodeVhighBits:
		m.lowerVhighBits(instr)
	case ssa.OpcodeVbnot:
		m.lowerVbnot(instr)
	case ssa.OpcodeVband:
		x, y := instr.Arg2()
		m.lowerVbBinOp(sseOpcodePand, x, y, instr.Return())
	case ssa.OpcodeVbor:
		x, y := instr.Arg2()
		m.lowerVbBinOp(sseOpcodePor, x, y, instr.Return())
	case ssa.OpcodeVbxor:
		x, y := instr.Arg2()
		m.lowerVbBinOp(sseOpcodePxor, x, y, instr.Return())
	case ssa.OpcodeVbandnot:
		m.lowerVbandnot(instr, sseOpcodePandn)
	case ssa.OpcodeVbitselect:
		m.lowerVbitselect(instr)
	case ssa.OpcodeVIadd:
		x, y, lane := instr.Arg2WithLane()
		var vecOp sseOpcode
		switch lane {
		case ssa.VecLaneI8x16:
			vecOp = sseOpcodePaddb
		case ssa.VecLaneI16x8:
			vecOp = sseOpcodePaddw
		case ssa.VecLaneI32x4:
			vecOp = sseOpcodePaddd
		case ssa.VecLaneI64x2:
			vecOp = sseOpcodePaddq
		}
		m.lowerVbBinOp(vecOp, x, y, instr.Return())

	case ssa.OpcodeVSaddSat:
		x, y, lane := instr.Arg2WithLane()
		var vecOp sseOpcode
		switch lane {
		case ssa.VecLaneI8x16:
			vecOp = sseOpcodePaddsb
		case ssa.VecLaneI16x8:
			vecOp = sseOpcodePaddsw
		}
		m.lowerVbBinOp(vecOp, x, y, instr.Return())

	case ssa.OpcodeVUaddSat:
		x, y, lane := instr.Arg2WithLane()
		var vecOp sseOpcode
		switch lane {
		case ssa.VecLaneI8x16:
			vecOp = sseOpcodePaddusb
		case ssa.VecLaneI16x8:
			vecOp = sseOpcodePaddusw
		}
		m.lowerVbBinOp(vecOp, x, y, instr.Return())

	case ssa.OpcodeVIsub:
		x, y, lane := instr.Arg2WithLane()
		var vecOp sseOpcode
		switch lane {
		case ssa.VecLaneI8x16:
			vecOp = sseOpcodePsubb
		case ssa.VecLaneI16x8:
			vecOp = sseOpcodePsubw
		case ssa.VecLaneI32x4:
			vecOp = sseOpcodePsubd
		case ssa.VecLaneI64x2:
			vecOp = sseOpcodePsubq
		}
		m.lowerVbBinOp(vecOp, x, y, instr.Return())

	case ssa.OpcodeVSsubSat:
		x, y, lane := instr.Arg2WithLane()
		var vecOp sseOpcode
		switch lane {
		case ssa.VecLaneI8x16:
			vecOp = sseOpcodePsubsb
		case ssa.VecLaneI16x8:
			vecOp = sseOpcodePsubsw
		}
		m.lowerVbBinOp(vecOp, x, y, instr.Return())

	case ssa.OpcodeVUsubSat:
		x, y, lane := instr.Arg2WithLane()
		var vecOp sseOpcode
		switch lane {
		case ssa.VecLaneI8x16:
			vecOp = sseOpcodePsubusb
		case ssa.VecLaneI16x8:
			vecOp = sseOpcodePsubusw
		}
		m.lowerVbBinOp(vecOp, x, y, instr.Return())

	case ssa.OpcodeVImul:
		m.lowerVImul(instr)
	case ssa.OpcodeVIneg:
		x, lane := instr.ArgWithLane()
		rn := m.getOperand_Reg(m.c.ValueDefinition(x))
		rd := m.c.VRegOf(instr.Return())
		var vecOp sseOpcode
		switch lane {
		case ssa.VecLaneI8x16:
			vecOp = sseOpcodePsubb
		case ssa.VecLaneI16x8:
			vecOp = sseOpcodePsubw
		case ssa.VecLaneI32x4:
			vecOp = sseOpcodePsubd
		case ssa.VecLaneI64x2:
			vecOp = sseOpcodePsubq
		default:
			panic("BUG")
		}

		tmp := m.c.AllocateVReg(ssa.TypeV128)
		m.insert(m.allocateInstr().asZeros(tmp))

		i := m.allocateInstr()
		i.asXmmRmR(vecOp, rn, tmp)
		m.insert(i)

		m.copyTo(tmp, rd)
	case ssa.OpcodeVFadd:
		x, y, lane := instr.Arg2WithLane()
		var vecOp sseOpcode
		switch lane {
		case ssa.VecLaneF32x4:
			vecOp = sseOpcodeAddps
		case ssa.VecLaneF64x2:
			vecOp = sseOpcodeAddpd
		}
		m.lowerVbBinOp(vecOp, x, y, instr.Return())

	case ssa.OpcodeVFsub:
		x, y, lane := instr.Arg2WithLane()
		var vecOp sseOpcode
		switch lane {
		case ssa.VecLaneF32x4:
			vecOp = sseOpcodeSubps
		case ssa.VecLaneF64x2:
			vecOp = sseOpcodeSubpd
		}
		m.lowerVbBinOp(vecOp, x, y, instr.Return())

	case ssa.OpcodeVFdiv:
		x, y, lane := instr.Arg2WithLane()
		var vecOp sseOpcode
		switch lane {
		case ssa.VecLaneF32x4:
			vecOp = sseOpcodeDivps
		case ssa.VecLaneF64x2:
			vecOp = sseOpcodeDivpd
		}
		m.lowerVbBinOp(vecOp, x, y, instr.Return())

	case ssa.OpcodeVFmul:
		x, y, lane := instr.Arg2WithLane()
		var vecOp sseOpcode
		switch lane {
		case ssa.VecLaneF32x4:
			vecOp = sseOpcodeMulps
		case ssa.VecLaneF64x2:
			vecOp = sseOpcodeMulpd
		}
		m.lowerVbBinOp(vecOp, x, y, instr.Return())

	case ssa.OpcodeVFneg:
		x, lane := instr.ArgWithLane()
		rn := m.getOperand_Reg(m.c.ValueDefinition(x))
		rd := m.c.VRegOf(instr.Return())

		tmp := m.c.AllocateVReg(ssa.TypeV128)

		var shiftOp, xorOp sseOpcode
		var shiftAmt uint32
		switch lane {
		case ssa.VecLaneF32x4:
			shiftOp, shiftAmt, xorOp = sseOpcodePslld, 31, sseOpcodeXorps
		case ssa.VecLaneF64x2:
			shiftOp, shiftAmt, xorOp = sseOpcodePsllq, 63, sseOpcodeXorpd
		}

		zero := m.allocateInstr()
		zero.asZeros(tmp)
		m.insert(zero)

		// Set all bits on tmp by CMPPD with arg=0 (== pseudo CMPEQPD instruction).
		// See https://www.felixcloutier.com/x86/cmpps
		//
		// Note: if we do not clear all the bits ^ with XORPS, this might end up not setting ones on some lane
		// if the lane is NaN.
		cmp := m.allocateInstr()
		cmp.asXmmRmRImm(sseOpcodeCmppd, uint8(cmpPredEQ_UQ), newOperandReg(tmp), tmp)
		m.insert(cmp)

		// Do the left shift on each lane to set only the most significant bit in each.
		i := m.allocateInstr()
		i.asXmmRmiReg(shiftOp, newOperandImm32(shiftAmt), tmp)
		m.insert(i)

		// Get the negated result by XOR on each lane with tmp.
		i = m.allocateInstr()
		i.asXmmRmR(xorOp, rn, tmp)
		m.insert(i)

		m.copyTo(tmp, rd)

	case ssa.OpcodeVSqrt:
		x, lane := instr.ArgWithLane()
		rn := m.getOperand_Reg(m.c.ValueDefinition(x))
		rd := m.c.VRegOf(instr.Return())

		var vecOp sseOpcode
		switch lane {
		case ssa.VecLaneF32x4:
			vecOp = sseOpcodeSqrtps
		case ssa.VecLaneF64x2:
			vecOp = sseOpcodeSqrtpd
		}
		i := m.allocateInstr()
		i.asXmmUnaryRmR(vecOp, rn, rd)
		m.insert(i)

	case ssa.OpcodeVImin:
		x, y, lane := instr.Arg2WithLane()
		var vecOp sseOpcode
		switch lane {
		case ssa.VecLaneI8x16:
			vecOp = sseOpcodePminsb
		case ssa.VecLaneI16x8:
			vecOp = sseOpcodePminsw
		case ssa.VecLaneI32x4:
			vecOp = sseOpcodePminsd
		}
		m.lowerVbBinOp(vecOp, x, y, instr.Return())

	case ssa.OpcodeVUmin:
		x, y, lane := instr.Arg2WithLane()
		var vecOp sseOpcode
		switch lane {
		case ssa.VecLaneI8x16:
			vecOp = sseOpcodePminub
		case ssa.VecLaneI16x8:
			vecOp = sseOpcodePminuw
		case ssa.VecLaneI32x4:
			vecOp = sseOpcodePminud
		}
		m.lowerVbBinOp(vecOp, x, y, instr.Return())

	case ssa.OpcodeVImax:
		x, y, lane := instr.Arg2WithLane()
		var vecOp sseOpcode
		switch lane {
		case ssa.VecLaneI8x16:
			vecOp = sseOpcodePmaxsb
		case ssa.VecLaneI16x8:
			vecOp = sseOpcodePmaxsw
		case ssa.VecLaneI32x4:
			vecOp = sseOpcodePmaxsd
		}
		m.lowerVbBinOp(vecOp, x, y, instr.Return())

	case ssa.OpcodeVUmax:
		x, y, lane := instr.Arg2WithLane()
		var vecOp sseOpcode
		switch lane {
		case ssa.VecLaneI8x16:
			vecOp = sseOpcodePmaxub
		case ssa.VecLaneI16x8:
			vecOp = sseOpcodePmaxuw
		case ssa.VecLaneI32x4:
			vecOp = sseOpcodePmaxud
		}
		m.lowerVbBinOp(vecOp, x, y, instr.Return())

	case ssa.OpcodeVAvgRound:
		x, y, lane := instr.Arg2WithLane()
		var vecOp sseOpcode
		switch lane {
		case ssa.VecLaneI8x16:
			vecOp = sseOpcodePavgb
		case ssa.VecLaneI16x8:
			vecOp = sseOpcodePavgw
		}
		m.lowerVbBinOp(vecOp, x, y, instr.Return())

	case ssa.OpcodeVIcmp:
		x, y, c, lane := instr.VIcmpData()
		m.lowerVIcmp(x, y, c, instr.Return(), lane)

	case ssa.OpcodeVFcmp:
		x, y, c, lane := instr.VFcmpData()
		m.lowerVFcmp(x, y, c, instr.Return(), lane)

	case ssa.OpcodeExtractlane:
		x, index, signed, lane := instr.ExtractlaneData()
		m.lowerExtractLane(x, index, signed, instr.Return(), lane)

	case ssa.OpcodeInsertlane:
		x, y, index, lane := instr.InsertlaneData()
		m.lowerInsertLane(x, y, index, instr.Return(), lane)

	case ssa.OpcodeSwizzle:
		x, y, _ := instr.Arg2WithLane()
		m.lowerSwizzle(x, y, instr.Return())

	case ssa.OpcodeShuffle:
		x, y, lo, hi := instr.ShuffleData()
		m.lowerShuffle(x, y, lo, hi, instr.Return())

	case ssa.OpcodeSplat:
		x, lane := instr.ArgWithLane()
		m.lowerSplat(x, instr.Return(), lane)

	case ssa.OpcodeSqmulRoundSat:
		x, y := instr.Arg2()
		m.lowerSqmulRoundSat(x, y, instr.Return())

	case ssa.OpcodeVZeroExtLoad:
		ptr, offset, typ := instr.VZeroExtLoadData()
		var sseOp sseOpcode
		// Both movss and movsd clears the higher bits of the destination register upt 128 bits.
		// https://www.felixcloutier.com/x86/movss
		// https://www.felixcloutier.com/x86/movsd
		if typ == ssa.TypeF32 {
			sseOp = sseOpcodeMovss
		} else {
			sseOp = sseOpcodeMovsd
		}
		mem := m.lowerToAddressMode(ptr, offset)
		dst := m.c.VRegOf(instr.Return())
		m.insert(m.allocateInstr().asXmmUnaryRmR(sseOp, newOperandMem(mem), dst))

	case ssa.OpcodeVMinPseudo:
		x, y, lane := instr.Arg2WithLane()
		var vecOp sseOpcode
		switch lane {
		case ssa.VecLaneF32x4:
			vecOp = sseOpcodeMinps
		case ssa.VecLaneF64x2:
			vecOp = sseOpcodeMinpd
		default:
			panic("BUG: unexpected lane type")
		}
		m.lowerVbBinOpUnaligned(vecOp, y, x, instr.Return())

	case ssa.OpcodeVMaxPseudo:
		x, y, lane := instr.Arg2WithLane()
		var vecOp sseOpcode
		switch lane {
		case ssa.VecLaneF32x4:
			vecOp = sseOpcodeMaxps
		case ssa.VecLaneF64x2:
			vecOp = sseOpcodeMaxpd
		default:
			panic("BUG: unexpected lane type")
		}
		m.lowerVbBinOpUnaligned(vecOp, y, x, instr.Return())

	case ssa.OpcodeVIshl:
		x, y, lane := instr.Arg2WithLane()
		m.lowerVIshl(x, y, instr.Return(), lane)

	case ssa.OpcodeVSshr:
		x, y, lane := instr.Arg2WithLane()
		m.lowerVSshr(x, y, instr.Return(), lane)

	case ssa.OpcodeVUshr:
		x, y, lane := instr.Arg2WithLane()
		m.lowerVUshr(x, y, instr.Return(), lane)

	case ssa.OpcodeVCeil:
		x, lane := instr.ArgWithLane()
		m.lowerVRound(x, instr.Return(), 0x2, lane == ssa.VecLaneF64x2)

	case ssa.OpcodeVFloor:
		x, lane := instr.ArgWithLane()
		m.lowerVRound(x, instr.Return(), 0x1, lane == ssa.VecLaneF64x2)

	case ssa.OpcodeVTrunc:
		x, lane := instr.ArgWithLane()
		m.lowerVRound(x, instr.Return(), 0x3, lane == ssa.VecLaneF64x2)

	case ssa.OpcodeVNearest:
		x, lane := instr.ArgWithLane()
		m.lowerVRound(x, instr.Return(), 0x0, lane == ssa.VecLaneF64x2)

	case ssa.OpcodeExtIaddPairwise:
		x, lane, signed := instr.ExtIaddPairwiseData()
		m.lowerExtIaddPairwise(x, instr.Return(), lane, signed)

	case ssa.OpcodeUwidenLow, ssa.OpcodeSwidenLow:
		x, lane := instr.ArgWithLane()
		m.lowerWidenLow(x, instr.Return(), lane, op == ssa.OpcodeSwidenLow)

	case ssa.OpcodeUwidenHigh, ssa.OpcodeSwidenHigh:
		x, lane := instr.ArgWithLane()
		m.lowerWidenHigh(x, instr.Return(), lane, op == ssa.OpcodeSwidenHigh)

	case ssa.OpcodeLoadSplat:
		ptr, offset, lane := instr.LoadSplatData()
		m.lowerLoadSplat(ptr, offset, instr.Return(), lane)

	case ssa.OpcodeVFcvtFromUint, ssa.OpcodeVFcvtFromSint:
		x, lane := instr.ArgWithLane()
		m.lowerVFcvtFromInt(x, instr.Return(), lane, op == ssa.OpcodeVFcvtFromSint)

	case ssa.OpcodeVFcvtToSintSat, ssa.OpcodeVFcvtToUintSat:
		x, lane := instr.ArgWithLane()
		m.lowerVFcvtToIntSat(x, instr.Return(), lane, op == ssa.OpcodeVFcvtToSintSat)

	case ssa.OpcodeSnarrow, ssa.OpcodeUnarrow:
		x, y, lane := instr.Arg2WithLane()
		m.lowerNarrow(x, y, instr.Return(), lane, op == ssa.OpcodeSnarrow)

	case ssa.OpcodeFvpromoteLow:
		x := instr.Arg()
		src := m.getOperand_Reg(m.c.ValueDefinition(x))
		dst := m.c.VRegOf(instr.Return())
		m.insert(m.allocateInstr().asXmmUnaryRmR(sseOpcodeCvtps2pd, src, dst))

	case ssa.OpcodeFvdemote:
		x := instr.Arg()
		src := m.getOperand_Reg(m.c.ValueDefinition(x))
		dst := m.c.VRegOf(instr.Return())
		m.insert(m.allocateInstr().asXmmUnaryRmR(sseOpcodeCvtpd2ps, src, dst))

	case ssa.OpcodeWideningPairwiseDotProductS:
		x, y := instr.Arg2()
		m.lowerWideningPairwiseDotProductS(x, y, instr.Return())

	case ssa.OpcodeVIabs:
		m.lowerVIabs(instr)
	case ssa.OpcodeVIpopcnt:
		m.lowerVIpopcnt(instr)
	case ssa.OpcodeVFmin:
		m.lowerVFmin(instr)
	case ssa.OpcodeVFmax:
		m.lowerVFmax(instr)
	case ssa.OpcodeVFabs:
		m.lowerVFabs(instr)
	case ssa.OpcodeUndefined:
		m.insert(m.allocateInstr().asUD2())
	case ssa.OpcodeExitWithCode:
		execCtx, code := instr.ExitWithCodeData()
		m.lowerExitWithCode(m.c.VRegOf(execCtx), code)
	case ssa.OpcodeExitIfTrueWithCode:
		execCtx, c, code := instr.ExitIfTrueWithCodeData()
		m.lowerExitIfTrueWithCode(m.c.VRegOf(execCtx), c, code)
	case ssa.OpcodeLoad:
		ptr, offset, typ := instr.LoadData()
		dst := m.c.VRegOf(instr.Return())
		m.lowerLoad(ptr, offset, typ, dst)
	case ssa.OpcodeUload8, ssa.OpcodeUload16, ssa.OpcodeUload32, ssa.OpcodeSload8, ssa.OpcodeSload16, ssa.OpcodeSload32:
		ptr, offset, _ := instr.LoadData()
		ret := m.c.VRegOf(instr.Return())
		m.lowerExtLoad(op, ptr, offset, ret)
	case ssa.OpcodeVconst:
		result := m.c.VRegOf(instr.Return())
		lo, hi := instr.VconstData()
		m.lowerVconst(result, lo, hi)
	case ssa.OpcodeSExtend, ssa.OpcodeUExtend:
		from, to, signed := instr.ExtendData()
		m.lowerExtend(instr.Arg(), instr.Return(), from, to, signed)
	case ssa.OpcodeIcmp:
		m.lowerIcmp(instr)
	case ssa.OpcodeFcmp:
		m.lowerFcmp(instr)
	case ssa.OpcodeSelect:
		cval, x, y := instr.SelectData()
		m.lowerSelect(x, y, cval, instr.Return())
	case ssa.OpcodeIreduce:
		rn := m.getOperand_Mem_Reg(m.c.ValueDefinition(instr.Arg()))
		retVal := instr.Return()
		rd := m.c.VRegOf(retVal)

		if retVal.Type() != ssa.TypeI32 {
			panic("TODO?: Ireduce to non-i32")
		}
		m.insert(m.allocateInstr().asMovzxRmR(extModeLQ, rn, rd))

	case ssa.OpcodeAtomicLoad:
		ptr := instr.Arg()
		size := instr.AtomicTargetSize()
		dst := m.c.VRegOf(instr.Return())

		// At this point, the ptr is ensured to be aligned, so using a normal load is atomic.
		// https://github.com/golang/go/blob/adead1a93f472affa97c494ef19f2f492ee6f34a/src/runtime/internal/atomic/atomic_amd64.go#L30
		mem := newOperandMem(m.lowerToAddressMode(ptr, 0))
		load := m.allocateInstr()
		switch size {
		case 8:
			load.asMov64MR(mem, dst)
		case 4:
			load.asMovzxRmR(extModeLQ, mem, dst)
		case 2:
			load.asMovzxRmR(extModeWQ, mem, dst)
		case 1:
			load.asMovzxRmR(extModeBQ, mem, dst)
		default:
			panic("BUG")
		}
		m.insert(load)

	case ssa.OpcodeFence:
		m.insert(m.allocateInstr().asMFence())

	case ssa.OpcodeAtomicStore:
		ptr, _val := instr.Arg2()
		size := instr.AtomicTargetSize()

		val := m.getOperand_Reg(m.c.ValueDefinition(_val))
		// The content on the val register will be overwritten by xchg, so we need to copy it to a temporary register.
		copied := m.copyToTmp(val.reg())

		mem := newOperandMem(m.lowerToAddressMode(ptr, 0))
		store := m.allocateInstr().asXCHG(copied, mem, byte(size))
		m.insert(store)

	case ssa.OpcodeAtomicCas:
		addr, exp, repl := instr.Arg3()
		size := instr.AtomicTargetSize()
		m.lowerAtomicCas(addr, exp, repl, size, instr.Return())

	case ssa.OpcodeAtomicRmw:
		addr, val := instr.Arg2()
		atomicOp, size := instr.AtomicRmwData()
		m.lowerAtomicRmw(atomicOp, addr, val, size, instr.Return())

	default:
		panic("TODO: lowering " + op.String())
	}
}

func (m *machine) lowerAtomicRmw(op ssa.AtomicRmwOp, addr, val ssa.Value, size uint64, ret ssa.Value) {
	mem := m.lowerToAddressMode(addr, 0)
	_val := m.getOperand_Reg(m.c.ValueDefinition(val))

	switch op {
	case ssa.AtomicRmwOpAdd, ssa.AtomicRmwOpSub:
		valCopied := m.copyToTmp(_val.reg())
		if op == ssa.AtomicRmwOpSub {
			// Negate the value.
			m.insert(m.allocateInstr().asNeg(newOperandReg(valCopied), true))
		}
		m.insert(m.allocateInstr().asLockXAdd(valCopied, mem, byte(size)))
		m.clearHigherBitsForAtomic(valCopied, size, ret.Type())
		m.copyTo(valCopied, m.c.VRegOf(ret))

	case ssa.AtomicRmwOpAnd, ssa.AtomicRmwOpOr, ssa.AtomicRmwOpXor:
		accumulator := raxVReg
		// Reserve rax for the accumulator to make regalloc happy.
		// Note: do this initialization before defining valCopied, because it might be the same register and
		// if that happens, the unnecessary load/store will be performed inside the loop.
		// This can be mitigated in any way once the register allocator is clever enough.
		m.insert(m.allocateInstr().asDefineUninitializedReg(accumulator))

		// Copy the value to a temporary register.
		valCopied := m.copyToTmp(_val.reg())
		m.clearHigherBitsForAtomic(valCopied, size, ret.Type())

		memOp := newOperandMem(mem)
		tmp := m.c.AllocateVReg(ssa.TypeI64)
		beginLoop, beginLoopLabel := m.allocateBrTarget()
		{
			m.insert(beginLoop)
			// Reset the value on tmp by the original value.
			m.copyTo(valCopied, tmp)
			// Load the current value at the memory location into accumulator.
			switch size {
			case 1:
				m.insert(m.allocateInstr().asMovzxRmR(extModeBQ, memOp, accumulator))
			case 2:
				m.insert(m.allocateInstr().asMovzxRmR(extModeWQ, memOp, accumulator))
			case 4:
				m.insert(m.allocateInstr().asMovzxRmR(extModeLQ, memOp, accumulator))
			case 8:
				m.insert(m.allocateInstr().asMov64MR(memOp, accumulator))
			default:
				panic("BUG")
			}
			// Then perform the logical operation on the accumulator and the value on tmp.
			switch op {
			case ssa.AtomicRmwOpAnd:
				m.insert(m.allocateInstr().asAluRmiR(aluRmiROpcodeAnd, newOperandReg(accumulator), tmp, true))
			case ssa.AtomicRmwOpOr:
				m.insert(m.allocateInstr().asAluRmiR(aluRmiROpcodeOr, newOperandReg(accumulator), tmp, true))
			case ssa.AtomicRmwOpXor:
				m.insert(m.allocateInstr().asAluRmiR(aluRmiROpcodeXor, newOperandReg(accumulator), tmp, true))
			default:
				panic("BUG")
			}
			// Finally, try compare-exchange the value at the memory location with the tmp.
			m.insert(m.allocateInstr().asLockCmpXCHG(tmp, memOp.addressMode(), byte(size)))
			// If it succeeds, ZF will be set, and we can break the loop.
			m.insert(m.allocateInstr().asJmpIf(condNZ, newOperandLabel(beginLoopLabel)))
		}

		// valCopied must be alive at the end of the loop.
		m.insert(m.allocateInstr().asNopUseReg(valCopied))

		// At this point, accumulator contains the result.
		m.clearHigherBitsForAtomic(accumulator, size, ret.Type())
		m.copyTo(accumulator, m.c.VRegOf(ret))

	case ssa.AtomicRmwOpXchg:
		valCopied := m.copyToTmp(_val.reg())

		m.insert(m.allocateInstr().asXCHG(valCopied, newOperandMem(mem), byte(size)))
		m.clearHigherBitsForAtomic(valCopied, size, ret.Type())
		m.copyTo(valCopied, m.c.VRegOf(ret))

	default:
		panic("BUG")
	}
}

func (m *machine) lowerAtomicCas(addr, exp, repl ssa.Value, size uint64, ret ssa.Value) {
	mem := m.lowerToAddressMode(addr, 0)
	expOp := m.getOperand_Reg(m.c.ValueDefinition(exp))
	replOp := m.getOperand_Reg(m.c.ValueDefinition(repl))

	accumulator := raxVReg
	m.copyTo(expOp.reg(), accumulator)
	m.insert(m.allocateInstr().asLockCmpXCHG(replOp.reg(), mem, byte(size)))
	m.clearHigherBitsForAtomic(accumulator, size, ret.Type())
	m.copyTo(accumulator, m.c.VRegOf(ret))
}

func (m *machine) clearHigherBitsForAtomic(r regalloc.VReg, valSize uint64, resultType ssa.Type) {
	switch resultType {
	case ssa.TypeI32:
		switch valSize {
		case 1:
			m.insert(m.allocateInstr().asMovzxRmR(extModeBL, newOperandReg(r), r))
		case 2:
			m.insert(m.allocateInstr().asMovzxRmR(extModeWL, newOperandReg(r), r))
		}
	case ssa.TypeI64:
		switch valSize {
		case 1:
			m.insert(m.allocateInstr().asMovzxRmR(extModeBQ, newOperandReg(r), r))
		case 2:
			m.insert(m.allocateInstr().asMovzxRmR(extModeWQ, newOperandReg(r), r))
		case 4:
			m.insert(m.allocateInstr().asMovzxRmR(extModeLQ, newOperandReg(r), r))
		}
	}
}

func (m *machine) lowerFcmp(instr *ssa.Instruction) {
	f1, f2, and := m.lowerFcmpToFlags(instr)
	rd := m.c.VRegOf(instr.Return())
	if f2 == condInvalid {
		tmp := m.c.AllocateVReg(ssa.TypeI32)
		m.insert(m.allocateInstr().asSetcc(f1, tmp))
		// On amd64, setcc only sets the first byte of the register, so we need to zero extend it to match
		// the semantics of Icmp that sets either 0 or 1.
		m.insert(m.allocateInstr().asMovzxRmR(extModeBQ, newOperandReg(tmp), rd))
	} else {
		tmp1, tmp2 := m.c.AllocateVReg(ssa.TypeI32), m.c.AllocateVReg(ssa.TypeI32)
		m.insert(m.allocateInstr().asSetcc(f1, tmp1))
		m.insert(m.allocateInstr().asSetcc(f2, tmp2))
		var op aluRmiROpcode
		if and {
			op = aluRmiROpcodeAnd
		} else {
			op = aluRmiROpcodeOr
		}
		m.insert(m.allocateInstr().asAluRmiR(op, newOperandReg(tmp1), tmp2, false))
		m.insert(m.allocateInstr().asMovzxRmR(extModeBQ, newOperandReg(tmp2), rd))
	}
}

func (m *machine) lowerIcmp(instr *ssa.Instruction) {
	x, y, c := instr.IcmpData()
	m.lowerIcmpToFlag(m.c.ValueDefinition(x), m.c.ValueDefinition(y), x.Type() == ssa.TypeI64)
	rd := m.c.VRegOf(instr.Return())
	tmp := m.c.AllocateVReg(ssa.TypeI32)
	m.insert(m.allocateInstr().asSetcc(condFromSSAIntCmpCond(c), tmp))
	// On amd64, setcc only sets the first byte of the register, so we need to zero extend it to match
	// the semantics of Icmp that sets either 0 or 1.
	m.insert(m.allocateInstr().asMovzxRmR(extModeBQ, newOperandReg(tmp), rd))
}

func (m *machine) lowerSelect(x, y, cval, ret ssa.Value) {
	xo, yo := m.getOperand_Mem_Reg(m.c.ValueDefinition(x)), m.getOperand_Reg(m.c.ValueDefinition(y))
	rd := m.c.VRegOf(ret)

	var cond cond
	cvalDef := m.c.ValueDefinition(cval)
	switch m.c.MatchInstrOneOf(cvalDef, condBranchMatches[:]) {
	case ssa.OpcodeIcmp:
		icmp := cvalDef.Instr
		xc, yc, cc := icmp.IcmpData()
		m.lowerIcmpToFlag(m.c.ValueDefinition(xc), m.c.ValueDefinition(yc), xc.Type() == ssa.TypeI64)
		cond = condFromSSAIntCmpCond(cc)
		icmp.Lowered()
	default: // TODO: match ssa.OpcodeFcmp for optimization, but seems a bit complex.
		cv := m.getOperand_Reg(cvalDef)
		test := m.allocateInstr().asCmpRmiR(false, cv, cv.reg(), false)
		m.insert(test)
		cond = condNZ
	}

	if typ := x.Type(); typ.IsInt() {
		_64 := typ.Bits() == 64
		mov := m.allocateInstr()
		tmp := m.c.AllocateVReg(typ)
		switch yo.kind {
		case operandKindReg:
			mov.asMovRR(yo.reg(), tmp, _64)
		case operandKindMem:
			if _64 {
				mov.asMov64MR(yo, tmp)
			} else {
				mov.asMovzxRmR(extModeLQ, yo, tmp)
			}
		default:
			panic("BUG")
		}
		m.insert(mov)
		cmov := m.allocateInstr().asCmove(cond, xo, tmp, _64)
		m.insert(cmov)
		m.insert(m.allocateInstr().asMovRR(tmp, rd, _64))
	} else {
		mov := m.allocateInstr()
		tmp := m.c.AllocateVReg(typ)
		switch typ {
		case ssa.TypeF32:
			mov.asXmmUnaryRmR(sseOpcodeMovss, yo, tmp)
		case ssa.TypeF64:
			mov.asXmmUnaryRmR(sseOpcodeMovsd, yo, tmp)
		case ssa.TypeV128:
			mov.asXmmUnaryRmR(sseOpcodeMovdqu, yo, tmp)
		default:
			panic("BUG")
		}
		m.insert(mov)

		cmov := m.allocateInstr().asXmmCMov(cond, xo, tmp, typ.Size())
		m.insert(cmov)

		m.copyTo(tmp, rd)
	}
}

func (m *machine) lowerXmmCmovAfterRegAlloc(i *instruction) {
	x := i.op1
	rd := i.op2.reg()
	cond := cond(i.u1)

	jcc := m.allocateInstr()
	m.insert(jcc)

	mov := m.allocateInstr()
	switch i.u2 {
	case 4:
		mov.asXmmUnaryRmR(sseOpcodeMovss, x, rd)
	case 8:
		mov.asXmmUnaryRmR(sseOpcodeMovsd, x, rd)
	case 16:
		mov.asXmmUnaryRmR(sseOpcodeMovdqu, x, rd)
	default:
		panic("BUG")
	}
	m.insert(mov)

	nop, end := m.allocateBrTarget()
	m.insert(nop)
	jcc.asJmpIf(cond.invert(), newOperandLabel(end))
}

func (m *machine) lowerExtend(_arg, ret ssa.Value, from, to byte, signed bool) {
	rd0 := m.c.VRegOf(ret)
	arg := m.getOperand_Mem_Reg(m.c.ValueDefinition(_arg))

	rd := m.c.AllocateVReg(ret.Type())

	ext := m.allocateInstr()
	switch {
	case from == 8 && to == 16 && signed:
		ext.asMovsxRmR(extModeBQ, arg, rd)
	case from == 8 && to == 16 && !signed:
		ext.asMovzxRmR(extModeBL, arg, rd)
	case from == 8 && to == 32 && signed:
		ext.asMovsxRmR(extModeBL, arg, rd)
	case from == 8 && to == 32 && !signed:
		ext.asMovzxRmR(extModeBQ, arg, rd)
	case from == 8 && to == 64 && signed:
		ext.asMovsxRmR(extModeBQ, arg, rd)
	case from == 8 && to == 64 && !signed:
		ext.asMovzxRmR(extModeBQ, arg, rd)
	case from == 16 && to == 32 && signed:
		ext.asMovsxRmR(extModeWL, arg, rd)
	case from == 16 && to == 32 && !signed:
		ext.asMovzxRmR(extModeWL, arg, rd)
	case from == 16 && to == 64 && signed:
		ext.asMovsxRmR(extModeWQ, arg, rd)
	case from == 16 && to == 64 && !signed:
		ext.asMovzxRmR(extModeWQ, arg, rd)
	case from == 32 && to == 64 && signed:
		ext.asMovsxRmR(extModeLQ, arg, rd)
	case from == 32 && to == 64 && !signed:
		ext.asMovzxRmR(extModeLQ, arg, rd)
	default:
		panic(fmt.Sprintf("BUG: unhandled extend: from=%d, to=%d, signed=%t", from, to, signed))
	}
	m.insert(ext)

	m.copyTo(rd, rd0)
}

func (m *machine) lowerVconst(dst regalloc.VReg, lo, hi uint64) {
	if lo == 0 && hi == 0 {
		m.insert(m.allocateInstr().asZeros(dst))
		return
	}

	load := m.allocateInstr()
	l, pos := m.allocateLabel()
	m.consts = append(m.consts, _const{label: l, labelPos: pos, lo: lo, hi: hi})
	load.asXmmUnaryRmR(sseOpcodeMovdqu, newOperandMem(m.newAmodeRipRel(l)), dst)
	m.insert(load)
}

func (m *machine) lowerCtz(instr *ssa.Instruction) {
	if m.cpuFeatures.HasExtra(platform.CpuExtraFeatureAmd64ABM) {
		m.lowerUnaryRmR(instr, unaryRmROpcodeTzcnt)
	} else {
		// On processors that do not support TZCNT, the BSF instruction is
		// executed instead. The key difference between TZCNT and BSF
		// instruction is that if source operand is zero, the content of
		// destination operand is undefined.
		// https://www.felixcloutier.com/x86/tzcnt.html

		x := instr.Arg()
		if !x.Type().IsInt() {
			panic("BUG?")
		}
		_64 := x.Type().Bits() == 64

		xDef := m.c.ValueDefinition(x)
		tmp := m.c.AllocateVReg(x.Type())
		rm := m.getOperand_Reg(xDef)

		// First, we have to check if the target is non-zero.
		test := m.allocateInstr()
		test.asCmpRmiR(false, rm, rm.reg(), _64)
		m.insert(test)

		jmpNz := m.allocateInstr()
		m.insert(jmpNz)

		// If the value is zero, we just push the const value.
		m.lowerIconst(tmp, uint64(x.Type().Bits()), _64)

		// Now jump right after the non-zero case.
		jmpAtEnd := m.allocateInstr()
		m.insert(jmpAtEnd)

		// jmpNz target label is set here.
		nop, nz := m.allocateBrTarget()
		jmpNz.asJmpIf(condNZ, newOperandLabel(nz))
		m.insert(nop)

		// Emit the non-zero case.
		bsr := m.allocateInstr()
		bsr.asUnaryRmR(unaryRmROpcodeBsf, rm, tmp, _64)
		m.insert(bsr)

		// jmpAtEnd target label is set here.
		nopEnd, end := m.allocateBrTarget()
		jmpAtEnd.asJmp(newOperandLabel(end))
		m.insert(nopEnd)

		m.copyTo(tmp, m.c.VRegOf(instr.Return()))
	}
}

func (m *machine) lowerClz(instr *ssa.Instruction) {
	if m.cpuFeatures.HasExtra(platform.CpuExtraFeatureAmd64ABM) {
		m.lowerUnaryRmR(instr, unaryRmROpcodeLzcnt)
	} else {
		// On processors that do not support LZCNT, we combine BSR (calculating
		// most significant set bit) with XOR. This logic is described in
		// "Replace Raw Assembly Code with Builtin Intrinsics" section in:
		// https://developer.apple.com/documentation/apple-silicon/addressing-architectural-differences-in-your-macos-code.

		x := instr.Arg()
		if !x.Type().IsInt() {
			panic("BUG?")
		}
		_64 := x.Type().Bits() == 64

		xDef := m.c.ValueDefinition(x)
		rm := m.getOperand_Reg(xDef)
		tmp := m.c.AllocateVReg(x.Type())

		// First, we have to check if the rm is non-zero as BSR is undefined
		// on zero. See https://www.felixcloutier.com/x86/bsr.
		test := m.allocateInstr()
		test.asCmpRmiR(false, rm, rm.reg(), _64)
		m.insert(test)

		jmpNz := m.allocateInstr()
		m.insert(jmpNz)

		// If the value is zero, we just push the const value.
		m.lowerIconst(tmp, uint64(x.Type().Bits()), _64)

		// Now jump right after the non-zero case.
		jmpAtEnd := m.allocateInstr()
		m.insert(jmpAtEnd)

		// jmpNz target label is set here.
		nop, nz := m.allocateBrTarget()
		jmpNz.asJmpIf(condNZ, newOperandLabel(nz))
		m.insert(nop)

		// Emit the non-zero case.
		bsr := m.allocateInstr()
		bsr.asUnaryRmR(unaryRmROpcodeBsr, rm, tmp, _64)
		m.insert(bsr)

		// Now we XOR the value with the bit length minus one.
		xor := m.allocateInstr()
		xor.asAluRmiR(aluRmiROpcodeXor, newOperandImm32(uint32(x.Type().Bits()-1)), tmp, _64)
		m.insert(xor)

		// jmpAtEnd target label is set here.
		nopEnd, end := m.allocateBrTarget()
		jmpAtEnd.asJmp(newOperandLabel(end))
		m.insert(nopEnd)

		m.copyTo(tmp, m.c.VRegOf(instr.Return()))
	}
}

func (m *machine) lowerUnaryRmR(si *ssa.Instruction, op unaryRmROpcode) {
	x := si.Arg()
	if !x.Type().IsInt() {
		panic("BUG?")
	}
	_64 := x.Type().Bits() == 64

	xDef := m.c.ValueDefinition(x)
	rm := m.getOperand_Mem_Reg(xDef)
	rd := m.c.VRegOf(si.Return())

	instr := m.allocateInstr()
	instr.asUnaryRmR(op, rm, rd, _64)
	m.insert(instr)
}

func (m *machine) lowerLoad(ptr ssa.Value, offset uint32, typ ssa.Type, dst regalloc.VReg) {
	mem := newOperandMem(m.lowerToAddressMode(ptr, offset))
	load := m.allocateInstr()
	switch typ {
	case ssa.TypeI32:
		load.asMovzxRmR(extModeLQ, mem, dst)
	case ssa.TypeI64:
		load.asMov64MR(mem, dst)
	case ssa.TypeF32:
		load.asXmmUnaryRmR(sseOpcodeMovss, mem, dst)
	case ssa.TypeF64:
		load.asXmmUnaryRmR(sseOpcodeMovsd, mem, dst)
	case ssa.TypeV128:
		load.asXmmUnaryRmR(sseOpcodeMovdqu, mem, dst)
	default:
		panic("BUG")
	}
	m.insert(load)
}

func (m *machine) lowerExtLoad(op ssa.Opcode, ptr ssa.Value, offset uint32, dst regalloc.VReg) {
	mem := newOperandMem(m.lowerToAddressMode(ptr, offset))
	load := m.allocateInstr()
	switch op {
	case ssa.OpcodeUload8:
		load.asMovzxRmR(extModeBQ, mem, dst)
	case ssa.OpcodeUload16:
		load.asMovzxRmR(extModeWQ, mem, dst)
	case ssa.OpcodeUload32:
		load.asMovzxRmR(extModeLQ, mem, dst)
	case ssa.OpcodeSload8:
		load.asMovsxRmR(extModeBQ, mem, dst)
	case ssa.OpcodeSload16:
		load.asMovsxRmR(extModeWQ, mem, dst)
	case ssa.OpcodeSload32:
		load.asMovsxRmR(extModeLQ, mem, dst)
	default:
		panic("BUG")
	}
	m.insert(load)
}

func (m *machine) lowerExitIfTrueWithCode(execCtx regalloc.VReg, cond ssa.Value, code wazevoapi.ExitCode) {
	condDef := m.c.ValueDefinition(cond)
	if !m.c.MatchInstr(condDef, ssa.OpcodeIcmp) {
		panic("TODO: ExitIfTrue must come after Icmp at the moment: " + condDef.Instr.Opcode().String())
	}
	cvalInstr := condDef.Instr
	cvalInstr.MarkLowered()

	// We need to copy the execution context to a temp register, because if it's spilled,
	// it might end up being reloaded inside the exiting branch.
	execCtxTmp := m.copyToTmp(execCtx)

	x, y, c := cvalInstr.IcmpData()
	xx, yy := m.c.ValueDefinition(x), m.c.ValueDefinition(y)
	if !m.tryLowerBandToFlag(xx, yy) {
		m.lowerIcmpToFlag(xx, yy, x.Type() == ssa.TypeI64)
	}

	jmpIf := m.allocateInstr()
	m.insert(jmpIf)
	l := m.lowerExitWithCode(execCtxTmp, code)
	jmpIf.asJmpIf(condFromSSAIntCmpCond(c).invert(), newOperandLabel(l))
}

func (m *machine) tryLowerBandToFlag(x, y backend.SSAValueDefinition) (ok bool) {
	var target backend.SSAValueDefinition
	var got bool
	if x.IsFromInstr() && x.Instr.Constant() && x.Instr.ConstantVal() == 0 {
		if m.c.MatchInstr(y, ssa.OpcodeBand) {
			target = y
			got = true
		}
	}

	if y.IsFromInstr() && y.Instr.Constant() && y.Instr.ConstantVal() == 0 {
		if m.c.MatchInstr(x, ssa.OpcodeBand) {
			target = x
			got = true
		}
	}

	if !got {
		return false
	}

	bandInstr := target.Instr
	bandX, bandY := bandInstr.Arg2()

	xx := m.getOperand_Reg(m.c.ValueDefinition(bandX))
	yy := m.getOperand_Mem_Imm32_Reg(m.c.ValueDefinition(bandY))
	test := m.allocateInstr().asCmpRmiR(false, yy, xx.reg(), bandX.Type() == ssa.TypeI64)
	m.insert(test)
	bandInstr.MarkLowered()
	return true
}

func (m *machine) allocateExitInstructions(execCtx, exitCodeReg regalloc.VReg) (saveRsp, saveRbp, setExitCode *instruction) {
	saveRsp = m.allocateInstr().asMovRM(
		rspVReg,
		newOperandMem(m.newAmodeImmReg(wazevoapi.ExecutionContextOffsetStackPointerBeforeGoCall.U32(), execCtx)),
		8,
	)

	saveRbp = m.allocateInstr().asMovRM(
		rbpVReg,
		newOperandMem(m.newAmodeImmReg(wazevoapi.ExecutionContextOffsetFramePointerBeforeGoCall.U32(), execCtx)),
		8,
	)
	setExitCode = m.allocateInstr().asMovRM(
		exitCodeReg,
		newOperandMem(m.newAmodeImmReg(wazevoapi.ExecutionContextOffsetExitCodeOffset.U32(), execCtx)),
		4,
	)
	return
}

func (m *machine) lowerExitWithCode(execCtx regalloc.VReg, code wazevoapi.ExitCode) (afterLabel label) {
	exitCodeReg := rbpVReg
	saveRsp, saveRbp, setExitCode := m.allocateExitInstructions(execCtx, exitCodeReg)

	// Set save RSP, RBP, and write exit code.
	m.insert(saveRsp)
	m.insert(saveRbp)
	m.lowerIconst(exitCodeReg, uint64(code), false)
	m.insert(setExitCode)

	ripReg := rbpVReg

	// Next is to save the current address for stack unwinding.
	nop, currentAddrLabel := m.allocateBrTarget()
	m.insert(nop)
	readRip := m.allocateInstr().asLEA(newOperandLabel(currentAddrLabel), ripReg)
	m.insert(readRip)
	saveRip := m.allocateInstr().asMovRM(
		ripReg,
		newOperandMem(m.newAmodeImmReg(wazevoapi.ExecutionContextOffsetGoCallReturnAddress.U32(), execCtx)),
		8,
	)
	m.insert(saveRip)

	// Finally exit.
	exitSq := m.allocateExitSeq(execCtx)
	m.insert(exitSq)

	// Return the label for continuation.
	continuation, afterLabel := m.allocateBrTarget()
	m.insert(continuation)
	return afterLabel
}

func (m *machine) lowerAluRmiROp(si *ssa.Instruction, op aluRmiROpcode) {
	x, y := si.Arg2()
	if !x.Type().IsInt() {
		panic("BUG?")
	}

	_64 := x.Type().Bits() == 64

	xDef, yDef := m.c.ValueDefinition(x), m.c.ValueDefinition(y)

	// TODO: commutative args can be swapped if one of them is an immediate.
	rn := m.getOperand_Reg(xDef)
	rm := m.getOperand_Mem_Imm32_Reg(yDef)
	rd := m.c.VRegOf(si.Return())

	// rn is being overwritten, so we first copy its value to a temp register,
	// in case it is referenced again later.
	tmp := m.copyToTmp(rn.reg())

	alu := m.allocateInstr()
	alu.asAluRmiR(op, rm, tmp, _64)
	m.insert(alu)

	// tmp now contains the result, we copy it to the dest register.
	m.copyTo(tmp, rd)
}

func (m *machine) lowerShiftR(si *ssa.Instruction, op shiftROp) {
	x, amt := si.Arg2()
	if !x.Type().IsInt() {
		panic("BUG?")
	}
	_64 := x.Type().Bits() == 64

	xDef, amtDef := m.c.ValueDefinition(x), m.c.ValueDefinition(amt)

	opAmt := m.getOperand_Imm32_Reg(amtDef)
	rx := m.getOperand_Reg(xDef)
	rd := m.c.VRegOf(si.Return())

	// rx is being overwritten, so we first copy its value to a temp register,
	// in case it is referenced again later.
	tmpDst := m.copyToTmp(rx.reg())

	if opAmt.kind == operandKindReg {
		// If opAmt is a register we must copy its value to rcx,
		// because shiftR encoding mandates that the shift amount is in rcx.
		m.copyTo(opAmt.reg(), rcxVReg)

		alu := m.allocateInstr()
		alu.asShiftR(op, newOperandReg(rcxVReg), tmpDst, _64)
		m.insert(alu)

	} else {
		alu := m.allocateInstr()
		alu.asShiftR(op, opAmt, tmpDst, _64)
		m.insert(alu)
	}

	// tmp now contains the result, we copy it to the dest register.
	m.copyTo(tmpDst, rd)
}

func (m *machine) lowerXmmRmR(instr *ssa.Instruction) {
	x, y := instr.Arg2()
	if !x.Type().IsFloat() {
		panic("BUG?")
	}
	_64 := x.Type().Bits() == 64

	var op sseOpcode
	if _64 {
		switch instr.Opcode() {
		case ssa.OpcodeFadd:
			op = sseOpcodeAddsd
		case ssa.OpcodeFsub:
			op = sseOpcodeSubsd
		case ssa.OpcodeFmul:
			op = sseOpcodeMulsd
		case ssa.OpcodeFdiv:
			op = sseOpcodeDivsd
		default:
			panic("BUG")
		}
	} else {
		switch instr.Opcode() {
		case ssa.OpcodeFadd:
			op = sseOpcodeAddss
		case ssa.OpcodeFsub:
			op = sseOpcodeSubss
		case ssa.OpcodeFmul:
			op = sseOpcodeMulss
		case ssa.OpcodeFdiv:
			op = sseOpcodeDivss
		default:
			panic("BUG")
		}
	}

	xDef, yDef := m.c.ValueDefinition(x), m.c.ValueDefinition(y)
	rn := m.getOperand_Reg(yDef)
	rm := m.getOperand_Reg(xDef)
	rd := m.c.VRegOf(instr.Return())

	// rm is being overwritten, so we first copy its value to a temp register,
	// in case it is referenced again later.
	tmp := m.copyToTmp(rm.reg())

	xmm := m.allocateInstr().asXmmRmR(op, rn, tmp)
	m.insert(xmm)

	m.copyTo(tmp, rd)
}

func (m *machine) lowerSqrt(instr *ssa.Instruction) {
	x := instr.Arg()
	if !x.Type().IsFloat() {
		panic("BUG")
	}
	_64 := x.Type().Bits() == 64
	var op sseOpcode
	if _64 {
		op = sseOpcodeSqrtsd
	} else {
		op = sseOpcodeSqrtss
	}

	xDef := m.c.ValueDefinition(x)
	rm := m.getOperand_Mem_Reg(xDef)
	rd := m.c.VRegOf(instr.Return())

	xmm := m.allocateInstr().asXmmUnaryRmR(op, rm, rd)
	m.insert(xmm)
}

func (m *machine) lowerFabsFneg(instr *ssa.Instruction) {
	x := instr.Arg()
	if !x.Type().IsFloat() {
		panic("BUG")
	}
	_64 := x.Type().Bits() == 64
	var op sseOpcode
	var mask uint64
	if _64 {
		switch instr.Opcode() {
		case ssa.OpcodeFabs:
			mask, op = 0x7fffffffffffffff, sseOpcodeAndpd
		case ssa.OpcodeFneg:
			mask, op = 0x8000000000000000, sseOpcodeXorpd
		}
	} else {
		switch instr.Opcode() {
		case ssa.OpcodeFabs:
			mask, op = 0x7fffffff, sseOpcodeAndps
		case ssa.OpcodeFneg:
			mask, op = 0x80000000, sseOpcodeXorps
		}
	}

	tmp := m.c.AllocateVReg(x.Type())

	xDef := m.c.ValueDefinition(x)
	rm := m.getOperand_Reg(xDef)
	rd := m.c.VRegOf(instr.Return())

	m.lowerFconst(tmp, mask, _64)

	xmm := m.allocateInstr().asXmmRmR(op, rm, tmp)
	m.insert(xmm)

	m.copyTo(tmp, rd)
}

func (m *machine) lowerStore(si *ssa.Instruction) {
	value, ptr, offset, storeSizeInBits := si.StoreData()
	rm := m.getOperand_Reg(m.c.ValueDefinition(value))
	mem := newOperandMem(m.lowerToAddressMode(ptr, offset))

	store := m.allocateInstr()
	switch value.Type() {
	case ssa.TypeI32:
		store.asMovRM(rm.reg(), mem, storeSizeInBits/8)
	case ssa.TypeI64:
		store.asMovRM(rm.reg(), mem, storeSizeInBits/8)
	case ssa.TypeF32:
		store.asXmmMovRM(sseOpcodeMovss, rm.reg(), mem)
	case ssa.TypeF64:
		store.asXmmMovRM(sseOpcodeMovsd, rm.reg(), mem)
	case ssa.TypeV128:
		store.asXmmMovRM(sseOpcodeMovdqu, rm.reg(), mem)
	default:
		panic("BUG")
	}
	m.insert(store)
}

func (m *machine) lowerCall(si *ssa.Instruction) {
	isDirectCall := si.Opcode() == ssa.OpcodeCall
	var indirectCalleePtr ssa.Value
	var directCallee ssa.FuncRef
	var sigID ssa.SignatureID
	var args []ssa.Value
	var isMemmove bool
	if isDirectCall {
		directCallee, sigID, args = si.CallData()
	} else {
		indirectCalleePtr, sigID, args, isMemmove = si.CallIndirectData()
	}
	calleeABI := m.c.GetFunctionABI(m.c.SSABuilder().ResolveSignature(sigID))

	stackSlotSize := int64(calleeABI.AlignedArgResultStackSlotSize())
	if m.maxRequiredStackSizeForCalls < stackSlotSize+16 {
		m.maxRequiredStackSizeForCalls = stackSlotSize + 16 // 16 == return address + RBP.
	}

	// Note: See machine.SetupPrologue for the stack layout.
	// The stack pointer decrease/increase will be inserted later in the compilation.

	for i, arg := range args {
		reg := m.c.VRegOf(arg)
		def := m.c.ValueDefinition(arg)
		m.callerGenVRegToFunctionArg(calleeABI, i, reg, def, stackSlotSize)
	}

	if isMemmove {
		// Go's memmove *might* use all xmm0-xmm15, so we need to release them.
		// https://github.com/golang/go/blob/49d42128fd8594c172162961ead19ac95e247d24/src/cmd/compile/abi-internal.md#architecture-specifics
		// https://github.com/golang/go/blob/49d42128fd8594c172162961ead19ac95e247d24/src/runtime/memmove_amd64.s#L271-L286
		for i := regalloc.RealReg(0); i < 16; i++ {
			m.insert(m.allocateInstr().asDefineUninitializedReg(regInfo.RealRegToVReg[xmm0+i]))
		}
		// Since Go 1.24 it may also use DX, which is not reserved for the function call's 3 args.
		// https://github.com/golang/go/blob/go1.24.0/src/runtime/memmove_amd64.s#L123
		m.insert(m.allocateInstr().asDefineUninitializedReg(regInfo.RealRegToVReg[rdx]))
	}

	if isDirectCall {
		call := m.allocateInstr().asCall(directCallee, calleeABI)
		m.insert(call)
	} else {
		ptrOp := m.getOperand_Mem_Reg(m.c.ValueDefinition(indirectCalleePtr))
		callInd := m.allocateInstr().asCallIndirect(ptrOp, calleeABI)
		m.insert(callInd)
	}

	if isMemmove {
		for i := regalloc.RealReg(0); i < 16; i++ {
			m.insert(m.allocateInstr().asNopUseReg(regInfo.RealRegToVReg[xmm0+i]))
		}
		m.insert(m.allocateInstr().asNopUseReg(regInfo.RealRegToVReg[rdx]))
	}

	var index int
	r1, rs := si.Returns()
	if r1.Valid() {
		m.callerGenFunctionReturnVReg(calleeABI, 0, m.c.VRegOf(r1), stackSlotSize)
		index++
	}

	for _, r := range rs {
		m.callerGenFunctionReturnVReg(calleeABI, index, m.c.VRegOf(r), stackSlotSize)
		index++
	}
}

// callerGenVRegToFunctionArg is the opposite of GenFunctionArgToVReg, which is used to generate the
// caller side of the function call.
func (m *machine) callerGenVRegToFunctionArg(a *backend.FunctionABI, argIndex int, reg regalloc.VReg, def backend.SSAValueDefinition, stackSlotSize int64) {
	arg := &a.Args[argIndex]
	if def.IsFromInstr() {
		// Constant instructions are inlined.
		if inst := def.Instr; inst.Constant() {
			m.insertLoadConstant(inst, reg)
		}
	}
	if arg.Kind == backend.ABIArgKindReg {
		m.InsertMove(arg.Reg, reg, arg.Type)
	} else {
		store := m.allocateInstr()
		mem := newOperandMem(m.newAmodeImmReg(
			// -stackSlotSize because the stack pointer is not yet decreased.
			uint32(arg.Offset-stackSlotSize), rspVReg))
		switch arg.Type {
		case ssa.TypeI32:
			store.asMovRM(reg, mem, 4)
		case ssa.TypeI64:
			store.asMovRM(reg, mem, 8)
		case ssa.TypeF32:
			store.asXmmMovRM(sseOpcodeMovss, reg, mem)
		case ssa.TypeF64:
			store.asXmmMovRM(sseOpcodeMovsd, reg, mem)
		case ssa.TypeV128:
			store.asXmmMovRM(sseOpcodeMovdqu, reg, mem)
		default:
			panic("BUG")
		}
		m.insert(store)
	}
}

func (m *machine) callerGenFunctionReturnVReg(a *backend.FunctionABI, retIndex int, reg regalloc.VReg, stackSlotSize int64) {
	r := &a.Rets[retIndex]
	if r.Kind == backend.ABIArgKindReg {
		m.InsertMove(reg, r.Reg, r.Type)
	} else {
		load := m.allocateInstr()
		mem := newOperandMem(m.newAmodeImmReg(
			// -stackSlotSize because the stack pointer is not yet decreased.
			uint32(a.ArgStackSize+r.Offset-stackSlotSize), rspVReg))
		switch r.Type {
		case ssa.TypeI32:
			load.asMovzxRmR(extModeLQ, mem, reg)
		case ssa.TypeI64:
			load.asMov64MR(mem, reg)
		case ssa.TypeF32:
			load.asXmmUnaryRmR(sseOpcodeMovss, mem, reg)
		case ssa.TypeF64:
			load.asXmmUnaryRmR(sseOpcodeMovsd, mem, reg)
		case ssa.TypeV128:
			load.asXmmUnaryRmR(sseOpcodeMovdqu, mem, reg)
		default:
			panic("BUG")
		}
		m.insert(load)
	}
}

// InsertMove implements backend.Machine.
func (m *machine) InsertMove(dst, src regalloc.VReg, typ ssa.Type) {
	switch typ {
	case ssa.TypeI32, ssa.TypeI64:
		i := m.allocateInstr().asMovRR(src, dst, typ.Bits() == 64)
		m.insert(i)
	case ssa.TypeF32, ssa.TypeF64, ssa.TypeV128:
		var op sseOpcode
		switch typ {
		case ssa.TypeF32:
			op = sseOpcodeMovss
		case ssa.TypeF64:
			op = sseOpcodeMovsd
		case ssa.TypeV128:
			op = sseOpcodeMovdqa
		}
		i := m.allocateInstr().asXmmUnaryRmR(op, newOperandReg(src), dst)
		m.insert(i)
	default:
		panic("BUG")
	}
}

// Format implements backend.Machine.
func (m *machine) Format() string {
	begins := map[*instruction]label{}
	for l := label(0); l < m.nextLabel; l++ {
		pos := m.labelPositionPool.Get(int(l))
		if pos != nil {
			begins[pos.begin] = l
		}
	}

	var lines []string
	for cur := m.rootInstr; cur != nil; cur = cur.next {
		if l, ok := begins[cur]; ok {
			var labelStr string
			if l <= m.maxSSABlockID {
				labelStr = fmt.Sprintf("%s (SSA Block: blk%d):", l, l)
			} else {
				labelStr = fmt.Sprintf("%s:", l)
			}
			lines = append(lines, labelStr)
		}
		if cur.kind == nop0 {
			continue
		}
		lines = append(lines, "\t"+cur.String())
	}
	for _, vc := range m.consts {
		if vc._var == nil {
			lines = append(lines, fmt.Sprintf("%s: const [%d %d]", vc.label, vc.lo, vc.hi))
		} else {
			lines = append(lines, fmt.Sprintf("%s: const %#x", vc.label, vc._var))
		}
	}
	return "\n" + strings.Join(lines, "\n") + "\n"
}

func (m *machine) encodeWithoutSSA(root *instruction) {
	m.labelResolutionPends = m.labelResolutionPends[:0]
	bufPtr := m.c.BufPtr()
	for cur := root; cur != nil; cur = cur.next {
		offset := int64(len(*bufPtr))
		if cur.kind == nop0 {
			l := cur.nop0Label()
			pos := m.labelPositionPool.Get(int(l))
			if pos != nil {
				pos.binaryOffset = offset
			}
		}

		needLabelResolution := cur.encode(m.c)
		if needLabelResolution {
			m.labelResolutionPends = append(m.labelResolutionPends,
				labelResolutionPend{instr: cur, imm32Offset: int64(len(*bufPtr)) - 4},
			)
		}
	}

	for i := range m.labelResolutionPends {
		p := &m.labelResolutionPends[i]
		switch p.instr.kind {
		case jmp, jmpIf, lea:
			target := p.instr.jmpLabel()
			targetOffset := m.labelPositionPool.Get(int(target)).binaryOffset
			imm32Offset := p.imm32Offset
			jmpOffset := int32(targetOffset - (p.imm32Offset + 4)) // +4 because RIP points to the next instruction.
			binary.LittleEndian.PutUint32((*bufPtr)[imm32Offset:], uint32(jmpOffset))
		default:
			panic("BUG")
		}
	}
}

// Encode implements backend.Machine Encode.
func (m *machine) Encode(ctx context.Context) (err error) {
	bufPtr := m.c.BufPtr()

	var fn string
	var fnIndex int
	var labelPosToLabel map[*labelPosition]label
	if wazevoapi.PerfMapEnabled {
		fn = wazevoapi.GetCurrentFunctionName(ctx)
		labelPosToLabel = make(map[*labelPosition]label)
		for i := 0; i <= m.labelPositionPool.MaxIDEncountered(); i++ {
			pos := m.labelPositionPool.Get(i)
			labelPosToLabel[pos] = label(i)
		}
		fnIndex = wazevoapi.GetCurrentFunctionIndex(ctx)
	}

	m.labelResolutionPends = m.labelResolutionPends[:0]
	for _, pos := range m.orderedSSABlockLabelPos {
		offset := int64(len(*bufPtr))
		pos.binaryOffset = offset
		for cur := pos.begin; cur != pos.end.next; cur = cur.next {
			offset := int64(len(*bufPtr))

			switch cur.kind {
			case nop0:
				l := cur.nop0Label()
				if pos := m.labelPositionPool.Get(int(l)); pos != nil {
					pos.binaryOffset = offset
				}
			case sourceOffsetInfo:
				m.c.AddSourceOffsetInfo(offset, cur.sourceOffsetInfo())
			}

			needLabelResolution := cur.encode(m.c)
			if needLabelResolution {
				m.labelResolutionPends = append(m.labelResolutionPends,
					labelResolutionPend{instr: cur, instrOffset: offset, imm32Offset: int64(len(*bufPtr)) - 4},
				)
			}
		}

		if wazevoapi.PerfMapEnabled {
			l := labelPosToLabel[pos]
			size := int64(len(*bufPtr)) - offset
			wazevoapi.PerfMap.AddModuleEntry(fnIndex, offset, uint64(size), fmt.Sprintf("%s:::::%s", fn, l))
		}
	}

	for i := range m.consts {
		offset := int64(len(*bufPtr))
		vc := &m.consts[i]
		vc.labelPos.binaryOffset = offset
		if vc._var == nil {
			lo, hi := vc.lo, vc.hi
			m.c.Emit8Bytes(lo)
			m.c.Emit8Bytes(hi)
		} else {
			for _, b := range vc._var {
				m.c.EmitByte(b)
			}
		}
	}

	buf := *bufPtr
	for i := range m.labelResolutionPends {
		p := &m.labelResolutionPends[i]
		switch p.instr.kind {
		case jmp, jmpIf, lea, xmmUnaryRmR:
			target := p.instr.jmpLabel()
			targetOffset := m.labelPositionPool.Get(int(target)).binaryOffset
			imm32Offset := p.imm32Offset
			jmpOffset := int32(targetOffset - (p.imm32Offset + 4)) // +4 because RIP points to the next instruction.
			binary.LittleEndian.PutUint32(buf[imm32Offset:], uint32(jmpOffset))
		case jmpTableIsland:
			tableBegin := p.instrOffset
			// Each entry is the offset from the beginning of the jmpTableIsland instruction in 8 bytes.
			targets := m.jmpTableTargets[p.instr.u1]
			for i, l := range targets {
				targetOffset := m.labelPositionPool.Get(int(l)).binaryOffset
				jmpOffset := targetOffset - tableBegin
				binary.LittleEndian.PutUint64(buf[tableBegin+int64(i)*8:], uint64(jmpOffset))
			}
		default:
			panic("BUG")
		}
	}
	return
}

// ResolveRelocations implements backend.Machine.
func (m *machine) ResolveRelocations(refToBinaryOffset []int, _ int, binary []byte, relocations []backend.RelocationInfo, _ []int) {
	for _, r := range relocations {
		offset := r.Offset
		calleeFnOffset := refToBinaryOffset[r.FuncRef]
		// offset is the offset of the last 4 bytes of the call instruction.
		callInstrOffsetBytes := binary[offset : offset+4]
		diff := int64(calleeFnOffset) - (offset + 4) // +4 because we want the offset of the next instruction (In x64, RIP always points to the next instruction).
		callInstrOffsetBytes[0] = byte(diff)
		callInstrOffsetBytes[1] = byte(diff >> 8)
		callInstrOffsetBytes[2] = byte(diff >> 16)
		callInstrOffsetBytes[3] = byte(diff >> 24)
	}
}

// CallTrampolineIslandInfo implements backend.Machine CallTrampolineIslandInfo.
func (m *machine) CallTrampolineIslandInfo(_ int) (_, _ int, _ error) { return }

func (m *machine) lowerIcmpToFlag(xd, yd backend.SSAValueDefinition, _64 bool) {
	x := m.getOperand_Reg(xd)
	y := m.getOperand_Mem_Imm32_Reg(yd)
	cmp := m.allocateInstr().asCmpRmiR(true, y, x.reg(), _64)
	m.insert(cmp)
}

func (m *machine) lowerFcmpToFlags(instr *ssa.Instruction) (f1, f2 cond, and bool) {
	x, y, c := instr.FcmpData()
	switch c {
	case ssa.FloatCmpCondEqual:
		f1, f2 = condNP, condZ
		and = true
	case ssa.FloatCmpCondNotEqual:
		f1, f2 = condP, condNZ
	case ssa.FloatCmpCondLessThan:
		f1 = condFromSSAFloatCmpCond(ssa.FloatCmpCondGreaterThan)
		f2 = condInvalid
		x, y = y, x
	case ssa.FloatCmpCondLessThanOrEqual:
		f1 = condFromSSAFloatCmpCond(ssa.FloatCmpCondGreaterThanOrEqual)
		f2 = condInvalid
		x, y = y, x
	default:
		f1 = condFromSSAFloatCmpCond(c)
		f2 = condInvalid
	}

	var opc sseOpcode
	if x.Type() == ssa.TypeF32 {
		opc = sseOpcodeUcomiss
	} else {
		opc = sseOpcodeUcomisd
	}

	xr := m.getOperand_Reg(m.c.ValueDefinition(x))
	yr := m.getOperand_Mem_Reg(m.c.ValueDefinition(y))
	m.insert(m.allocateInstr().asXmmCmpRmR(opc, yr, xr.reg()))
	return
}

// allocateInstr allocates an instruction.
func (m *machine) allocateInstr() *instruction {
	instr := m.instrPool.Allocate()
	if !m.regAllocStarted {
		instr.addedBeforeRegAlloc = true
	}
	return instr
}

func (m *machine) allocateNop() *instruction {
	instr := m.allocateInstr()
	instr.kind = nop0
	return instr
}

func (m *machine) insert(i *instruction) {
	m.pendingInstructions = append(m.pendingInstructions, i)
}

func (m *machine) allocateBrTarget() (nop *instruction, l label) { //nolint
	l, pos := m.allocateLabel()
	nop = m.allocateInstr()
	nop.asNop0WithLabel(l)
	pos.begin, pos.end = nop, nop
	return
}

func (m *machine) allocateLabel() (label, *labelPosition) {
	l := m.nextLabel
	pos := m.labelPositionPool.GetOrAllocate(int(l))
	m.nextLabel++
	return l, pos
}

func (m *machine) getVRegSpillSlotOffsetFromSP(id regalloc.VRegID, size byte) int64 {
	offset, ok := m.spillSlots[id]
	if !ok {
		offset = m.spillSlotSize
		m.spillSlots[id] = offset
		m.spillSlotSize += int64(size)
	}
	return offset
}

func (m *machine) copyTo(src regalloc.VReg, dst regalloc.VReg) {
	mov := m.allocateInstr()
	if src.RegType() == regalloc.RegTypeInt {
		mov.asMovRR(src, dst, true)
	} else {
		mov.asXmmUnaryRmR(sseOpcodeMovdqu, newOperandReg(src), dst)
	}
	m.insert(mov)
}

func (m *machine) copyToTmp(v regalloc.VReg) regalloc.VReg {
	typ := m.c.TypeOf(v)
	tmp := m.c.AllocateVReg(typ)
	m.copyTo(v, tmp)
	return tmp
}

func (m *machine) requiredStackSize() int64 {
	return m.maxRequiredStackSizeForCalls +
		m.frameSize() +
		16 + // Need for stack checking.
		16 // return address and the caller RBP.
}

func (m *machine) frameSize() int64 {
	s := m.clobberedRegSlotSize() + m.spillSlotSize
	if s&0xf != 0 {
		panic(fmt.Errorf("BUG: frame size %d is not 16-byte aligned", s))
	}
	return s
}

func (m *machine) clobberedRegSlotSize() int64 {
	return int64(len(m.clobberedRegs) * 16)
}

func (m *machine) lowerIDivRem(si *ssa.Instruction, isDiv bool, signed bool) {
	x, y, execCtx := si.Arg3()

	dividend := m.getOperand_Reg(m.c.ValueDefinition(x))
	divisor := m.getOperand_Reg(m.c.ValueDefinition(y))
	ctxVReg := m.c.VRegOf(execCtx)
	tmpGp := m.c.AllocateVReg(si.Return().Type())

	m.copyTo(dividend.reg(), raxVReg)
	m.insert(m.allocateInstr().asDefineUninitializedReg(rdxVReg))
	m.insert(m.allocateInstr().asDefineUninitializedReg(tmpGp))
	seq := m.allocateInstr().asIdivRemSequence(ctxVReg, divisor.reg(), tmpGp, isDiv, signed, x.Type().Bits() == 64)
	m.insert(seq)
	rd := m.c.VRegOf(si.Return())
	if isDiv {
		m.copyTo(raxVReg, rd)
	} else {
		m.copyTo(rdxVReg, rd)
	}
}

func (m *machine) lowerIDivRemSequenceAfterRegAlloc(i *instruction) {
	execCtx, divisor, tmpGp, isDiv, signed, _64 := i.idivRemSequenceData()

	dividend := raxVReg

	// Ensure yr is not zero.
	test := m.allocateInstr()
	test.asCmpRmiR(false, newOperandReg(divisor), divisor, _64)
	m.insert(test)

	jnz := m.allocateInstr()
	m.insert(jnz)

	nz := m.lowerExitWithCode(execCtx, wazevoapi.ExitCodeIntegerDivisionByZero)

	// If not zero, we can proceed with the division.
	jnz.asJmpIf(condNZ, newOperandLabel(nz))

	var ifRemNeg1 *instruction
	if signed {
		var neg1 uint64
		if _64 {
			neg1 = 0xffffffffffffffff
		} else {
			neg1 = 0xffffffff
		}
		m.lowerIconst(tmpGp, neg1, _64)

		if isDiv {
			// For signed division, we have to have branches for "math.MinInt{32,64} / -1"
			// case which results in the floating point exception via division error as
			// the resulting value exceeds the maximum of signed int.

			// First, we check if the divisor is -1.
			cmp := m.allocateInstr()
			cmp.asCmpRmiR(true, newOperandReg(tmpGp), divisor, _64)
			m.insert(cmp)

			ifNotNeg1 := m.allocateInstr()
			m.insert(ifNotNeg1)

			var minInt uint64
			if _64 {
				minInt = 0x8000000000000000
			} else {
				minInt = 0x80000000
			}
			m.lowerIconst(tmpGp, minInt, _64)

			// Next we check if the quotient is the most negative value for the signed integer, i.e.
			// if we are trying to do (math.MinInt32 / -1) or (math.MinInt64 / -1) respectively.
			cmp2 := m.allocateInstr()
			cmp2.asCmpRmiR(true, newOperandReg(tmpGp), dividend, _64)
			m.insert(cmp2)

			ifNotMinInt := m.allocateInstr()
			m.insert(ifNotMinInt)

			// Trap if we are trying to do (math.MinInt32 / -1) or (math.MinInt64 / -1),
			// as that is the overflow in division as the result becomes 2^31 which is larger than
			// the maximum of signed 32-bit int (2^31-1).
			end := m.lowerExitWithCode(execCtx, wazevoapi.ExitCodeIntegerOverflow)
			ifNotNeg1.asJmpIf(condNZ, newOperandLabel(end))
			ifNotMinInt.asJmpIf(condNZ, newOperandLabel(end))
		} else {
			// If it is remainder, zeros DX register and compare the divisor to -1.
			xor := m.allocateInstr().asZeros(rdxVReg)
			m.insert(xor)

			// We check if the divisor is -1.
			cmp := m.allocateInstr()
			cmp.asCmpRmiR(true, newOperandReg(tmpGp), divisor, _64)
			m.insert(cmp)

			ifRemNeg1 = m.allocateInstr()
			m.insert(ifRemNeg1)
		}

		// Sign-extend DX register to have 2*x.Type().Bits() dividend over DX and AX registers.
		sed := m.allocateInstr()
		sed.asSignExtendData(_64)
		m.insert(sed)
	} else {
		// Zeros DX register to have 2*x.Type().Bits() dividend over DX and AX registers.
		zeros := m.allocateInstr().asZeros(rdxVReg)
		m.insert(zeros)
	}

	div := m.allocateInstr()
	div.asDiv(newOperandReg(divisor), signed, _64)
	m.insert(div)

	nop, end := m.allocateBrTarget()
	m.insert(nop)
	// If we are compiling a Rem instruction, when the divisor is -1 we land at the end of the function.
	if ifRemNeg1 != nil {
		ifRemNeg1.asJmpIf(condZ, newOperandLabel(end))
	}
}

func (m *machine) lowerRound(instr *ssa.Instruction, imm roundingMode) {
	x := instr.Arg()
	if !x.Type().IsFloat() {
		panic("BUG?")
	}
	var op sseOpcode
	if x.Type().Bits() == 64 {
		op = sseOpcodeRoundsd
	} else {
		op = sseOpcodeRoundss
	}

	xDef := m.c.ValueDefinition(x)
	rm := m.getOperand_Mem_Reg(xDef)
	rd := m.c.VRegOf(instr.Return())

	xmm := m.allocateInstr().asXmmUnaryRmRImm(op, uint8(imm), rm, rd)
	m.insert(xmm)
}

func (m *machine) lowerFminFmax(instr *ssa.Instruction) {
	x, y := instr.Arg2()
	if !x.Type().IsFloat() {
		panic("BUG?")
	}

	_64 := x.Type().Bits() == 64
	isMin := instr.Opcode() == ssa.OpcodeFmin
	var minMaxOp sseOpcode

	switch {
	case _64 && isMin:
		minMaxOp = sseOpcodeMinpd
	case _64 && !isMin:
		minMaxOp = sseOpcodeMaxpd
	case !_64 && isMin:
		minMaxOp = sseOpcodeMinps
	case !_64 && !isMin:
		minMaxOp = sseOpcodeMaxps
	}

	xDef, yDef := m.c.ValueDefinition(x), m.c.ValueDefinition(y)
	rm := m.getOperand_Reg(xDef)
	// We cannot ensure that y is aligned to 16 bytes, so we have to use it on reg.
	rn := m.getOperand_Reg(yDef)
	rd := m.c.VRegOf(instr.Return())

	tmp := m.copyToTmp(rm.reg())

	// Check if this is (either x1 or x2 is NaN) or (x1 equals x2) case.
	cmp := m.allocateInstr()
	if _64 {
		cmp.asXmmCmpRmR(sseOpcodeUcomisd, rn, tmp)
	} else {
		cmp.asXmmCmpRmR(sseOpcodeUcomiss, rn, tmp)
	}
	m.insert(cmp)

	// At this point, we have the three cases of conditional flags below
	// (See https://www.felixcloutier.com/x86/ucomiss#operation for detail.)
	//
	// 1) Two values are NaN-free and different: All flags are cleared.
	// 2) Two values are NaN-free and equal: Only ZF flags is set.
	// 3) One of Two values is NaN: ZF, PF and CF flags are set.

	// Jump instruction to handle 1) case by checking the ZF flag
	// as ZF is only set for 2) and 3) cases.
	nanFreeOrDiffJump := m.allocateInstr()
	m.insert(nanFreeOrDiffJump)

	// Start handling 2) and 3).

	// Jump if one of two values is NaN by checking the parity flag (PF).
	ifIsNan := m.allocateInstr()
	m.insert(ifIsNan)

	// Start handling 2) NaN-free and equal.

	// Before we exit this case, we have to ensure that positive zero (or negative zero for min instruction) is
	// returned if two values are positive and negative zeros.
	var op sseOpcode
	switch {
	case !_64 && isMin:
		op = sseOpcodeOrps
	case _64 && isMin:
		op = sseOpcodeOrpd
	case !_64 && !isMin:
		op = sseOpcodeAndps
	case _64 && !isMin:
		op = sseOpcodeAndpd
	}
	orAnd := m.allocateInstr()
	orAnd.asXmmRmR(op, rn, tmp)
	m.insert(orAnd)

	// Done, jump to end.
	sameExitJump := m.allocateInstr()
	m.insert(sameExitJump)

	// Start handling 3) either is NaN.
	isNanTarget, isNan := m.allocateBrTarget()
	m.insert(isNanTarget)
	ifIsNan.asJmpIf(condP, newOperandLabel(isNan))

	// We emit the ADD instruction to produce the NaN in tmp.
	add := m.allocateInstr()
	if _64 {
		add.asXmmRmR(sseOpcodeAddsd, rn, tmp)
	} else {
		add.asXmmRmR(sseOpcodeAddss, rn, tmp)
	}
	m.insert(add)

	// Exit from the NaN case branch.
	nanExitJmp := m.allocateInstr()
	m.insert(nanExitJmp)

	// Start handling 1).
	doMinMaxTarget, doMinMax := m.allocateBrTarget()
	m.insert(doMinMaxTarget)
	nanFreeOrDiffJump.asJmpIf(condNZ, newOperandLabel(doMinMax))

	// Now handle the NaN-free and different values case.
	minMax := m.allocateInstr()
	minMax.asXmmRmR(minMaxOp, rn, tmp)
	m.insert(minMax)

	endNop, end := m.allocateBrTarget()
	m.insert(endNop)
	nanExitJmp.asJmp(newOperandLabel(end))
	sameExitJump.asJmp(newOperandLabel(end))

	m.copyTo(tmp, rd)
}

func (m *machine) lowerFcopysign(instr *ssa.Instruction) {
	x, y := instr.Arg2()
	if !x.Type().IsFloat() {
		panic("BUG")
	}

	_64 := x.Type().Bits() == 64

	xDef, yDef := m.c.ValueDefinition(x), m.c.ValueDefinition(y)
	rm := m.getOperand_Reg(xDef)
	rn := m.getOperand_Reg(yDef)
	rd := m.c.VRegOf(instr.Return())

	// Clear the non-sign bits of src via AND with the mask.
	var opAnd, opOr sseOpcode
	var signMask uint64
	if _64 {
		signMask, opAnd, opOr = 0x8000000000000000, sseOpcodeAndpd, sseOpcodeOrpd
	} else {
		signMask, opAnd, opOr = 0x80000000, sseOpcodeAndps, sseOpcodeOrps
	}

	signBitReg := m.c.AllocateVReg(x.Type())
	m.lowerFconst(signBitReg, signMask, _64)
	nonSignBitReg := m.c.AllocateVReg(x.Type())
	m.lowerFconst(nonSignBitReg, ^signMask, _64)

	// Extract the sign bits of rn.
	and := m.allocateInstr().asXmmRmR(opAnd, rn, signBitReg)
	m.insert(and)

	// Clear the sign bit of dst via AND with the non-sign bit mask.
	xor := m.allocateInstr().asXmmRmR(opAnd, rm, nonSignBitReg)
	m.insert(xor)

	// Copy the sign bits of src to dst via OR.
	or := m.allocateInstr().asXmmRmR(opOr, newOperandReg(signBitReg), nonSignBitReg)
	m.insert(or)

	m.copyTo(nonSignBitReg, rd)
}

func (m *machine) lowerBitcast(instr *ssa.Instruction) {
	x, dstTyp := instr.BitcastData()
	srcTyp := x.Type()
	rn := m.getOperand_Reg(m.c.ValueDefinition(x))
	rd := m.c.VRegOf(instr.Return())
	switch {
	case srcTyp == ssa.TypeF32 && dstTyp == ssa.TypeI32:
		cvt := m.allocateInstr().asXmmToGpr(sseOpcodeMovd, rn.reg(), rd, false)
		m.insert(cvt)
	case srcTyp == ssa.TypeI32 && dstTyp == ssa.TypeF32:
		cvt := m.allocateInstr().asGprToXmm(sseOpcodeMovd, rn, rd, false)
		m.insert(cvt)
	case srcTyp == ssa.TypeF64 && dstTyp == ssa.TypeI64:
		cvt := m.allocateInstr().asXmmToGpr(sseOpcodeMovq, rn.reg(), rd, true)
		m.insert(cvt)
	case srcTyp == ssa.TypeI64 && dstTyp == ssa.TypeF64:
		cvt := m.allocateInstr().asGprToXmm(sseOpcodeMovq, rn, rd, true)
		m.insert(cvt)
	default:
		panic(fmt.Sprintf("invalid bitcast from %s to %s", srcTyp, dstTyp))
	}
}

func (m *machine) lowerFcvtToSint(ctxVReg, rn, rd regalloc.VReg, src64, dst64, sat bool) {
	var tmpXmm regalloc.VReg
	if dst64 {
		tmpXmm = m.c.AllocateVReg(ssa.TypeF64)
	} else {
		tmpXmm = m.c.AllocateVReg(ssa.TypeF32)
	}

	m.insert(m.allocateInstr().asDefineUninitializedReg(tmpXmm))
	tmpGp, tmpGp2 := m.c.AllocateVReg(ssa.TypeI64), m.c.AllocateVReg(ssa.TypeI64)
	m.insert(m.allocateInstr().asDefineUninitializedReg(tmpGp))
	m.insert(m.allocateInstr().asDefineUninitializedReg(tmpGp2))

	m.insert(m.allocateFcvtToSintSequence(ctxVReg, rn, tmpGp, tmpGp2, tmpXmm, src64, dst64, sat))
	m.copyTo(tmpGp, rd)
}

func (m *machine) lowerFcvtToSintSequenceAfterRegalloc(i *instruction) {
	execCtx, src, tmpGp, tmpGp2, tmpXmm, src64, dst64, sat := i.fcvtToSintSequenceData()
	var cmpOp, truncOp sseOpcode
	if src64 {
		cmpOp, truncOp = sseOpcodeUcomisd, sseOpcodeCvttsd2si
	} else {
		cmpOp, truncOp = sseOpcodeUcomiss, sseOpcodeCvttss2si
	}

	trunc := m.allocateInstr()
	trunc.asXmmToGpr(truncOp, src, tmpGp, dst64)
	m.insert(trunc)

	// Check if the dst operand was INT_MIN, by checking it against 1.
	cmp1 := m.allocateInstr()
	cmp1.asCmpRmiR(true, newOperandImm32(1), tmpGp, dst64)
	m.insert(cmp1)

	// If no overflow, then we are done.
	doneTarget, done := m.allocateBrTarget()
	ifNoOverflow := m.allocateInstr()
	ifNoOverflow.asJmpIf(condNO, newOperandLabel(done))
	m.insert(ifNoOverflow)

	// Now, check for NaN.
	cmpNan := m.allocateInstr()
	cmpNan.asXmmCmpRmR(cmpOp, newOperandReg(src), src)
	m.insert(cmpNan)

	// We allocate the "non-nan target" here, but we will insert it later.
	notNanTarget, notNaN := m.allocateBrTarget()
	ifNotNan := m.allocateInstr()
	ifNotNan.asJmpIf(condNP, newOperandLabel(notNaN))
	m.insert(ifNotNan)

	if sat {
		// If NaN and saturating, return 0.
		zeroDst := m.allocateInstr().asZeros(tmpGp)
		m.insert(zeroDst)

		jmpEnd := m.allocateInstr()
		jmpEnd.asJmp(newOperandLabel(done))
		m.insert(jmpEnd)

		// Otherwise:
		m.insert(notNanTarget)

		// Zero-out the tmp register.
		zero := m.allocateInstr().asZeros(tmpXmm)
		m.insert(zero)

		cmpXmm := m.allocateInstr().asXmmCmpRmR(cmpOp, newOperandReg(tmpXmm), src)
		m.insert(cmpXmm)

		// if >= jump to end.
		jmpEnd2 := m.allocateInstr()
		jmpEnd2.asJmpIf(condB, newOperandLabel(done))
		m.insert(jmpEnd2)

		// Otherwise, saturate to INT_MAX.
		if dst64 {
			m.lowerIconst(tmpGp, math.MaxInt64, dst64)
		} else {
			m.lowerIconst(tmpGp, math.MaxInt32, dst64)
		}

	} else {

		// If non-sat, NaN, trap.
		m.lowerExitWithCode(execCtx, wazevoapi.ExitCodeInvalidConversionToInteger)

		// Otherwise, we will jump here.
		m.insert(notNanTarget)

		// jump over trap if src larger than threshold
		condAboveThreshold := condNB

		// The magic constants are various combination of minInt for int[32|64] represented as float[32|64].
		var minInt uint64
		switch {
		case src64 && dst64:
			minInt = 0xc3e0000000000000
		case src64 && !dst64:
			condAboveThreshold = condNBE
			minInt = 0xC1E0_0000_0020_0000
		case !src64 && dst64:
			minInt = 0xDF00_0000
		case !src64 && !dst64:
			minInt = 0xCF00_0000
		}

		loadToGP := m.allocateInstr().asImm(tmpGp2, minInt, src64)
		m.insert(loadToGP)

		movToXmm := m.allocateInstr().asGprToXmm(sseOpcodeMovq, newOperandReg(tmpGp2), tmpXmm, src64)
		m.insert(movToXmm)

		cmpXmm := m.allocateInstr().asXmmCmpRmR(cmpOp, newOperandReg(tmpXmm), src)
		m.insert(cmpXmm)

		jmpIfLarger := m.allocateInstr()
		checkPositiveTarget, checkPositive := m.allocateBrTarget()
		jmpIfLarger.asJmpIf(condAboveThreshold, newOperandLabel(checkPositive))
		m.insert(jmpIfLarger)

		m.lowerExitWithCode(execCtx, wazevoapi.ExitCodeIntegerOverflow)

		// If positive, it was a real overflow.
		m.insert(checkPositiveTarget)

		// Zero out the temp register.
		xorpd := m.allocateInstr()
		xorpd.asXmmRmR(sseOpcodeXorpd, newOperandReg(tmpXmm), tmpXmm)
		m.insert(xorpd)

		pos := m.allocateInstr()
		pos.asXmmCmpRmR(cmpOp, newOperandReg(src), tmpXmm)
		m.insert(pos)

		// If >= jump to end.
		jmp := m.allocateInstr().asJmpIf(condNB, newOperandLabel(done))
		m.insert(jmp)
		m.lowerExitWithCode(execCtx, wazevoapi.ExitCodeIntegerOverflow)
	}

	m.insert(doneTarget)
}

func (m *machine) lowerFcvtToUint(ctxVReg, rn, rd regalloc.VReg, src64, dst64, sat bool) {
	tmpXmm, tmpXmm2 := m.c.AllocateVReg(ssa.TypeF64), m.c.AllocateVReg(ssa.TypeF64)
	m.insert(m.allocateInstr().asDefineUninitializedReg(tmpXmm))
	m.insert(m.allocateInstr().asDefineUninitializedReg(tmpXmm2))
	tmpGp, tmpGp2 := m.c.AllocateVReg(ssa.TypeI64), m.c.AllocateVReg(ssa.TypeI64)
	m.insert(m.allocateInstr().asDefineUninitializedReg(tmpGp))
	m.insert(m.allocateInstr().asDefineUninitializedReg(tmpGp2))

	m.insert(m.allocateFcvtToUintSequence(
		ctxVReg, rn, tmpGp, tmpGp2, tmpXmm, tmpXmm2, src64, dst64, sat,
	))
	m.copyTo(tmpGp, rd)
}

func (m *machine) lowerFcvtToUintSequenceAfterRegalloc(i *instruction) {
	execCtx, src, tmpGp, tmpGp2, tmpXmm, tmpXmm2, src64, dst64, sat := i.fcvtToUintSequenceData()

	var subOp, cmpOp, truncOp sseOpcode
	if src64 {
		subOp, cmpOp, truncOp = sseOpcodeSubsd, sseOpcodeUcomisd, sseOpcodeCvttsd2si
	} else {
		subOp, cmpOp, truncOp = sseOpcodeSubss, sseOpcodeUcomiss, sseOpcodeCvttss2si
	}

	doneTarget, done := m.allocateBrTarget()

	switch {
	case src64 && dst64:
		loadToGP := m.allocateInstr().asImm(tmpGp, 0x43e0000000000000, true)
		m.insert(loadToGP)
		movToXmm := m.allocateInstr().asGprToXmm(sseOpcodeMovq, newOperandReg(tmpGp), tmpXmm, true)
		m.insert(movToXmm)
	case src64 && !dst64:
		loadToGP := m.allocateInstr().asImm(tmpGp, 0x41e0000000000000, true)
		m.insert(loadToGP)
		movToXmm := m.allocateInstr().asGprToXmm(sseOpcodeMovq, newOperandReg(tmpGp), tmpXmm, true)
		m.insert(movToXmm)
	case !src64 && dst64:
		loadToGP := m.allocateInstr().asImm(tmpGp, 0x5f000000, false)
		m.insert(loadToGP)
		movToXmm := m.allocateInstr().asGprToXmm(sseOpcodeMovq, newOperandReg(tmpGp), tmpXmm, false)
		m.insert(movToXmm)
	case !src64 && !dst64:
		loadToGP := m.allocateInstr().asImm(tmpGp, 0x4f000000, false)
		m.insert(loadToGP)
		movToXmm := m.allocateInstr().asGprToXmm(sseOpcodeMovq, newOperandReg(tmpGp), tmpXmm, false)
		m.insert(movToXmm)
	}

	cmp := m.allocateInstr()
	cmp.asXmmCmpRmR(cmpOp, newOperandReg(tmpXmm), src)
	m.insert(cmp)

	// If above `tmp` ("large threshold"), jump to `ifAboveThreshold`
	ifAboveThresholdTarget, ifAboveThreshold := m.allocateBrTarget()
	jmpIfAboveThreshold := m.allocateInstr()
	jmpIfAboveThreshold.asJmpIf(condNB, newOperandLabel(ifAboveThreshold))
	m.insert(jmpIfAboveThreshold)

	ifNotNaNTarget, ifNotNaN := m.allocateBrTarget()
	jmpIfNotNaN := m.allocateInstr()
	jmpIfNotNaN.asJmpIf(condNP, newOperandLabel(ifNotNaN))
	m.insert(jmpIfNotNaN)

	// If NaN, handle the error condition.
	if sat {
		// On NaN, saturating, we just return 0.
		zeros := m.allocateInstr().asZeros(tmpGp)
		m.insert(zeros)

		jmpEnd := m.allocateInstr()
		jmpEnd.asJmp(newOperandLabel(done))
		m.insert(jmpEnd)
	} else {
		// On NaN, non-saturating, we trap.
		m.lowerExitWithCode(execCtx, wazevoapi.ExitCodeInvalidConversionToInteger)
	}

	// If not NaN, land here.
	m.insert(ifNotNaNTarget)

	// Truncation happens here.

	trunc := m.allocateInstr()
	trunc.asXmmToGpr(truncOp, src, tmpGp, dst64)
	m.insert(trunc)

	// Check if the result is negative.
	cmpNeg := m.allocateInstr()
	cmpNeg.asCmpRmiR(true, newOperandImm32(0), tmpGp, dst64)
	m.insert(cmpNeg)

	// If non-neg, jump to end.
	jmpIfNonNeg := m.allocateInstr()
	jmpIfNonNeg.asJmpIf(condNL, newOperandLabel(done))
	m.insert(jmpIfNonNeg)

	if sat {
		// If the input was "small" (< 2**(width -1)), the only way to get an integer
		// overflow is because the input was too small: saturate to the min value, i.e. 0.
		zeros := m.allocateInstr().asZeros(tmpGp)
		m.insert(zeros)

		jmpEnd := m.allocateInstr()
		jmpEnd.asJmp(newOperandLabel(done))
		m.insert(jmpEnd)
	} else {
		// If not saturating, trap.
		m.lowerExitWithCode(execCtx, wazevoapi.ExitCodeIntegerOverflow)
	}

	// If above the threshold, land here.
	m.insert(ifAboveThresholdTarget)

	// tmpDiff := threshold - rn.
	copySrc := m.allocateInstr().asXmmUnaryRmR(sseOpcodeMovdqu, newOperandReg(src), tmpXmm2)
	m.insert(copySrc)

	sub := m.allocateInstr()
	sub.asXmmRmR(subOp, newOperandReg(tmpXmm), tmpXmm2) // must be -0x8000000000000000
	m.insert(sub)

	trunc2 := m.allocateInstr()
	trunc2.asXmmToGpr(truncOp, tmpXmm2, tmpGp, dst64)
	m.insert(trunc2)

	// Check if the result is negative.
	cmpNeg2 := m.allocateInstr().asCmpRmiR(true, newOperandImm32(0), tmpGp, dst64)
	m.insert(cmpNeg2)

	ifNextLargeTarget, ifNextLarge := m.allocateBrTarget()
	jmpIfNextLarge := m.allocateInstr()
	jmpIfNextLarge.asJmpIf(condNL, newOperandLabel(ifNextLarge))
	m.insert(jmpIfNextLarge)

	if sat {
		// The input was "large" (>= maxInt), so the only way to get an integer
		// overflow is because the input was too large: saturate to the max value.
		var maxInt uint64
		if dst64 {
			maxInt = math.MaxUint64
		} else {
			maxInt = math.MaxUint32
		}
		m.lowerIconst(tmpGp, maxInt, dst64)

		jmpToEnd := m.allocateInstr()
		jmpToEnd.asJmp(newOperandLabel(done))
		m.insert(jmpToEnd)
	} else {
		// If not saturating, trap.
		m.lowerExitWithCode(execCtx, wazevoapi.ExitCodeIntegerOverflow)
	}

	m.insert(ifNextLargeTarget)

	var op operand
	if dst64 {
		m.lowerIconst(tmpGp2, 0x8000000000000000, true)
		op = newOperandReg(tmpGp2)
	} else {
		op = newOperandImm32(0x80000000)
	}

	add := m.allocateInstr()
	add.asAluRmiR(aluRmiROpcodeAdd, op, tmpGp, dst64)
	m.insert(add)

	m.insert(doneTarget)
}

func (m *machine) lowerFcvtFromSint(rn, rd operand, src64, dst64 bool) {
	var op sseOpcode
	if dst64 {
		op = sseOpcodeCvtsi2sd
	} else {
		op = sseOpcodeCvtsi2ss
	}

	trunc := m.allocateInstr()
	trunc.asGprToXmm(op, rn, rd.reg(), src64)
	m.insert(trunc)
}

func (m *machine) lowerFcvtFromUint(rn, rd operand, src64, dst64 bool) {
	var op sseOpcode
	if dst64 {
		op = sseOpcodeCvtsi2sd
	} else {
		op = sseOpcodeCvtsi2ss
	}

	// Src is 32 bit, then we just perform the conversion with 64 bit width.
	//
	// See the following link for why we use 64bit conversion for unsigned 32bit integer sources:
	// https://stackoverflow.com/questions/41495498/fpu-operations-generated-by-gcc-during-casting-integer-to-float.
	//
	// Here's the summary:
	// >> CVTSI2SS is indeed designed for converting a signed integer to a scalar single-precision float,
	// >> not an unsigned integer like you have here. So what gives? Well, a 64-bit processor has 64-bit wide
	// >> registers available, so the unsigned 32-bit input values can be stored as signed 64-bit intermediate values,
	// >> which allows CVTSI2SS to be used after all.
	//
	if !src64 {
		// Before we convert, we have to clear the higher 32-bits of the 64-bit register
		// to get the correct result.
		tmp := m.c.AllocateVReg(ssa.TypeI32)
		m.insert(m.allocateInstr().asMovzxRmR(extModeLQ, rn, tmp))
		m.insert(m.allocateInstr().asGprToXmm(op, newOperandReg(tmp), rd.reg(), true))
		return
	}

	// If uint64, we have to do a bit more work.
	endTarget, end := m.allocateBrTarget()

	var tmpXmm regalloc.VReg
	if dst64 {
		tmpXmm = m.c.AllocateVReg(ssa.TypeF64)
	} else {
		tmpXmm = m.c.AllocateVReg(ssa.TypeF32)
	}

	// Check if the most significant bit (sign bit) is set.
	test := m.allocateInstr()
	test.asCmpRmiR(false, rn, rn.reg(), src64)
	m.insert(test)

	// Jump if the sign bit is set.
	ifSignTarget, ifSign := m.allocateBrTarget()
	jmpIfNeg := m.allocateInstr()
	jmpIfNeg.asJmpIf(condS, newOperandLabel(ifSign))
	m.insert(jmpIfNeg)

	// If the sign bit is not set, we could fit the unsigned int into float32/float64.
	// So, we convert it to float and emit jump instruction to exit from this branch.
	cvt := m.allocateInstr()
	cvt.asGprToXmm(op, rn, tmpXmm, src64)
	m.insert(cvt)

	// We are done, jump to end.
	jmpEnd := m.allocateInstr()
	jmpEnd.asJmp(newOperandLabel(end))
	m.insert(jmpEnd)

	// Now handling the case where sign-bit is set.
	// We emit the following sequences:
	// 	   mov      %rn, %tmp
	// 	   shr      1, %tmp
	// 	   mov      %rn, %tmp2
	// 	   and      1, %tmp2
	// 	   or       %tmp2, %tmp
	// 	   cvtsi2ss %tmp, %xmm0
	// 	   addsd    %xmm0, %xmm0
	m.insert(ifSignTarget)

	tmp := m.copyToTmp(rn.reg())
	shr := m.allocateInstr()
	shr.asShiftR(shiftROpShiftRightLogical, newOperandImm32(1), tmp, src64)
	m.insert(shr)

	tmp2 := m.copyToTmp(rn.reg())
	and := m.allocateInstr()
	and.asAluRmiR(aluRmiROpcodeAnd, newOperandImm32(1), tmp2, src64)
	m.insert(and)

	or := m.allocateInstr()
	or.asAluRmiR(aluRmiROpcodeOr, newOperandReg(tmp2), tmp, src64)
	m.insert(or)

	cvt2 := m.allocateInstr()
	cvt2.asGprToXmm(op, newOperandReg(tmp), tmpXmm, src64)
	m.insert(cvt2)

	addsd := m.allocateInstr()
	if dst64 {
		addsd.asXmmRmR(sseOpcodeAddsd, newOperandReg(tmpXmm), tmpXmm)
	} else {
		addsd.asXmmRmR(sseOpcodeAddss, newOperandReg(tmpXmm), tmpXmm)
	}
	m.insert(addsd)

	m.insert(endTarget)
	m.copyTo(tmpXmm, rd.reg())
}

func (m *machine) lowerVanyTrue(instr *ssa.Instruction) {
	x := instr.Arg()
	rm := m.getOperand_Reg(m.c.ValueDefinition(x))
	rd := m.c.VRegOf(instr.Return())

	tmp := m.c.AllocateVReg(ssa.TypeI32)

	cmp := m.allocateInstr()
	cmp.asXmmCmpRmR(sseOpcodePtest, rm, rm.reg())
	m.insert(cmp)

	setcc := m.allocateInstr()
	setcc.asSetcc(condNZ, tmp)
	m.insert(setcc)

	// Clear the irrelevant bits.
	and := m.allocateInstr()
	and.asAluRmiR(aluRmiROpcodeAnd, newOperandImm32(1), tmp, false)
	m.insert(and)

	m.copyTo(tmp, rd)
}

func (m *machine) lowerVallTrue(instr *ssa.Instruction) {
	x, lane := instr.ArgWithLane()
	var op sseOpcode
	switch lane {
	case ssa.VecLaneI8x16:
		op = sseOpcodePcmpeqb
	case ssa.VecLaneI16x8:
		op = sseOpcodePcmpeqw
	case ssa.VecLaneI32x4:
		op = sseOpcodePcmpeqd
	case ssa.VecLaneI64x2:
		op = sseOpcodePcmpeqq
	}
	rm := m.getOperand_Reg(m.c.ValueDefinition(x))
	rd := m.c.VRegOf(instr.Return())

	tmp := m.c.AllocateVReg(ssa.TypeV128)

	zeros := m.allocateInstr()
	zeros.asZeros(tmp)
	m.insert(zeros)

	pcmp := m.allocateInstr()
	pcmp.asXmmRmR(op, rm, tmp)
	m.insert(pcmp)

	test := m.allocateInstr()
	test.asXmmCmpRmR(sseOpcodePtest, newOperandReg(tmp), tmp)
	m.insert(test)

	tmp2 := m.c.AllocateVReg(ssa.TypeI32)

	setcc := m.allocateInstr()
	setcc.asSetcc(condZ, tmp2)
	m.insert(setcc)

	// Clear the irrelevant bits.
	and := m.allocateInstr()
	and.asAluRmiR(aluRmiROpcodeAnd, newOperandImm32(1), tmp2, false)
	m.insert(and)

	m.copyTo(tmp2, rd)
}

func (m *machine) lowerVhighBits(instr *ssa.Instruction) {
	x, lane := instr.ArgWithLane()
	rm := m.getOperand_Reg(m.c.ValueDefinition(x))
	rd := m.c.VRegOf(instr.Return())
	switch lane {
	case ssa.VecLaneI8x16:
		mov := m.allocateInstr()
		mov.asXmmToGpr(sseOpcodePmovmskb, rm.reg(), rd, false)
		m.insert(mov)

	case ssa.VecLaneI16x8:
		// When we have:
		// 	R1 = [R1(w1), R1(w2), R1(w3), R1(w4), R1(w5), R1(w6), R1(w7), R1(v8)]
		// 	R2 = [R2(w1), R2(w2), R2(w3), R2(v4), R2(w5), R2(w6), R2(w7), R2(v8)]
		//	where RX(wn) is n-th signed word (16-bit) of RX register,
		//
		// "PACKSSWB R1, R2" produces
		//  R1 = [
		// 		byte_sat(R1(w1)), byte_sat(R1(w2)), byte_sat(R1(w3)), byte_sat(R1(w4)),
		// 		byte_sat(R1(w5)), byte_sat(R1(w6)), byte_sat(R1(w7)), byte_sat(R1(w8)),
		// 		byte_sat(R2(w1)), byte_sat(R2(w2)), byte_sat(R2(w3)), byte_sat(R2(w4)),
		// 		byte_sat(R2(w5)), byte_sat(R2(w6)), byte_sat(R2(w7)), byte_sat(R2(w8)),
		//  ]
		//  where R1 is the destination register, and
		// 	byte_sat(w) = int8(w) if w fits as signed 8-bit,
		//                0x80 if w is less than 0x80
		//                0x7F if w is greater than 0x7f
		//
		// See https://www.felixcloutier.com/x86/packsswb:packssdw for detail.
		//
		// Therefore, v.register ends up having i-th and (i+8)-th bit set if i-th lane is negative (for i in 0..8).
		tmp := m.copyToTmp(rm.reg())
		res := m.c.AllocateVReg(ssa.TypeI32)

		pak := m.allocateInstr()
		pak.asXmmRmR(sseOpcodePacksswb, rm, tmp)
		m.insert(pak)

		mov := m.allocateInstr()
		mov.asXmmToGpr(sseOpcodePmovmskb, tmp, res, false)
		m.insert(mov)

		// Clear the higher bits than 8.
		shr := m.allocateInstr()
		shr.asShiftR(shiftROpShiftRightLogical, newOperandImm32(8), res, false)
		m.insert(shr)

		m.copyTo(res, rd)

	case ssa.VecLaneI32x4:
		mov := m.allocateInstr()
		mov.asXmmToGpr(sseOpcodeMovmskps, rm.reg(), rd, true)
		m.insert(mov)

	case ssa.VecLaneI64x2:
		mov := m.allocateInstr()
		mov.asXmmToGpr(sseOpcodeMovmskpd, rm.reg(), rd, true)
		m.insert(mov)
	}
}

func (m *machine) lowerVbnot(instr *ssa.Instruction) {
	x := instr.Arg()
	xDef := m.c.ValueDefinition(x)
	rm := m.getOperand_Reg(xDef)
	rd := m.c.VRegOf(instr.Return())

	tmp := m.copyToTmp(rm.reg())
	tmp2 := m.c.AllocateVReg(ssa.TypeV128)

	// Ensure tmp2 is considered defined by regalloc.
	m.insert(m.allocateInstr().asDefineUninitializedReg(tmp2))

	// Set all bits on tmp register.
	pak := m.allocateInstr()
	pak.asXmmRmR(sseOpcodePcmpeqd, newOperandReg(tmp2), tmp2)
	m.insert(pak)

	// Then XOR with tmp to reverse all bits on v.register.
	xor := m.allocateInstr()
	xor.asXmmRmR(sseOpcodePxor, newOperandReg(tmp2), tmp)
	m.insert(xor)

	m.copyTo(tmp, rd)
}

func (m *machine) lowerSplat(x, ret ssa.Value, lane ssa.VecLane) {
	tmpDst := m.c.AllocateVReg(ssa.TypeV128)
	m.insert(m.allocateInstr().asDefineUninitializedReg(tmpDst))

	switch lane {
	case ssa.VecLaneI8x16:
		tmp := m.c.AllocateVReg(ssa.TypeV128)
		m.insert(m.allocateInstr().asDefineUninitializedReg(tmp))
		xx := m.getOperand_Mem_Reg(m.c.ValueDefinition(x))
		m.insert(m.allocateInstr().asXmmRmRImm(sseOpcodePinsrb, 0, xx, tmpDst))
		m.insert(m.allocateInstr().asXmmRmR(sseOpcodePxor, newOperandReg(tmp), tmp))
		m.insert(m.allocateInstr().asXmmRmR(sseOpcodePshufb, newOperandReg(tmp), tmpDst))
	case ssa.VecLaneI16x8:
		xx := m.getOperand_Reg(m.c.ValueDefinition(x))
		m.insert(m.allocateInstr().asXmmRmRImm(sseOpcodePinsrw, 0, xx, tmpDst))
		m.insert(m.allocateInstr().asXmmRmRImm(sseOpcodePinsrw, 1, xx, tmpDst))
		m.insert(m.allocateInstr().asXmmRmRImm(sseOpcodePshufd, 0, newOperandReg(tmpDst), tmpDst))
	case ssa.VecLaneI32x4:
		xx := m.getOperand_Mem_Reg(m.c.ValueDefinition(x))
		m.insert(m.allocateInstr().asXmmRmRImm(sseOpcodePinsrd, 0, xx, tmpDst))
		m.insert(m.allocateInstr().asXmmRmRImm(sseOpcodePshufd, 0, newOperandReg(tmpDst), tmpDst))
	case ssa.VecLaneI64x2:
		xx := m.getOperand_Reg(m.c.ValueDefinition(x))
		m.insert(m.allocateInstr().asXmmRmRImm(sseOpcodePinsrq, 0, xx, tmpDst))
		m.insert(m.allocateInstr().asXmmRmRImm(sseOpcodePinsrq, 1, xx, tmpDst))
	case ssa.VecLaneF32x4:
		xx := m.getOperand_Mem_Reg(m.c.ValueDefinition(x))
		m.insert(m.allocateInstr().asXmmRmRImm(sseOpcodeInsertps, 0, xx, tmpDst))
		m.insert(m.allocateInstr().asXmmRmRImm(sseOpcodePshufd, 0, newOperandReg(tmpDst), tmpDst))
	case ssa.VecLaneF64x2:
		xx := m.getOperand_Reg(m.c.ValueDefinition(x))
		m.insert(m.allocateInstr().asXmmUnaryRmR(sseOpcodeMovsd, xx, tmpDst))
		m.insert(m.allocateInstr().asXmmRmR(sseOpcodeMovlhps, xx, tmpDst))
	default:
		panic(fmt.Sprintf("invalid lane type: %s", lane))
	}

	m.copyTo(tmpDst, m.c.VRegOf(ret))
}

func (m *machine) lowerShuffle(x, y ssa.Value, lo, hi uint64, ret ssa.Value) {
	var xMask, yMask [2]uint64
	for i := 0; i < 8; i++ {
		loLane := byte(lo >> (i * 8))
		if loLane < 16 {
			xMask[0] |= uint64(loLane) << (i * 8)
			yMask[0] |= uint64(0x80) << (i * 8)
		} else {
			xMask[0] |= uint64(0x80) << (i * 8)
			yMask[0] |= uint64(loLane-16) << (i * 8)
		}
		hiLane := byte(hi >> (i * 8))
		if hiLane < 16 {
			xMask[1] |= uint64(hiLane) << (i * 8)
			yMask[1] |= uint64(0x80) << (i * 8)
		} else {
			xMask[1] |= uint64(0x80) << (i * 8)
			yMask[1] |= uint64(hiLane-16) << (i * 8)
		}
	}

	xl, xmaskPos := m.allocateLabel()
	m.consts = append(m.consts, _const{lo: xMask[0], hi: xMask[1], label: xl, labelPos: xmaskPos})
	yl, ymaskPos := m.allocateLabel()
	m.consts = append(m.consts, _const{lo: yMask[0], hi: yMask[1], label: yl, labelPos: ymaskPos})

	xx, yy := m.getOperand_Reg(m.c.ValueDefinition(x)), m.getOperand_Reg(m.c.ValueDefinition(y))
	tmpX, tmpY := m.copyToTmp(xx.reg()), m.copyToTmp(yy.reg())

	// Apply mask to X.
	tmp := m.c.AllocateVReg(ssa.TypeV128)
	loadMaskLo := m.allocateInstr().asXmmUnaryRmR(sseOpcodeMovdqu, newOperandMem(m.newAmodeRipRel(xl)), tmp)
	m.insert(loadMaskLo)
	m.insert(m.allocateInstr().asXmmRmR(sseOpcodePshufb, newOperandReg(tmp), tmpX))

	// Apply mask to Y.
	loadMaskHi := m.allocateInstr().asXmmUnaryRmR(sseOpcodeMovdqu, newOperandMem(m.newAmodeRipRel(yl)), tmp)
	m.insert(loadMaskHi)
	m.insert(m.allocateInstr().asXmmRmR(sseOpcodePshufb, newOperandReg(tmp), tmpY))

	// Combine the results.
	m.insert(m.allocateInstr().asXmmRmR(sseOpcodeOrps, newOperandReg(tmpX), tmpY))

	m.copyTo(tmpY, m.c.VRegOf(ret))
}

func (m *machine) lowerVbBinOpUnaligned(op sseOpcode, x, y, ret ssa.Value) {
	rn := m.getOperand_Reg(m.c.ValueDefinition(x))
	rm := m.getOperand_Reg(m.c.ValueDefinition(y))
	rd := m.c.VRegOf(ret)

	tmp := m.copyToTmp(rn.reg())

	binOp := m.allocateInstr()
	binOp.asXmmRmR(op, rm, tmp)
	m.insert(binOp)

	m.copyTo(tmp, rd)
}

func (m *machine) lowerVbBinOp(op sseOpcode, x, y, ret ssa.Value) {
	rn := m.getOperand_Reg(m.c.ValueDefinition(x))
	rm := m.getOperand_Mem_Reg(m.c.ValueDefinition(y))
	rd := m.c.VRegOf(ret)

	tmp := m.copyToTmp(rn.reg())

	binOp := m.allocateInstr()
	binOp.asXmmRmR(op, rm, tmp)
	m.insert(binOp)

	m.copyTo(tmp, rd)
}

func (m *machine) lowerVFcmp(x, y ssa.Value, c ssa.FloatCmpCond, ret ssa.Value, lane ssa.VecLane) {
	var cmpOp sseOpcode
	switch lane {
	case ssa.VecLaneF32x4:
		cmpOp = sseOpcodeCmpps
	case ssa.VecLaneF64x2:
		cmpOp = sseOpcodeCmppd
	default:
		panic(fmt.Sprintf("invalid lane type: %s", lane))
	}

	xx, yy := m.c.ValueDefinition(x), m.c.ValueDefinition(y)
	var cmpImm cmpPred
	switch c {
	case ssa.FloatCmpCondGreaterThan:
		yy, xx = xx, yy
		cmpImm = cmpPredLT_OS
	case ssa.FloatCmpCondGreaterThanOrEqual:
		yy, xx = xx, yy
		cmpImm = cmpPredLE_OS
	case ssa.FloatCmpCondEqual:
		cmpImm = cmpPredEQ_OQ
	case ssa.FloatCmpCondNotEqual:
		cmpImm = cmpPredNEQ_UQ
	case ssa.FloatCmpCondLessThan:
		cmpImm = cmpPredLT_OS
	case ssa.FloatCmpCondLessThanOrEqual:
		cmpImm = cmpPredLE_OS
	default:
		panic(fmt.Sprintf("invalid float comparison condition: %s", c))
	}

	tmp := m.c.AllocateVReg(ssa.TypeV128)
	xxx := m.getOperand_Mem_Reg(xx)
	m.insert(m.allocateInstr().asXmmUnaryRmR(sseOpcodeMovdqu, xxx, tmp))

	rm := m.getOperand_Mem_Reg(yy)
	m.insert(m.allocateInstr().asXmmRmRImm(cmpOp, byte(cmpImm), rm, tmp))

	m.copyTo(tmp, m.c.VRegOf(ret))
}

func (m *machine) lowerVIcmp(x, y ssa.Value, c ssa.IntegerCmpCond, ret ssa.Value, lane ssa.VecLane) {
	var eq, gt, maxu, minu, mins sseOpcode
	switch lane {
	case ssa.VecLaneI8x16:
		eq, gt, maxu, minu, mins = sseOpcodePcmpeqb, sseOpcodePcmpgtb, sseOpcodePmaxub, sseOpcodePminub, sseOpcodePminsb
	case ssa.VecLaneI16x8:
		eq, gt, maxu, minu, mins = sseOpcodePcmpeqw, sseOpcodePcmpgtw, sseOpcodePmaxuw, sseOpcodePminuw, sseOpcodePminsw
	case ssa.VecLaneI32x4:
		eq, gt, maxu, minu, mins = sseOpcodePcmpeqd, sseOpcodePcmpgtd, sseOpcodePmaxud, sseOpcodePminud, sseOpcodePminsd
	case ssa.VecLaneI64x2:
		eq, gt = sseOpcodePcmpeqq, sseOpcodePcmpgtq
	default:
		panic(fmt.Sprintf("invalid lane type: %s", lane))
	}

	tmp := m.c.AllocateVReg(ssa.TypeV128)
	var op operand
	switch c {
	case ssa.IntegerCmpCondSignedLessThanOrEqual:
		if lane == ssa.VecLaneI64x2 {
			x := m.getOperand_Mem_Reg(m.c.ValueDefinition(x))
			// Copy x to tmp.
			m.insert(m.allocateInstr().asXmmUnaryRmR(sseOpcodeMovdqu, x, tmp))
			op = m.getOperand_Mem_Reg(m.c.ValueDefinition(y))
		} else {
			y := m.getOperand_Mem_Reg(m.c.ValueDefinition(y))
			// Copy y to tmp.
			m.insert(m.allocateInstr().asXmmUnaryRmR(sseOpcodeMovdqu, y, tmp))
			op = m.getOperand_Mem_Reg(m.c.ValueDefinition(x))
		}
	case ssa.IntegerCmpCondSignedGreaterThanOrEqual:
		if lane == ssa.VecLaneI64x2 {
			y := m.getOperand_Mem_Reg(m.c.ValueDefinition(y))
			// Copy y to tmp.
			m.insert(m.allocateInstr().asXmmUnaryRmR(sseOpcodeMovdqu, y, tmp))
			op = m.getOperand_Mem_Reg(m.c.ValueDefinition(x))
		} else {
			x := m.getOperand_Mem_Reg(m.c.ValueDefinition(x))
			// Copy x to tmp.
			m.insert(m.allocateInstr().asXmmUnaryRmR(sseOpcodeMovdqu, x, tmp))
			op = m.getOperand_Mem_Reg(m.c.ValueDefinition(y))
		}
	case ssa.IntegerCmpCondSignedLessThan, ssa.IntegerCmpCondUnsignedLessThan, ssa.IntegerCmpCondUnsignedLessThanOrEqual:
		y := m.getOperand_Mem_Reg(m.c.ValueDefinition(y))
		// Copy y to tmp.
		m.insert(m.allocateInstr().asXmmUnaryRmR(sseOpcodeMovdqu, y, tmp))
		op = m.getOperand_Mem_Reg(m.c.ValueDefinition(x))
	default:
		x := m.getOperand_Mem_Reg(m.c.ValueDefinition(x))
		// Copy x to tmp.
		m.insert(m.allocateInstr().asXmmUnaryRmR(sseOpcodeMovdqu, x, tmp))
		op = m.getOperand_Mem_Reg(m.c.ValueDefinition(y))
	}

	switch c {
	case ssa.IntegerCmpCondEqual:
		m.insert(m.allocateInstr().asXmmRmR(eq, op, tmp))
	case ssa.IntegerCmpCondNotEqual:
		// First we compare for equality.
		m.insert(m.allocateInstr().asXmmRmR(eq, op, tmp))
		// Then flip the bits. To do so, we set all bits on tmp2.
		tmp2 := m.c.AllocateVReg(ssa.TypeV128)
		m.insert(m.allocateInstr().asDefineUninitializedReg(tmp2))
		m.insert(m.allocateInstr().asXmmRmR(eq, newOperandReg(tmp2), tmp2))
		// And then xor with tmp.
		m.insert(m.allocateInstr().asXmmRmR(sseOpcodePxor, newOperandReg(tmp2), tmp))
	case ssa.IntegerCmpCondSignedGreaterThan, ssa.IntegerCmpCondSignedLessThan:
		m.insert(m.allocateInstr().asXmmRmR(gt, op, tmp))
	case ssa.IntegerCmpCondSignedGreaterThanOrEqual, ssa.IntegerCmpCondSignedLessThanOrEqual:
		if lane == ssa.VecLaneI64x2 {
			m.insert(m.allocateInstr().asXmmRmR(gt, op, tmp))
			// Then flip the bits. To do so, we set all bits on tmp2.
			tmp2 := m.c.AllocateVReg(ssa.TypeV128)
			m.insert(m.allocateInstr().asDefineUninitializedReg(tmp2))
			m.insert(m.allocateInstr().asXmmRmR(eq, newOperandReg(tmp2), tmp2))
			// And then xor with tmp.
			m.insert(m.allocateInstr().asXmmRmR(sseOpcodePxor, newOperandReg(tmp2), tmp))
		} else {
			// First take min of x and y.
			m.insert(m.allocateInstr().asXmmRmR(mins, op, tmp))
			// Then compare for equality.
			m.insert(m.allocateInstr().asXmmRmR(eq, op, tmp))
		}
	case ssa.IntegerCmpCondUnsignedGreaterThan, ssa.IntegerCmpCondUnsignedLessThan:
		// First maxu of x and y.
		m.insert(m.allocateInstr().asXmmRmR(maxu, op, tmp))
		// Then compare for equality.
		m.insert(m.allocateInstr().asXmmRmR(eq, op, tmp))
		// Then flip the bits. To do so, we set all bits on tmp2.
		tmp2 := m.c.AllocateVReg(ssa.TypeV128)
		m.insert(m.allocateInstr().asDefineUninitializedReg(tmp2))
		m.insert(m.allocateInstr().asXmmRmR(eq, newOperandReg(tmp2), tmp2))
		// And then xor with tmp.
		m.insert(m.allocateInstr().asXmmRmR(sseOpcodePxor, newOperandReg(tmp2), tmp))
	case ssa.IntegerCmpCondUnsignedGreaterThanOrEqual, ssa.IntegerCmpCondUnsignedLessThanOrEqual:
		m.insert(m.allocateInstr().asXmmRmR(minu, op, tmp))
		m.insert(m.allocateInstr().asXmmRmR(eq, op, tmp))
	default:
		panic("BUG")
	}

	m.copyTo(tmp, m.c.VRegOf(ret))
}

func (m *machine) lowerVbandnot(instr *ssa.Instruction, op sseOpcode) {
	x, y := instr.Arg2()
	xDef := m.c.ValueDefinition(x)
	yDef := m.c.ValueDefinition(y)
	rm, rn := m.getOperand_Reg(xDef), m.getOperand_Reg(yDef)
	rd := m.c.VRegOf(instr.Return())

	tmp := m.copyToTmp(rn.reg())

	// pandn between rn, rm.
	pand := m.allocateInstr()
	pand.asXmmRmR(sseOpcodePandn, rm, tmp)
	m.insert(pand)

	m.copyTo(tmp, rd)
}

func (m *machine) lowerVbitselect(instr *ssa.Instruction) {
	c, x, y := instr.SelectData()
	xDef := m.c.ValueDefinition(x)
	yDef := m.c.ValueDefinition(y)
	rm, rn := m.getOperand_Reg(xDef), m.getOperand_Reg(yDef)
	creg := m.getOperand_Reg(m.c.ValueDefinition(c))
	rd := m.c.VRegOf(instr.Return())

	tmpC := m.copyToTmp(creg.reg())
	tmpX := m.copyToTmp(rm.reg())

	// And between c, x (overwrites x).
	pand := m.allocateInstr()
	pand.asXmmRmR(sseOpcodePand, creg, tmpX)
	m.insert(pand)

	// Andn between y, c (overwrites c).
	pandn := m.allocateInstr()
	pandn.asXmmRmR(sseOpcodePandn, rn, tmpC)
	m.insert(pandn)

	por := m.allocateInstr()
	por.asXmmRmR(sseOpcodePor, newOperandReg(tmpC), tmpX)
	m.insert(por)

	m.copyTo(tmpX, rd)
}

func (m *machine) lowerVFmin(instr *ssa.Instruction) {
	x, y, lane := instr.Arg2WithLane()
	rn := m.getOperand_Reg(m.c.ValueDefinition(x))
	rm := m.getOperand_Reg(m.c.ValueDefinition(y))
	rd := m.c.VRegOf(instr.Return())

	var min, cmp, andn, or, srl /* shift right logical */ sseOpcode
	var shiftNumToInverseNaN uint32
	if lane == ssa.VecLaneF32x4 {
		min, cmp, andn, or, srl, shiftNumToInverseNaN = sseOpcodeMinps, sseOpcodeCmpps, sseOpcodeAndnps, sseOpcodeOrps, sseOpcodePsrld, 0xa
	} else {
		min, cmp, andn, or, srl, shiftNumToInverseNaN = sseOpcodeMinpd, sseOpcodeCmppd, sseOpcodeAndnpd, sseOpcodeOrpd, sseOpcodePsrlq, 0xd
	}

	tmp1 := m.copyToTmp(rn.reg())
	tmp2 := m.copyToTmp(rm.reg())

	// tmp1=min(rn, rm)
	minIns1 := m.allocateInstr()
	minIns1.asXmmRmR(min, rn, tmp2)
	m.insert(minIns1)

	// tmp2=min(rm, rn)
	minIns2 := m.allocateInstr()
	minIns2.asXmmRmR(min, rm, tmp1)
	m.insert(minIns2)

	// tmp3:=tmp1=min(rn, rm)
	tmp3 := m.copyToTmp(tmp1)

	// tmp1 = -0         if (rn == -0 || rm == -0) && rn != NaN && rm !=NaN
	//       NaN         if rn == NaN || rm == NaN
	//       min(rm, rm) otherwise
	orIns := m.allocateInstr()
	orIns.asXmmRmR(or, newOperandReg(tmp2), tmp1)
	m.insert(orIns)

	// tmp3 is originally min(rn,rm).
	// tmp3 = 0^ (set all bits) if rn == NaN || rm == NaN
	//        0 otherwise
	cmpIns := m.allocateInstr()
	cmpIns.asXmmRmRImm(cmp, uint8(cmpPredUNORD_Q), newOperandReg(tmp2), tmp3)
	m.insert(cmpIns)

	// tmp1 = -0          if (rn == -0 || rm == -0) && rn != NaN && rm !=NaN
	//        ^0          if rn == NaN || rm == NaN
	//        min(v1, v2) otherwise
	orIns2 := m.allocateInstr()
	orIns2.asXmmRmR(or, newOperandReg(tmp3), tmp1)
	m.insert(orIns2)

	// tmp3 = set all bits on the mantissa bits
	//        0 otherwise
	shift := m.allocateInstr()
	shift.asXmmRmiReg(srl, newOperandImm32(shiftNumToInverseNaN), tmp3)
	m.insert(shift)

	// tmp3 = tmp1 and !tmp3
	//     = -0                                                   if (rn == -0 || rm == -0) && rn != NaN && rm !=NaN
	//       set all bits on exponential and sign bit (== NaN)    if rn == NaN || rm == NaN
	//       min(rn, rm)                                          otherwise
	andnIns := m.allocateInstr()
	andnIns.asXmmRmR(andn, newOperandReg(tmp1), tmp3)
	m.insert(andnIns)

	m.copyTo(tmp3, rd)
}

func (m *machine) lowerVFmax(instr *ssa.Instruction) {
	x, y, lane := instr.Arg2WithLane()
	rn := m.getOperand_Reg(m.c.ValueDefinition(x))
	rm := m.getOperand_Reg(m.c.ValueDefinition(y))
	rd := m.c.VRegOf(instr.Return())

	var max, cmp, andn, or, xor, sub, srl /* shift right logical */ sseOpcode
	var shiftNumToInverseNaN uint32
	if lane == ssa.VecLaneF32x4 {
		max, cmp, andn, or, xor, sub, srl, shiftNumToInverseNaN = sseOpcodeMaxps, sseOpcodeCmpps, sseOpcodeAndnps, sseOpcodeOrps, sseOpcodeXorps, sseOpcodeSubps, sseOpcodePsrld, 0xa
	} else {
		max, cmp, andn, or, xor, sub, srl, shiftNumToInverseNaN = sseOpcodeMaxpd, sseOpcodeCmppd, sseOpcodeAndnpd, sseOpcodeOrpd, sseOpcodeXorpd, sseOpcodeSubpd, sseOpcodePsrlq, 0xd
	}

	tmp0 := m.copyToTmp(rm.reg())
	tmp1 := m.copyToTmp(rn.reg())

	// tmp0=max(rn, rm)
	maxIns1 := m.allocateInstr()
	maxIns1.asXmmRmR(max, rn, tmp0)
	m.insert(maxIns1)

	// tmp1=max(rm, rn)
	maxIns2 := m.allocateInstr()
	maxIns2.asXmmRmR(max, rm, tmp1)
	m.insert(maxIns2)

	// tmp2=max(rm, rn)
	tmp2 := m.copyToTmp(tmp1)

	// tmp2 = -0       if (rn == -0 && rm == 0) || (rn == 0 && rm == -0)
	//         0       if (rn == 0 && rm ==  0)
	//        -0       if (rn == -0 && rm == -0)
	//       v1^v2     if rn == NaN || rm == NaN
	//         0       otherwise
	xorInstr := m.allocateInstr()
	xorInstr.asXmmRmR(xor, newOperandReg(tmp0), tmp2)
	m.insert(xorInstr)
	// tmp1 = -0           if (rn == -0 && rm == 0) || (rn == 0 && rm == -0)
	//         0           if (rn == 0 && rm ==  0)
	//        -0           if (rn == -0 && rm == -0)
	//        NaN          if rn == NaN || rm == NaN
	//        max(v1, v2)  otherwise
	orInstr := m.allocateInstr()
	orInstr.asXmmRmR(or, newOperandReg(tmp2), tmp1)
	m.insert(orInstr)

	tmp3 := m.copyToTmp(tmp1)

	// tmp3 = 0           if (rn == -0 && rm == 0) || (rn == 0 && rm == -0) || (rn == 0 && rm ==  0)
	//       -0           if (rn == -0 && rm == -0)
	//       NaN          if rn == NaN || rm == NaN
	//       max(v1, v2)  otherwise
	//
	// Note: -0 - (-0) = 0 (!= -0) in floating point operation.
	subIns := m.allocateInstr()
	subIns.asXmmRmR(sub, newOperandReg(tmp2), tmp3)
	m.insert(subIns)

	// tmp1 = 0^ if rn == NaN || rm == NaN
	cmpIns := m.allocateInstr()
	cmpIns.asXmmRmRImm(cmp, uint8(cmpPredUNORD_Q), newOperandReg(tmp1), tmp1)
	m.insert(cmpIns)

	// tmp1 = set all bits on the mantissa bits
	//        0 otherwise
	shift := m.allocateInstr()
	shift.asXmmRmiReg(srl, newOperandImm32(shiftNumToInverseNaN), tmp1)
	m.insert(shift)

	andnIns := m.allocateInstr()
	andnIns.asXmmRmR(andn, newOperandReg(tmp3), tmp1)
	m.insert(andnIns)

	m.copyTo(tmp1, rd)
}

func (m *machine) lowerVFabs(instr *ssa.Instruction) {
	x, lane := instr.ArgWithLane()
	rm := m.getOperand_Mem_Reg(m.c.ValueDefinition(x))
	rd := m.c.VRegOf(instr.Return())

	tmp := m.c.AllocateVReg(ssa.TypeV128)

	def := m.allocateInstr()
	def.asDefineUninitializedReg(tmp)
	m.insert(def)

	// Set all bits on tmp.
	pcmp := m.allocateInstr()
	pcmp.asXmmRmR(sseOpcodePcmpeqd, newOperandReg(tmp), tmp)
	m.insert(pcmp)

	switch lane {
	case ssa.VecLaneF32x4:
		// Shift right packed single floats by 1 to clear the sign bits.
		shift := m.allocateInstr()
		shift.asXmmRmiReg(sseOpcodePsrld, newOperandImm32(1), tmp)
		m.insert(shift)
		// Clear the sign bit of rm.
		andp := m.allocateInstr()
		andp.asXmmRmR(sseOpcodeAndpd, rm, tmp)
		m.insert(andp)
	case ssa.VecLaneF64x2:
		// Shift right packed single floats by 1 to clear the sign bits.
		shift := m.allocateInstr()
		shift.asXmmRmiReg(sseOpcodePsrlq, newOperandImm32(1), tmp)
		m.insert(shift)
		// Clear the sign bit of rm.
		andp := m.allocateInstr()
		andp.asXmmRmR(sseOpcodeAndps, rm, tmp)
		m.insert(andp)
	}

	m.copyTo(tmp, rd)
}
