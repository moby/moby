package arm64

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/tetratelabs/wazero/internal/engine/wazevo/backend"
	"github.com/tetratelabs/wazero/internal/engine/wazevo/backend/regalloc"
	"github.com/tetratelabs/wazero/internal/engine/wazevo/ssa"
	"github.com/tetratelabs/wazero/internal/engine/wazevo/wazevoapi"
)

type (
	// machine implements backend.Machine.
	machine struct {
		compiler   backend.Compiler
		currentABI *backend.FunctionABI
		instrPool  wazevoapi.Pool[instruction]
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

		regAlloc   regalloc.Allocator[*instruction, *labelPosition, *regAllocFn]
		regAllocFn regAllocFn

		amodePool wazevoapi.Pool[addressMode]

		// addendsWorkQueue is used during address lowering, defined here for reuse.
		addendsWorkQueue wazevoapi.Queue[ssa.Value]
		addends32        wazevoapi.Queue[addend32]
		// addends64 is used during address lowering, defined here for reuse.
		addends64              wazevoapi.Queue[regalloc.VReg]
		unresolvedAddressModes []*instruction

		// condBrRelocs holds the conditional branches which need offset relocation.
		condBrRelocs []condBrReloc

		// jmpTableTargets holds the labels of the jump table targets.
		jmpTableTargets [][]uint32
		// jmpTableTargetNext is the index to the jmpTableTargets slice to be used for the next jump table.
		jmpTableTargetsNext int

		// spillSlotSize is the size of the stack slot in bytes used for spilling registers.
		// During the execution of the function, the stack looks like:
		//
		//
		//            (high address)
		//          +-----------------+
		//          |     .......     |
		//          |      ret Y      |
		//          |     .......     |
		//          |      ret 0      |
		//          |      arg X      |
		//          |     .......     |
		//          |      arg 1      |
		//          |      arg 0      |
		//          |      xxxxx      |
		//          |   ReturnAddress |
		//          +-----------------+   <<-|
		//          |   ...........   |      |
		//          |   spill slot M  |      | <--- spillSlotSize
		//          |   ............  |      |
		//          |   spill slot 2  |      |
		//          |   spill slot 1  |   <<-+
		//          |   clobbered N   |
		//          |   ...........   |
		//          |   clobbered 1   |
		//          |   clobbered 0   |
		//   SP---> +-----------------+
		//             (low address)
		//
		// and it represents the size of the space between FP and the first spilled slot. This must be a multiple of 16.
		// Also note that this is only known after register allocation.
		spillSlotSize int64
		spillSlots    map[regalloc.VRegID]int64 // regalloc.VRegID to offset.
		// clobberedRegs holds real-register backed VRegs saved at the function prologue, and restored at the epilogue.
		clobberedRegs []regalloc.VReg

		maxRequiredStackSizeForCalls int64
		stackBoundsCheckDisabled     bool

		regAllocStarted bool
	}

	addend32 struct {
		r   regalloc.VReg
		ext extendOp
	}

	condBrReloc struct {
		cbr *instruction
		// currentLabelPos is the labelPosition within which condBr is defined.
		currentLabelPos *labelPosition
		// Next block's labelPosition.
		nextLabel label
		offset    int64
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

const (
	labelReturn  label = math.MaxUint32
	labelInvalid       = labelReturn - 1
)

// String implements backend.Machine.
func (l label) String() string {
	return fmt.Sprintf("L%d", l)
}

func resetLabelPosition(l *labelPosition) {
	*l = labelPosition{}
}

// NewBackend returns a new backend for arm64.
func NewBackend() backend.Machine {
	m := &machine{
		spillSlots:        make(map[regalloc.VRegID]int64),
		regAlloc:          regalloc.NewAllocator[*instruction, *labelPosition, *regAllocFn](regInfo),
		amodePool:         wazevoapi.NewPool[addressMode](resetAddressMode),
		instrPool:         wazevoapi.NewPool[instruction](resetInstruction),
		labelPositionPool: wazevoapi.NewIDedPool[labelPosition](resetLabelPosition),
	}
	m.regAllocFn.m = m
	return m
}

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

// RegAlloc implements backend.Machine Function.
func (m *machine) RegAlloc() {
	m.regAllocStarted = true
	m.regAlloc.DoAllocation(&m.regAllocFn)
	// Now that we know the final spill slot size, we must align spillSlotSize to 16 bytes.
	m.spillSlotSize = (m.spillSlotSize + 15) &^ 15
}

// Reset implements backend.Machine.
func (m *machine) Reset() {
	m.clobberedRegs = m.clobberedRegs[:0]
	for key := range m.spillSlots {
		m.clobberedRegs = append(m.clobberedRegs, regalloc.VReg(key))
	}
	for _, key := range m.clobberedRegs {
		delete(m.spillSlots, regalloc.VRegID(key))
	}
	m.clobberedRegs = m.clobberedRegs[:0]
	m.regAllocStarted = false
	m.regAlloc.Reset()
	m.spillSlotSize = 0
	m.unresolvedAddressModes = m.unresolvedAddressModes[:0]
	m.maxRequiredStackSizeForCalls = 0
	m.jmpTableTargetsNext = 0
	m.amodePool.Reset()
	m.instrPool.Reset()
	m.labelPositionPool.Reset()
	m.pendingInstructions = m.pendingInstructions[:0]
	m.perBlockHead, m.perBlockEnd, m.rootInstr = nil, nil, nil
	m.orderedSSABlockLabelPos = m.orderedSSABlockLabelPos[:0]
}

// StartLoweringFunction implements backend.Machine StartLoweringFunction.
func (m *machine) StartLoweringFunction(maxBlockID ssa.BasicBlockID) {
	m.maxSSABlockID = label(maxBlockID)
	m.nextLabel = label(maxBlockID) + 1
}

// SetCurrentABI implements backend.Machine SetCurrentABI.
func (m *machine) SetCurrentABI(abi *backend.FunctionABI) {
	m.currentABI = abi
}

// DisableStackCheck implements backend.Machine DisableStackCheck.
func (m *machine) DisableStackCheck() {
	m.stackBoundsCheckDisabled = true
}

// SetCompiler implements backend.Machine.
func (m *machine) SetCompiler(ctx backend.Compiler) {
	m.compiler = ctx
	m.regAllocFn.ssaB = ctx.SSABuilder()
}

func (m *machine) insert(i *instruction) {
	m.pendingInstructions = append(m.pendingInstructions, i)
}

func (m *machine) insertBrTargetLabel() label {
	nop, l := m.allocateBrTarget()
	m.insert(nop)
	return l
}

func (m *machine) allocateBrTarget() (nop *instruction, l label) {
	l = m.nextLabel
	m.nextLabel++
	nop = m.allocateInstr()
	nop.asNop0WithLabel(l)
	pos := m.labelPositionPool.GetOrAllocate(int(l))
	pos.begin, pos.end = nop, nop
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

func resetInstruction(i *instruction) {
	*i = instruction{}
}

func (m *machine) allocateNop() *instruction {
	instr := m.allocateInstr()
	instr.asNop0()
	return instr
}

func (m *machine) resolveAddressingMode(arg0offset, ret0offset int64, i *instruction) {
	amode := i.getAmode()
	switch amode.kind {
	case addressModeKindResultStackSpace:
		amode.imm += ret0offset
	case addressModeKindArgStackSpace:
		amode.imm += arg0offset
	default:
		panic("BUG")
	}

	var sizeInBits byte
	switch i.kind {
	case store8, uLoad8:
		sizeInBits = 8
	case store16, uLoad16:
		sizeInBits = 16
	case store32, fpuStore32, uLoad32, fpuLoad32:
		sizeInBits = 32
	case store64, fpuStore64, uLoad64, fpuLoad64:
		sizeInBits = 64
	case fpuStore128, fpuLoad128:
		sizeInBits = 128
	default:
		panic("BUG")
	}

	if offsetFitsInAddressModeKindRegUnsignedImm12(sizeInBits, amode.imm) {
		amode.kind = addressModeKindRegUnsignedImm12
	} else {
		// This case, we load the offset into the temporary register,
		// and then use it as the index register.
		newPrev := m.lowerConstantI64AndInsert(i.prev, tmpRegVReg, amode.imm)
		linkInstr(newPrev, i)
		*amode = addressMode{kind: addressModeKindRegReg, rn: amode.rn, rm: tmpRegVReg, extOp: extendOpUXTX /* indicates rm reg is 64-bit */}
	}
}

// resolveRelativeAddresses resolves the relative addresses before encoding.
func (m *machine) resolveRelativeAddresses(ctx context.Context) {
	for {
		if len(m.unresolvedAddressModes) > 0 {
			arg0offset, ret0offset := m.arg0OffsetFromSP(), m.ret0OffsetFromSP()
			for _, i := range m.unresolvedAddressModes {
				m.resolveAddressingMode(arg0offset, ret0offset, i)
			}
		}

		// Reuse the slice to gather the unresolved conditional branches.
		m.condBrRelocs = m.condBrRelocs[:0]

		var fn string
		var fnIndex int
		var labelPosToLabel map[*labelPosition]label
		if wazevoapi.PerfMapEnabled {
			labelPosToLabel = make(map[*labelPosition]label)
			for i := 0; i <= m.labelPositionPool.MaxIDEncountered(); i++ {
				labelPosToLabel[m.labelPositionPool.Get(i)] = label(i)
			}

			fn = wazevoapi.GetCurrentFunctionName(ctx)
			fnIndex = wazevoapi.GetCurrentFunctionIndex(ctx)
		}

		// Next, in order to determine the offsets of relative jumps, we have to calculate the size of each label.
		var offset int64
		for i, pos := range m.orderedSSABlockLabelPos {
			pos.binaryOffset = offset
			var size int64
			for cur := pos.begin; ; cur = cur.next {
				switch cur.kind {
				case nop0:
					l := cur.nop0Label()
					if pos := m.labelPositionPool.Get(int(l)); pos != nil {
						pos.binaryOffset = offset + size
					}
				case condBr:
					if !cur.condBrOffsetResolved() {
						var nextLabel label
						if i < len(m.orderedSSABlockLabelPos)-1 {
							// Note: this is only used when the block ends with fallthrough,
							// therefore can be safely assumed that the next block exists when it's needed.
							nextLabel = ssaBlockLabel(m.orderedSSABlockLabelPos[i+1].sb)
						}
						m.condBrRelocs = append(m.condBrRelocs, condBrReloc{
							cbr: cur, currentLabelPos: pos, offset: offset + size,
							nextLabel: nextLabel,
						})
					}
				}
				size += cur.size()
				if cur == pos.end {
					break
				}
			}

			if wazevoapi.PerfMapEnabled {
				if size > 0 {
					wazevoapi.PerfMap.AddModuleEntry(fnIndex, offset, uint64(size), fmt.Sprintf("%s:::::%s", fn, labelPosToLabel[pos]))
				}
			}
			offset += size
		}

		// Before resolving any offsets, we need to check if all the conditional branches can be resolved.
		var needRerun bool
		for i := range m.condBrRelocs {
			reloc := &m.condBrRelocs[i]
			cbr := reloc.cbr
			offset := reloc.offset

			target := cbr.condBrLabel()
			offsetOfTarget := m.labelPositionPool.Get(int(target)).binaryOffset
			diff := offsetOfTarget - offset
			if divided := diff >> 2; divided < minSignedInt19 || divided > maxSignedInt19 {
				// This case the conditional branch is too huge. We place the trampoline instructions at the end of the current block,
				// and jump to it.
				m.insertConditionalJumpTrampoline(cbr, reloc.currentLabelPos, reloc.nextLabel)
				// Then, we need to recall this function to fix up the label offsets
				// as they have changed after the trampoline is inserted.
				needRerun = true
			}
		}
		if needRerun {
			if wazevoapi.PerfMapEnabled {
				wazevoapi.PerfMap.Clear()
			}
		} else {
			break
		}
	}

	var currentOffset int64
	for cur := m.rootInstr; cur != nil; cur = cur.next {
		switch cur.kind {
		case br:
			target := cur.brLabel()
			offsetOfTarget := m.labelPositionPool.Get(int(target)).binaryOffset
			diff := offsetOfTarget - currentOffset
			divided := diff >> 2
			if divided < minSignedInt26 || divided > maxSignedInt26 {
				// This means the currently compiled single function is extremely large.
				panic("too large function that requires branch relocation of large unconditional branch larger than 26-bit range")
			}
			cur.brOffsetResolve(diff)
		case condBr:
			if !cur.condBrOffsetResolved() {
				target := cur.condBrLabel()
				offsetOfTarget := m.labelPositionPool.Get(int(target)).binaryOffset
				diff := offsetOfTarget - currentOffset
				if divided := diff >> 2; divided < minSignedInt19 || divided > maxSignedInt19 {
					panic("BUG: branch relocation for large conditional branch larger than 19-bit range must be handled properly")
				}
				cur.condBrOffsetResolve(diff)
			}
		case brTableSequence:
			tableIndex := cur.u1
			targets := m.jmpTableTargets[tableIndex]
			for i := range targets {
				l := label(targets[i])
				offsetOfTarget := m.labelPositionPool.Get(int(l)).binaryOffset
				diff := offsetOfTarget - (currentOffset + brTableSequenceOffsetTableBegin)
				targets[i] = uint32(diff)
			}
			cur.brTableSequenceOffsetsResolved()
		case emitSourceOffsetInfo:
			m.compiler.AddSourceOffsetInfo(currentOffset, cur.sourceOffsetInfo())
		}
		currentOffset += cur.size()
	}
}

const (
	maxSignedInt26 = 1<<25 - 1
	minSignedInt26 = -(1 << 25)

	maxSignedInt19 = 1<<18 - 1
	minSignedInt19 = -(1 << 18)
)

func (m *machine) insertConditionalJumpTrampoline(cbr *instruction, currentBlk *labelPosition, nextLabel label) {
	cur := currentBlk.end
	originalTarget := cbr.condBrLabel()
	endNext := cur.next

	if cur.kind != br {
		// If the current block ends with a conditional branch, we can just insert the trampoline after it.
		// Otherwise, we need to insert "skip" instruction to skip the trampoline instructions.
		skip := m.allocateInstr()
		skip.asBr(nextLabel)
		cur = linkInstr(cur, skip)
	}

	cbrNewTargetInstr, cbrNewTargetLabel := m.allocateBrTarget()
	cbr.setCondBrTargets(cbrNewTargetLabel)
	cur = linkInstr(cur, cbrNewTargetInstr)

	// Then insert the unconditional branch to the original, which should be possible to get encoded
	// as 26-bit offset should be enough for any practical application.
	br := m.allocateInstr()
	br.asBr(originalTarget)
	cur = linkInstr(cur, br)

	// Update the end of the current block.
	currentBlk.end = cur

	linkInstr(cur, endNext)
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
				labelStr = fmt.Sprintf("%s (SSA Block: blk%d):", l, int(l))
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
	return "\n" + strings.Join(lines, "\n") + "\n"
}

// InsertReturn implements backend.Machine.
func (m *machine) InsertReturn() {
	i := m.allocateInstr()
	i.asRet()
	m.insert(i)
}

func (m *machine) getVRegSpillSlotOffsetFromSP(id regalloc.VRegID, size byte) int64 {
	offset, ok := m.spillSlots[id]
	if !ok {
		offset = m.spillSlotSize
		// TODO: this should be aligned depending on the `size` to use Imm12 offset load/store as much as possible.
		m.spillSlots[id] = offset
		m.spillSlotSize += int64(size)
	}
	return offset + 16 // spill slot starts above the clobbered registers and the frame size.
}

func (m *machine) clobberedRegSlotSize() int64 {
	return int64(len(m.clobberedRegs) * 16)
}

func (m *machine) arg0OffsetFromSP() int64 {
	return m.frameSize() +
		16 + // 16-byte aligned return address
		16 // frame size saved below the clobbered registers.
}

func (m *machine) ret0OffsetFromSP() int64 {
	return m.arg0OffsetFromSP() + m.currentABI.ArgStackSize
}

func (m *machine) requiredStackSize() int64 {
	return m.maxRequiredStackSizeForCalls +
		m.frameSize() +
		16 + // 16-byte aligned return address.
		16 // frame size saved below the clobbered registers.
}

func (m *machine) frameSize() int64 {
	s := m.clobberedRegSlotSize() + m.spillSlotSize
	if s&0xf != 0 {
		panic(fmt.Errorf("BUG: frame size %d is not 16-byte aligned", s))
	}
	return s
}

func (m *machine) addJmpTableTarget(targets ssa.Values) (index int) {
	if m.jmpTableTargetsNext == len(m.jmpTableTargets) {
		m.jmpTableTargets = append(m.jmpTableTargets, make([]uint32, 0, len(targets.View())))
	}

	index = m.jmpTableTargetsNext
	m.jmpTableTargetsNext++
	m.jmpTableTargets[index] = m.jmpTableTargets[index][:0]
	for _, targetBlockID := range targets.View() {
		target := m.compiler.SSABuilder().BasicBlock(ssa.BasicBlockID(targetBlockID))
		m.jmpTableTargets[index] = append(m.jmpTableTargets[index], uint32(target.ID()))
	}
	return
}
