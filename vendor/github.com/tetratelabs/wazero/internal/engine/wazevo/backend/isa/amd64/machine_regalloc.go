package amd64

import (
	"github.com/tetratelabs/wazero/internal/engine/wazevo/backend/regalloc"
	"github.com/tetratelabs/wazero/internal/engine/wazevo/ssa"
)

// regAllocFn implements regalloc.Function.
type regAllocFn struct {
	ssaB                   ssa.Builder
	m                      *machine
	loopNestingForestRoots []ssa.BasicBlock
	blockIter              int
}

// PostOrderBlockIteratorBegin implements regalloc.Function.
func (f *regAllocFn) PostOrderBlockIteratorBegin() *labelPosition {
	f.blockIter = len(f.m.orderedSSABlockLabelPos) - 1
	return f.PostOrderBlockIteratorNext()
}

// PostOrderBlockIteratorNext implements regalloc.Function.
func (f *regAllocFn) PostOrderBlockIteratorNext() *labelPosition {
	if f.blockIter < 0 {
		return nil
	}
	b := f.m.orderedSSABlockLabelPos[f.blockIter]
	f.blockIter--
	return b
}

// ReversePostOrderBlockIteratorBegin implements regalloc.Function.
func (f *regAllocFn) ReversePostOrderBlockIteratorBegin() *labelPosition {
	f.blockIter = 0
	return f.ReversePostOrderBlockIteratorNext()
}

// ReversePostOrderBlockIteratorNext implements regalloc.Function.
func (f *regAllocFn) ReversePostOrderBlockIteratorNext() *labelPosition {
	if f.blockIter >= len(f.m.orderedSSABlockLabelPos) {
		return nil
	}
	b := f.m.orderedSSABlockLabelPos[f.blockIter]
	f.blockIter++
	return b
}

// ClobberedRegisters implements regalloc.Function.
func (f *regAllocFn) ClobberedRegisters(regs []regalloc.VReg) {
	f.m.clobberedRegs = append(f.m.clobberedRegs[:0], regs...)
}

// LoopNestingForestRoots implements regalloc.Function.
func (f *regAllocFn) LoopNestingForestRoots() int {
	f.loopNestingForestRoots = f.ssaB.LoopNestingForestRoots()
	return len(f.loopNestingForestRoots)
}

// LoopNestingForestRoot implements regalloc.Function.
func (f *regAllocFn) LoopNestingForestRoot(i int) *labelPosition {
	root := f.loopNestingForestRoots[i]
	pos := f.m.getOrAllocateSSABlockLabelPosition(root)
	return pos
}

// LowestCommonAncestor implements regalloc.Function.
func (f *regAllocFn) LowestCommonAncestor(blk1, blk2 *labelPosition) *labelPosition {
	sb := f.ssaB.LowestCommonAncestor(blk1.sb, blk2.sb)
	pos := f.m.getOrAllocateSSABlockLabelPosition(sb)
	return pos
}

// Idom implements regalloc.Function.
func (f *regAllocFn) Idom(blk *labelPosition) *labelPosition {
	sb := f.ssaB.Idom(blk.sb)
	pos := f.m.getOrAllocateSSABlockLabelPosition(sb)
	return pos
}

// SwapBefore implements regalloc.Function.
func (f *regAllocFn) SwapBefore(x1, x2, tmp regalloc.VReg, instr *instruction) {
	f.m.swap(instr.prev, x1, x2, tmp)
}

// StoreRegisterBefore implements regalloc.Function.
func (f *regAllocFn) StoreRegisterBefore(v regalloc.VReg, instr *instruction) {
	m := f.m
	m.insertStoreRegisterAt(v, instr, false)
}

// StoreRegisterAfter implements regalloc.Function.
func (f *regAllocFn) StoreRegisterAfter(v regalloc.VReg, instr *instruction) {
	m := f.m
	m.insertStoreRegisterAt(v, instr, true)
}

// ReloadRegisterBefore implements regalloc.Function.
func (f *regAllocFn) ReloadRegisterBefore(v regalloc.VReg, instr *instruction) {
	m := f.m
	m.insertReloadRegisterAt(v, instr, false)
}

// ReloadRegisterAfter implements regalloc.Function.
func (f *regAllocFn) ReloadRegisterAfter(v regalloc.VReg, instr *instruction) {
	m := f.m
	m.insertReloadRegisterAt(v, instr, true)
}

// InsertMoveBefore implements regalloc.Function.
func (f *regAllocFn) InsertMoveBefore(dst, src regalloc.VReg, instr *instruction) {
	f.m.insertMoveBefore(dst, src, instr)
}

// LoopNestingForestChild implements regalloc.Function.
func (f *regAllocFn) LoopNestingForestChild(pos *labelPosition, i int) *labelPosition {
	childSB := pos.sb.LoopNestingForestChildren()[i]
	return f.m.getOrAllocateSSABlockLabelPosition(childSB)
}

// Succ implements regalloc.Block.
func (f *regAllocFn) Succ(pos *labelPosition, i int) *labelPosition {
	succSB := pos.sb.Succ(i)
	if succSB.ReturnBlock() {
		return nil
	}
	return f.m.getOrAllocateSSABlockLabelPosition(succSB)
}

// Pred implements regalloc.Block.
func (f *regAllocFn) Pred(pos *labelPosition, i int) *labelPosition {
	predSB := pos.sb.Pred(i)
	return f.m.getOrAllocateSSABlockLabelPosition(predSB)
}

// BlockParams implements regalloc.Function.
func (f *regAllocFn) BlockParams(pos *labelPosition, regs *[]regalloc.VReg) []regalloc.VReg {
	c := f.m.c
	*regs = (*regs)[:0]
	for i := 0; i < pos.sb.Params(); i++ {
		v := c.VRegOf(pos.sb.Param(i))
		*regs = append(*regs, v)
	}
	return *regs
}

// ID implements regalloc.Block.
func (pos *labelPosition) ID() int32 {
	return int32(pos.sb.ID())
}

// InstrIteratorBegin implements regalloc.Block.
func (pos *labelPosition) InstrIteratorBegin() *instruction {
	ret := pos.begin
	pos.cur = ret
	return ret
}

// InstrIteratorNext implements regalloc.Block.
func (pos *labelPosition) InstrIteratorNext() *instruction {
	for {
		if pos.cur == pos.end {
			return nil
		}
		instr := pos.cur.next
		pos.cur = instr
		if instr == nil {
			return nil
		} else if instr.addedBeforeRegAlloc {
			// Only concerned about the instruction added before regalloc.
			return instr
		}
	}
}

// InstrRevIteratorBegin implements regalloc.Block.
func (pos *labelPosition) InstrRevIteratorBegin() *instruction {
	pos.cur = pos.end
	return pos.cur
}

// InstrRevIteratorNext implements regalloc.Block.
func (pos *labelPosition) InstrRevIteratorNext() *instruction {
	for {
		if pos.cur == pos.begin {
			return nil
		}
		instr := pos.cur.prev
		pos.cur = instr
		if instr == nil {
			return nil
		} else if instr.addedBeforeRegAlloc {
			// Only concerned about the instruction added before regalloc.
			return instr
		}
	}
}

// FirstInstr implements regalloc.Block.
func (pos *labelPosition) FirstInstr() *instruction { return pos.begin }

// LastInstrForInsertion implements regalloc.Block.
func (pos *labelPosition) LastInstrForInsertion() *instruction {
	return lastInstrForInsertion(pos.begin, pos.end)
}

// Preds implements regalloc.Block.
func (pos *labelPosition) Preds() int { return pos.sb.Preds() }

// Entry implements regalloc.Block.
func (pos *labelPosition) Entry() bool { return pos.sb.EntryBlock() }

// Succs implements regalloc.Block.
func (pos *labelPosition) Succs() int { return pos.sb.Succs() }

// LoopHeader implements regalloc.Block.
func (pos *labelPosition) LoopHeader() bool { return pos.sb.LoopHeader() }

// LoopNestingForestChildren implements regalloc.Block.
func (pos *labelPosition) LoopNestingForestChildren() int {
	return len(pos.sb.LoopNestingForestChildren())
}

func (m *machine) insertMoveBefore(dst, src regalloc.VReg, instr *instruction) {
	typ := src.RegType()
	if typ != dst.RegType() {
		panic("BUG: src and dst must have the same type")
	}

	mov := m.allocateInstr()
	if typ == regalloc.RegTypeInt {
		mov.asMovRR(src, dst, true)
	} else {
		mov.asXmmUnaryRmR(sseOpcodeMovdqu, newOperandReg(src), dst)
	}

	cur := instr.prev
	prevNext := cur.next
	cur = linkInstr(cur, mov)
	linkInstr(cur, prevNext)
}

func (m *machine) insertStoreRegisterAt(v regalloc.VReg, instr *instruction, after bool) *instruction {
	if !v.IsRealReg() {
		panic("BUG: VReg must be backed by real reg to be stored")
	}

	typ := m.c.TypeOf(v)

	var prevNext, cur *instruction
	if after {
		cur, prevNext = instr, instr.next
	} else {
		cur, prevNext = instr.prev, instr
	}

	offsetFromSP := m.getVRegSpillSlotOffsetFromSP(v.ID(), typ.Size())
	store := m.allocateInstr()
	mem := newOperandMem(m.newAmodeImmReg(uint32(offsetFromSP), rspVReg))
	switch typ {
	case ssa.TypeI32:
		store.asMovRM(v, mem, 4)
	case ssa.TypeI64:
		store.asMovRM(v, mem, 8)
	case ssa.TypeF32:
		store.asXmmMovRM(sseOpcodeMovss, v, mem)
	case ssa.TypeF64:
		store.asXmmMovRM(sseOpcodeMovsd, v, mem)
	case ssa.TypeV128:
		store.asXmmMovRM(sseOpcodeMovdqu, v, mem)
	}

	cur = linkInstr(cur, store)
	return linkInstr(cur, prevNext)
}

func (m *machine) insertReloadRegisterAt(v regalloc.VReg, instr *instruction, after bool) *instruction {
	if !v.IsRealReg() {
		panic("BUG: VReg must be backed by real reg to be stored")
	}

	typ := m.c.TypeOf(v)
	var prevNext, cur *instruction
	if after {
		cur, prevNext = instr, instr.next
	} else {
		cur, prevNext = instr.prev, instr
	}

	// Load the value to the temporary.
	load := m.allocateInstr()
	offsetFromSP := m.getVRegSpillSlotOffsetFromSP(v.ID(), typ.Size())
	a := newOperandMem(m.newAmodeImmReg(uint32(offsetFromSP), rspVReg))
	switch typ {
	case ssa.TypeI32:
		load.asMovzxRmR(extModeLQ, a, v)
	case ssa.TypeI64:
		load.asMov64MR(a, v)
	case ssa.TypeF32:
		load.asXmmUnaryRmR(sseOpcodeMovss, a, v)
	case ssa.TypeF64:
		load.asXmmUnaryRmR(sseOpcodeMovsd, a, v)
	case ssa.TypeV128:
		load.asXmmUnaryRmR(sseOpcodeMovdqu, a, v)
	default:
		panic("BUG")
	}

	cur = linkInstr(cur, load)
	return linkInstr(cur, prevNext)
}

func (m *machine) swap(cur *instruction, x1, x2, tmp regalloc.VReg) {
	if x1.RegType() == regalloc.RegTypeInt {
		prevNext := cur.next
		xc := m.allocateInstr().asXCHG(x1, newOperandReg(x2), 8)
		cur = linkInstr(cur, xc)
		linkInstr(cur, prevNext)
	} else {
		if tmp.Valid() {
			prevNext := cur.next
			m.insertMoveBefore(tmp, x1, prevNext)
			m.insertMoveBefore(x1, x2, prevNext)
			m.insertMoveBefore(x2, tmp, prevNext)
		} else {
			prevNext := cur.next
			r2 := x2.RealReg()
			// Temporarily spill x1 to stack.
			cur = m.insertStoreRegisterAt(x1, cur, true).prev
			// Then move x2 to x1.
			cur = linkInstr(cur, m.allocateInstr().asXmmUnaryRmR(sseOpcodeMovdqa, newOperandReg(x2), x1))
			linkInstr(cur, prevNext)
			// Then reload the original value on x1 from stack to r2.
			m.insertReloadRegisterAt(x1.SetRealReg(r2), cur, true)
		}
	}
}

func lastInstrForInsertion(begin, end *instruction) *instruction {
	cur := end
	for cur.kind == nop0 {
		cur = cur.prev
		if cur == begin {
			return end
		}
	}
	switch cur.kind {
	case jmp:
		return cur
	default:
		return end
	}
}
