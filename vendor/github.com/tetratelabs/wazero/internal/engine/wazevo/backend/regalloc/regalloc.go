// Package regalloc performs register allocation. The algorithm can work on any ISA by implementing the interfaces in
// api.go.
//
// References:
//   - https://web.stanford.edu/class/archive/cs/cs143/cs143.1128/lectures/17/Slides17.pdf
//   - https://en.wikipedia.org/wiki/Chaitin%27s_algorithm
//   - https://llvm.org/ProjectsWithLLVM/2004-Fall-CS426-LS.pdf
//   - https://pfalcon.github.io/ssabook/latest/book-full.pdf: Chapter 9. for liveness analysis.
//   - https://github.com/golang/go/blob/release-branch.go1.21/src/cmd/compile/internal/ssa/regalloc.go
package regalloc

import (
	"fmt"
	"math"
	"strings"

	"github.com/tetratelabs/wazero/internal/engine/wazevo/wazevoapi"
)

// NewAllocator returns a new Allocator.
func NewAllocator[I Instr, B Block[I], F Function[I, B]](allocatableRegs *RegisterInfo) Allocator[I, B, F] {
	a := Allocator[I, B, F]{
		regInfo:            allocatableRegs,
		phiDefInstListPool: wazevoapi.NewPool[phiDefInstList[I]](resetPhiDefInstList[I]),
		blockStates:        wazevoapi.NewIDedPool[blockState[I, B, F]](resetBlockState[I, B, F]),
	}
	a.state.vrStates = wazevoapi.NewIDedPool[vrState[I, B, F]](resetVrState[I, B, F])
	a.state.reset()
	for _, regs := range allocatableRegs.AllocatableRegisters {
		for _, r := range regs {
			a.allocatableSet = a.allocatableSet.add(r)
		}
	}
	return a
}

type (
	// RegisterInfo holds the statically-known ISA-specific register information.
	RegisterInfo struct {
		// AllocatableRegisters is a 2D array of allocatable RealReg, indexed by regTypeNum and regNum.
		// The order matters: the first element is the most preferred one when allocating.
		AllocatableRegisters [NumRegType][]RealReg
		CalleeSavedRegisters RegSet
		CallerSavedRegisters RegSet
		RealRegToVReg        []VReg
		// RealRegName returns the name of the given RealReg for debugging.
		RealRegName func(r RealReg) string
		RealRegType func(r RealReg) RegType
	}

	// Allocator is a register allocator.
	Allocator[I Instr, B Block[I], F Function[I, B]] struct {
		// regInfo is static per ABI/ISA, and is initialized by the machine during Machine.PrepareRegisterAllocator.
		regInfo *RegisterInfo
		// allocatableSet is a set of allocatable RealReg derived from regInfo. Static per ABI/ISA.
		allocatableSet           RegSet
		allocatedCalleeSavedRegs []VReg
		vs                       []VReg
		ss                       []*vrState[I, B, F]
		copies                   []_copy[I, B, F]
		phiDefInstListPool       wazevoapi.Pool[phiDefInstList[I]]

		// Followings are re-used during various places.
		blks  []B
		reals []RealReg

		// Following two fields are updated while iterating the blocks in the reverse postorder.
		state       state[I, B, F]
		blockStates wazevoapi.IDedPool[blockState[I, B, F]]
	}

	// _copy represents a source and destination pair of a copy instruction.
	_copy[I Instr, B Block[I], F Function[I, B]] struct {
		src   *vrState[I, B, F]
		dstID VRegID
	}

	// programCounter represents an opaque index into the program which is used to represents a LiveInterval of a VReg.
	programCounter int32

	state[I Instr, B Block[I], F Function[I, B]] struct {
		argRealRegs []VReg
		regsInUse   regInUseSet[I, B, F]
		vrStates    wazevoapi.IDedPool[vrState[I, B, F]]

		currentBlockID int32

		// allocatedRegSet is a set of RealReg that are allocated during the allocation phase. This is reset per function.
		allocatedRegSet RegSet
	}

	blockState[I Instr, B Block[I], F Function[I, B]] struct {
		// liveIns is a list of VReg that are live at the beginning of the block.
		liveIns []*vrState[I, B, F]
		// seen is true if the block is visited during the liveness analysis.
		seen bool
		// visited is true if the block is visited during the allocation phase.
		visited            bool
		startFromPredIndex int
		// startRegs is a list of RealReg that are used at the beginning of the block. This is used to fix the merge edges.
		startRegs regInUseSet[I, B, F]
		// endRegs is a list of RealReg that are used at the end of the block. This is used to fix the merge edges.
		endRegs regInUseSet[I, B, F]
	}

	vrState[I Instr, B Block[I], f Function[I, B]] struct {
		v VReg
		r RealReg
		// defInstr is the instruction that defines this value. If this is the phi value and not the entry block, this is nil.
		defInstr I
		// defBlk is the block that defines this value. If this is the phi value, this is the block whose arguments contain this value.
		defBlk B
		// lca = lowest common ancestor. This is the block that is the lowest common ancestor of all the blocks that
		// reloads this value. This is used to determine the spill location. Only valid if spilled=true.
		lca B
		// lastUse is the program counter of the last use of this value. This changes while iterating the block, and
		// should not be used across the blocks as it becomes invalid. To check the validity, use lastUseUpdatedAtBlockID.
		lastUse                 programCounter
		lastUseUpdatedAtBlockID int32
		// spilled is true if this value is spilled i.e. the value is reload from the stack somewhere in the program.
		//
		// Note that this field is used during liveness analysis for different purpose. This is used to determine the
		// value is live-in or not.
		spilled bool
		// isPhi is true if this is a phi value.
		isPhi      bool
		desiredLoc desiredLoc
		// phiDefInstList is a list of instructions that defines this phi value.
		// This is used to determine the spill location, and only valid if isPhi=true.
		*phiDefInstList[I]
	}

	// phiDefInstList is a linked list of instructions that defines a phi value.
	phiDefInstList[I Instr] struct {
		instr I
		v     VReg
		next  *phiDefInstList[I]
	}

	// desiredLoc represents a desired location for a VReg.
	desiredLoc uint16
	// desiredLocKind is a kind of desired location for a VReg.
	desiredLocKind uint16
)

const (
	// desiredLocKindUnspecified is a kind of desired location for a VReg that is not specified.
	desiredLocKindUnspecified desiredLocKind = iota
	// desiredLocKindStack is a kind of desired location for a VReg that is on the stack, only used for the phi values.
	desiredLocKindStack
	// desiredLocKindReg is a kind of desired location for a VReg that is in a register.
	desiredLocKindReg
	desiredLocUnspecified = desiredLoc(desiredLocKindUnspecified)
	desiredLocStack       = desiredLoc(desiredLocKindStack)
)

func newDesiredLocReg(r RealReg) desiredLoc {
	return desiredLoc(desiredLocKindReg) | desiredLoc(r<<2)
}

func (d desiredLoc) realReg() RealReg {
	return RealReg(d >> 2)
}

func (d desiredLoc) stack() bool {
	return d&3 == desiredLoc(desiredLocKindStack)
}

func resetPhiDefInstList[I Instr](l *phiDefInstList[I]) {
	var nilInstr I
	l.instr = nilInstr
	l.next = nil
	l.v = VRegInvalid
}

func (s *state[I, B, F]) dump(info *RegisterInfo) { //nolint:unused
	fmt.Println("\t\tstate:")
	fmt.Println("\t\t\targRealRegs:", s.argRealRegs)
	fmt.Println("\t\t\tregsInUse", s.regsInUse.format(info))
	fmt.Println("\t\t\tallocatedRegSet:", s.allocatedRegSet.format(info))
	fmt.Println("\t\t\tused:", s.regsInUse.format(info))
	var strs []string
	for i := 0; i <= s.vrStates.MaxIDEncountered(); i++ {
		vs := s.vrStates.Get(i)
		if vs == nil {
			continue
		}
		if vs.r != RealRegInvalid {
			strs = append(strs, fmt.Sprintf("(v%d: %s)", vs.v.ID(), info.RealRegName(vs.r)))
		}
	}
	fmt.Println("\t\t\tvrStates:", strings.Join(strs, ", "))
}

func (s *state[I, B, F]) reset() {
	s.argRealRegs = s.argRealRegs[:0]
	s.vrStates.Reset()
	s.allocatedRegSet = RegSet(0)
	s.regsInUse.reset()
	s.currentBlockID = -1
}

func resetVrState[I Instr, B Block[I], F Function[I, B]](vs *vrState[I, B, F]) {
	vs.v = VRegInvalid
	vs.r = RealRegInvalid
	var nilInstr I
	vs.defInstr = nilInstr
	var nilBlk B
	vs.defBlk = nilBlk
	vs.spilled = false
	vs.lastUse = -1
	vs.lastUseUpdatedAtBlockID = -1
	vs.lca = nilBlk
	vs.isPhi = false
	vs.phiDefInstList = nil
	vs.desiredLoc = desiredLocUnspecified
}

func (s *state[I, B, F]) getOrAllocateVRegState(v VReg) *vrState[I, B, F] {
	st := s.vrStates.GetOrAllocate(int(v.ID()))
	if st.v == VRegInvalid {
		st.v = v
	}
	return st
}

func (s *state[I, B, F]) getVRegState(v VRegID) *vrState[I, B, F] {
	return s.vrStates.Get(int(v))
}

func (s *state[I, B, F]) useRealReg(r RealReg, vr *vrState[I, B, F]) {
	s.regsInUse.add(r, vr)
	vr.r = r
	s.allocatedRegSet = s.allocatedRegSet.add(r)
}

func (s *state[I, B, F]) releaseRealReg(r RealReg) {
	current := s.regsInUse.get(r)
	if current != nil {
		s.regsInUse.remove(r)
		current.r = RealRegInvalid
	}
}

// recordReload records that the given VReg is reloaded in the given block.
// This is used to determine the spill location by tracking the lowest common ancestor of all the blocks that reloads the value.
func (vs *vrState[I, B, F]) recordReload(f F, blk B) {
	vs.spilled = true
	var nilBlk B
	if lca := vs.lca; lca == nilBlk {
		if wazevoapi.RegAllocLoggingEnabled {
			fmt.Printf("\t\tv%d is reloaded in blk%d,\n", vs.v.ID(), blk.ID())
		}
		vs.lca = blk
	} else if lca != blk {
		if wazevoapi.RegAllocLoggingEnabled {
			fmt.Printf("\t\tv%d is reloaded in blk%d, lca=%d\n", vs.v.ID(), blk.ID(), vs.lca.ID())
		}
		vs.lca = f.LowestCommonAncestor(lca, blk)
		if wazevoapi.RegAllocLoggingEnabled {
			fmt.Printf("updated lca=%d\n", vs.lca.ID())
		}
	}
}

func (a *Allocator[I, B, F]) findOrSpillAllocatable(s *state[I, B, F], allocatable []RealReg, forbiddenMask RegSet, preferred RealReg) (r RealReg) {
	r = RealRegInvalid
	// First, check if the preferredMask has any allocatable register.
	if preferred != RealRegInvalid && !forbiddenMask.has(preferred) && !s.regsInUse.has(preferred) {
		return preferred
	}

	var lastUseAt programCounter
	var spillVReg VReg
	for _, candidateReal := range allocatable {
		if forbiddenMask.has(candidateReal) {
			continue
		}

		using := s.regsInUse.get(candidateReal)
		if using == nil {
			// This is not used at this point.
			return candidateReal
		}

		// Real registers in use should not be spilled, so we skip them.
		// For example, if the register is used as an argument register, and it might be
		// spilled and not reloaded when it ends up being used as a temporary to pass
		// stack based argument.
		if using.v.IsRealReg() {
			continue
		}

		isPreferred := candidateReal == preferred

		// last == -1 means the value won't be used anymore.
		if last := using.lastUse; r == RealRegInvalid || isPreferred || last == -1 || (lastUseAt != -1 && last > lastUseAt) {
			lastUseAt = last
			r = candidateReal
			spillVReg = using.v
			if isPreferred {
				break
			}
		}
	}

	if r == RealRegInvalid {
		panic("not found any allocatable register")
	}

	if wazevoapi.RegAllocLoggingEnabled {
		fmt.Printf("\tspilling v%d when lastUseAt=%d and regsInUse=%s\n", spillVReg.ID(), lastUseAt, s.regsInUse.format(a.regInfo))
	}
	s.releaseRealReg(r)
	return r
}

func (s *state[I, B, F]) findAllocatable(allocatable []RealReg, forbiddenMask RegSet) RealReg {
	for _, r := range allocatable {
		if !s.regsInUse.has(r) && !forbiddenMask.has(r) {
			return r
		}
	}
	return RealRegInvalid
}

func (s *state[I, B, F]) resetAt(bs *blockState[I, B, F]) {
	s.regsInUse.range_(func(_ RealReg, vs *vrState[I, B, F]) {
		vs.r = RealRegInvalid
	})
	s.regsInUse.reset()
	bs.endRegs.range_(func(r RealReg, vs *vrState[I, B, F]) {
		if vs.lastUseUpdatedAtBlockID == s.currentBlockID && vs.lastUse == programCounterLiveIn {
			s.regsInUse.add(r, vs)
			vs.r = r
		}
	})
}

func resetBlockState[I Instr, B Block[I], F Function[I, B]](b *blockState[I, B, F]) {
	b.seen = false
	b.visited = false
	b.endRegs.reset()
	b.startRegs.reset()
	b.startFromPredIndex = -1
	b.liveIns = b.liveIns[:0]
}

func (b *blockState[I, B, F]) dump(a *RegisterInfo) {
	fmt.Println("\t\tblockState:")
	fmt.Println("\t\t\tstartRegs:", b.startRegs.format(a))
	fmt.Println("\t\t\tendRegs:", b.endRegs.format(a))
	fmt.Println("\t\t\tstartFromPredIndex:", b.startFromPredIndex)
	fmt.Println("\t\t\tvisited:", b.visited)
}

// DoAllocation performs register allocation on the given Function.
func (a *Allocator[I, B, F]) DoAllocation(f F) {
	a.livenessAnalysis(f)
	a.alloc(f)
	a.determineCalleeSavedRealRegs(f)
}

func (a *Allocator[I, B, F]) determineCalleeSavedRealRegs(f F) {
	a.allocatedCalleeSavedRegs = a.allocatedCalleeSavedRegs[:0]
	a.state.allocatedRegSet.Range(func(allocatedRealReg RealReg) {
		if a.regInfo.CalleeSavedRegisters.has(allocatedRealReg) {
			a.allocatedCalleeSavedRegs = append(a.allocatedCalleeSavedRegs, a.regInfo.RealRegToVReg[allocatedRealReg])
		}
	})
	f.ClobberedRegisters(a.allocatedCalleeSavedRegs)
}

func (a *Allocator[I, B, F]) getOrAllocateBlockState(blockID int32) *blockState[I, B, F] {
	return a.blockStates.GetOrAllocate(int(blockID))
}

// phiBlk returns the block that defines the given phi value, nil otherwise.
func (vs *vrState[I, B, F]) phiBlk() B {
	if vs.isPhi {
		return vs.defBlk
	}
	var nilBlk B
	return nilBlk
}

const (
	programCounterLiveIn  = math.MinInt32
	programCounterLiveOut = math.MaxInt32
)

// liveAnalysis constructs Allocator.blockLivenessData.
// The algorithm here is described in https://pfalcon.github.io/ssabook/latest/book-full.pdf Chapter 9.2.
func (a *Allocator[I, B, F]) livenessAnalysis(f F) {
	s := &a.state

	for i := VRegID(0); i < vRegIDReservedForRealNum; i++ {
		s.getOrAllocateVRegState(VReg(i).SetRealReg(RealReg(i)))
	}

	var nilBlk B
	var nilInstr I
	for blk := f.PostOrderBlockIteratorBegin(); blk != nilBlk; blk = f.PostOrderBlockIteratorNext() {
		// We should gather phi value data.
		for _, p := range f.BlockParams(blk, &a.vs) {
			vs := s.getOrAllocateVRegState(p)
			vs.isPhi = true
			vs.defBlk = blk
		}

		blkID := blk.ID()
		info := a.getOrAllocateBlockState(blkID)

		a.ss = a.ss[:0]
		const (
			flagDeleted = false
			flagLive    = true
		)
		ns := blk.Succs()
		for i := 0; i < ns; i++ {
			succ := f.Succ(blk, i)
			if succ == nilBlk {
				continue
			}

			succID := succ.ID()
			succInfo := a.getOrAllocateBlockState(succID)
			if !succInfo.seen { // This means the back edge.
				continue
			}

			for _, st := range succInfo.liveIns {
				if st.phiBlk() != succ && st.spilled != flagLive { //nolint:gosimple
					// We use .spilled field to store the flag.
					st.spilled = flagLive
					a.ss = append(a.ss, st)
				}
			}
		}

		for instr := blk.InstrRevIteratorBegin(); instr != nilInstr; instr = blk.InstrRevIteratorNext() {

			var use, def VReg
			var defIsPhi bool
			for _, def = range instr.Defs(&a.vs) {
				if !def.IsRealReg() {
					st := s.getOrAllocateVRegState(def)
					defIsPhi = st.isPhi
					// Note: We use .spilled field to store the flag.
					st.spilled = flagDeleted
				}
			}
			for _, use = range instr.Uses(&a.vs) {
				if !use.IsRealReg() {
					st := s.getOrAllocateVRegState(use)
					// Note: We use .spilled field to store the flag.
					if st.spilled != flagLive { //nolint:gosimple
						st.spilled = flagLive
						a.ss = append(a.ss, st)
					}
				}
			}

			if defIsPhi {
				if use.Valid() && use.IsRealReg() {
					// If the destination is a phi value, and the source is a real register, this is the beginning of the function.
					a.state.argRealRegs = append(a.state.argRealRegs, use)
				}
			}
		}

		for _, st := range a.ss {
			// We use .spilled field to store the flag.
			if st.spilled == flagLive { //nolint:gosimple
				info.liveIns = append(info.liveIns, st)
				st.spilled = false
			}
		}

		info.seen = true
	}

	nrs := f.LoopNestingForestRoots()
	for i := 0; i < nrs; i++ {
		root := f.LoopNestingForestRoot(i)
		a.loopTreeDFS(f, root)
	}
}

// loopTreeDFS implements the Algorithm 9.3 in the book in an iterative way.
func (a *Allocator[I, B, F]) loopTreeDFS(f F, entry B) {
	a.blks = a.blks[:0]
	a.blks = append(a.blks, entry)

	for len(a.blks) > 0 {
		tail := len(a.blks) - 1
		loop := a.blks[tail]
		a.blks = a.blks[:tail]
		a.ss = a.ss[:0]
		const (
			flagDone    = false
			flagPending = true
		)
		info := a.getOrAllocateBlockState(loop.ID())
		for _, st := range info.liveIns {
			if st.phiBlk() != loop {
				a.ss = append(a.ss, st)
				// We use .spilled field to store the flag.
				st.spilled = flagPending
			}
		}

		var siblingAddedView []*vrState[I, B, F]
		cn := loop.LoopNestingForestChildren()
		for i := 0; i < cn; i++ {
			child := f.LoopNestingForestChild(loop, i)
			childID := child.ID()
			childInfo := a.getOrAllocateBlockState(childID)

			if i == 0 {
				begin := len(childInfo.liveIns)
				for _, st := range a.ss {
					// We use .spilled field to store the flag.
					if st.spilled == flagPending { //nolint:gosimple
						st.spilled = flagDone
						// TODO: deduplicate, though I don't think it has much impact.
						childInfo.liveIns = append(childInfo.liveIns, st)
					}
				}
				siblingAddedView = childInfo.liveIns[begin:]
			} else {
				// TODO: deduplicate, though I don't think it has much impact.
				childInfo.liveIns = append(childInfo.liveIns, siblingAddedView...)
			}

			if child.LoopHeader() {
				a.blks = append(a.blks, child)
			}
		}

		if cn == 0 {
			// If there's no forest child, we haven't cleared the .spilled field at this point.
			for _, st := range a.ss {
				st.spilled = false
			}
		}
	}
}

// alloc allocates registers for the given function by iterating the blocks in the reverse postorder.
// The algorithm here is derived from the Go compiler's allocator https://github.com/golang/go/blob/release-branch.go1.21/src/cmd/compile/internal/ssa/regalloc.go
// In short, this is a simply linear scan register allocation where each block inherits the register allocation state from
// one of its predecessors. Each block inherits the selected state and starts allocation from there.
// If there's a discrepancy in the end states between predecessors, the adjustments are made to ensure consistency after allocation is done (which we call "fixing merge state").
// The spill instructions (store into the dedicated slots) are inserted after all the allocations and fixing merge states. That is because
// at the point, we all know where the reloads happen, and therefore we can know the best place to spill the values. More precisely,
// the spill happens in the block that is the lowest common ancestor of all the blocks that reloads the value.
//
// All of these logics are almost the same as Go's compiler which has a dedicated description in the source file ^^.
func (a *Allocator[I, B, F]) alloc(f F) {
	// First we allocate each block in the reverse postorder (at least one predecessor should be allocated for each block).
	var nilBlk B
	for blk := f.ReversePostOrderBlockIteratorBegin(); blk != nilBlk; blk = f.ReversePostOrderBlockIteratorNext() {
		if wazevoapi.RegAllocLoggingEnabled {
			fmt.Printf("========== allocating blk%d ========\n", blk.ID())
		}
		if blk.Entry() {
			a.finalizeStartReg(f, blk)
		}
		a.allocBlock(f, blk)
	}
	// After the allocation, we all know the start and end state of each block. So we can fix the merge states.
	for blk := f.ReversePostOrderBlockIteratorBegin(); blk != nilBlk; blk = f.ReversePostOrderBlockIteratorNext() {
		a.fixMergeState(f, blk)
	}
	// Finally, we insert the spill instructions as we know all the places where the reloads happen.
	a.scheduleSpills(f)
}

func (a *Allocator[I, B, F]) updateLiveInVRState(liveness *blockState[I, B, F]) {
	currentBlockID := a.state.currentBlockID
	for _, vs := range liveness.liveIns {
		vs.lastUse = programCounterLiveIn
		vs.lastUseUpdatedAtBlockID = currentBlockID
	}
}

func (a *Allocator[I, B, F]) finalizeStartReg(f F, blk B) {
	bID := blk.ID()
	s := &a.state
	currentBlkState := a.getOrAllocateBlockState(bID)
	if currentBlkState.startFromPredIndex > -1 {
		return
	}

	s.currentBlockID = bID
	a.updateLiveInVRState(currentBlkState)

	preds := blk.Preds()
	var predState *blockState[I, B, F]
	switch preds {
	case 0: // This is the entry block.
	case 1:
		predID := f.Pred(blk, 0).ID()
		predState = a.getOrAllocateBlockState(predID)
		currentBlkState.startFromPredIndex = 0
	default:
		// TODO: there should be some better heuristic to choose the predecessor.
		for i := 0; i < preds; i++ {
			predID := f.Pred(blk, i).ID()
			if _predState := a.getOrAllocateBlockState(predID); _predState.visited {
				predState = _predState
				currentBlkState.startFromPredIndex = i
				break
			}
		}
	}
	if predState == nil {
		if !blk.Entry() {
			panic(fmt.Sprintf("BUG: at lease one predecessor should be visited for blk%d", blk.ID()))
		}
		for _, u := range s.argRealRegs {
			s.useRealReg(u.RealReg(), s.getVRegState(u.ID()))
		}
		currentBlkState.startFromPredIndex = 0
	} else {
		if wazevoapi.RegAllocLoggingEnabled {
			fmt.Printf("allocating blk%d starting from blk%d (on index=%d) \n",
				bID, f.Pred(blk, currentBlkState.startFromPredIndex).ID(), currentBlkState.startFromPredIndex)
		}
		s.resetAt(predState)
	}

	s.regsInUse.range_(func(allocated RealReg, v *vrState[I, B, F]) {
		currentBlkState.startRegs.add(allocated, v)
	})
	if wazevoapi.RegAllocLoggingEnabled {
		fmt.Printf("finalized start reg for blk%d: %s\n", blk.ID(), currentBlkState.startRegs.format(a.regInfo))
	}
}

func (a *Allocator[I, B, F]) allocBlock(f F, blk B) {
	bID := blk.ID()
	s := &a.state
	currentBlkState := a.getOrAllocateBlockState(bID)
	s.currentBlockID = bID

	if currentBlkState.startFromPredIndex < 0 {
		panic("BUG: startFromPredIndex should be set in finalizeStartReg prior to allocBlock")
	}

	// Clears the previous state.
	s.regsInUse.range_(func(allocatedRealReg RealReg, vr *vrState[I, B, F]) { vr.r = RealRegInvalid })
	s.regsInUse.reset()
	// Then set the start state.
	currentBlkState.startRegs.range_(func(allocatedRealReg RealReg, vr *vrState[I, B, F]) { s.useRealReg(allocatedRealReg, vr) })

	desiredUpdated := a.ss[:0]

	// Update the last use of each VReg.
	a.copies = a.copies[:0] // Stores the copy instructions.
	var pc programCounter
	var nilInstr I
	for instr := blk.InstrIteratorBegin(); instr != nilInstr; instr = blk.InstrIteratorNext() {
		var useState *vrState[I, B, F]
		for _, use := range instr.Uses(&a.vs) {
			useState = s.getVRegState(use.ID())
			if !use.IsRealReg() {
				useState.lastUse = pc
			}
		}

		if instr.IsCopy() {
			def := instr.Defs(&a.vs)[0]
			a.copies = append(a.copies, _copy[I, B, F]{src: useState, dstID: def.ID()})
			r := def.RealReg()
			if r != RealRegInvalid {
				if !useState.isPhi { // TODO: no idea why do we need this.
					useState.desiredLoc = newDesiredLocReg(r)
					desiredUpdated = append(desiredUpdated, useState)
				}
			}
		}
		pc++
	}

	// Mark all live-out values by checking live-in of the successors.
	// While doing so, we also update the desired register values.
	var succ B
	var nilBlk B
	for i, ns := 0, blk.Succs(); i < ns; i++ {
		succ = f.Succ(blk, i)
		if succ == nilBlk {
			continue
		}

		succID := succ.ID()
		succState := a.getOrAllocateBlockState(succID)
		for _, st := range succState.liveIns {
			if st.phiBlk() != succ {
				st.lastUse = programCounterLiveOut
			}
		}

		if succState.startFromPredIndex > -1 {
			if wazevoapi.RegAllocLoggingEnabled {
				fmt.Printf("blk%d -> blk%d: start_regs: %s\n", bID, succID, succState.startRegs.format(a.regInfo))
			}
			succState.startRegs.range_(func(allocatedRealReg RealReg, vs *vrState[I, B, F]) {
				vs.desiredLoc = newDesiredLocReg(allocatedRealReg)
				desiredUpdated = append(desiredUpdated, vs)
			})
			for _, p := range f.BlockParams(succ, &a.vs) {
				vs := s.getVRegState(p.ID())
				if vs.desiredLoc.realReg() == RealRegInvalid {
					vs.desiredLoc = desiredLocStack
					desiredUpdated = append(desiredUpdated, vs)
				}
			}
		}
	}

	// Propagate the desired register values from the end of the block to the beginning.
	for _, instr := range a.copies {
		defState := s.getVRegState(instr.dstID)
		desired := defState.desiredLoc.realReg()
		useState := instr.src
		if useState.phiBlk() != succ && useState.desiredLoc == desiredLocUnspecified {
			useState.desiredLoc = newDesiredLocReg(desired)
			desiredUpdated = append(desiredUpdated, useState)
		}
	}

	pc = 0
	for instr := blk.InstrIteratorBegin(); instr != nilInstr; instr = blk.InstrIteratorNext() {
		if wazevoapi.RegAllocLoggingEnabled {
			fmt.Println(instr)
		}

		var currentUsedSet RegSet
		killSet := a.reals[:0]

		// Gather the set of registers that will be used in the current instruction.
		uses := instr.Uses(&a.vs)
		for _, use := range uses {
			if use.IsRealReg() {
				r := use.RealReg()
				currentUsedSet = currentUsedSet.add(r)
				if a.allocatableSet.has(r) {
					killSet = append(killSet, r)
				}
			} else {
				vs := s.getVRegState(use.ID())
				if r := vs.r; r != RealRegInvalid {
					currentUsedSet = currentUsedSet.add(r)
				}
			}
		}

		for i, use := range uses {
			if !use.IsRealReg() {
				vs := s.getVRegState(use.ID())
				killed := vs.lastUse == pc
				r := vs.r

				if r == RealRegInvalid {
					r = a.findOrSpillAllocatable(s, a.regInfo.AllocatableRegisters[use.RegType()], currentUsedSet,
						// Prefer the desired register if it's available.
						vs.desiredLoc.realReg())
					vs.recordReload(f, blk)
					f.ReloadRegisterBefore(use.SetRealReg(r), instr)
					s.useRealReg(r, vs)
				}
				if wazevoapi.RegAllocLoggingEnabled {
					fmt.Printf("\ttrying to use v%v on %s\n", use.ID(), a.regInfo.RealRegName(r))
				}
				instr.AssignUse(i, use.SetRealReg(r))
				currentUsedSet = currentUsedSet.add(r)
				if killed {
					if wazevoapi.RegAllocLoggingEnabled {
						fmt.Printf("\tkill v%d with %s\n", use.ID(), a.regInfo.RealRegName(r))
					}
					killSet = append(killSet, r)
				}
			}
		}

		isIndirect := instr.IsIndirectCall()
		if instr.IsCall() || isIndirect {
			addr := RealRegInvalid
			if isIndirect {
				addr = a.vs[0].RealReg()
			}
			a.releaseCallerSavedRegs(addr)
		}

		for _, r := range killSet {
			s.releaseRealReg(r)
		}
		a.reals = killSet

		defs := instr.Defs(&a.vs)
		switch len(defs) {
		default:
			// Some instructions define multiple values on real registers.
			// E.g. call instructions (following calling convention) / div instruction on x64 that defines both rax and rdx.
			//
			// Note that currently I assume that such instructions define only the pre colored real registers, not the VRegs
			// that require allocations. If we need to support such case, we need to add the logic to handle it here,
			// though is there any such instruction?
			for _, def := range defs {
				if !def.IsRealReg() {
					panic("BUG: multiple defs should be on real registers")
				}
				r := def.RealReg()
				if s.regsInUse.has(r) {
					s.releaseRealReg(r)
				}
				s.useRealReg(r, s.getVRegState(def.ID()))
			}
		case 0:
		case 1:
			def := defs[0]
			vState := s.getVRegState(def.ID())
			if def.IsRealReg() {
				r := def.RealReg()
				if a.allocatableSet.has(r) {
					if s.regsInUse.has(r) {
						s.releaseRealReg(r)
					}
					s.useRealReg(r, vState)
				}
			} else {
				r := vState.r

				if desired := vState.desiredLoc.realReg(); desired != RealRegInvalid {
					if r != desired {
						if (vState.isPhi && vState.defBlk == succ) ||
							// If this is not a phi and it's already assigned a real reg,
							// this value has multiple definitions, hence we cannot assign the desired register.
							(!s.regsInUse.has(desired) && r == RealRegInvalid) {
							// If the phi value is passed via a real register, we force the value to be in the desired register.
							if wazevoapi.RegAllocLoggingEnabled {
								fmt.Printf("\t\tv%d is phi and desiredReg=%s\n", def.ID(), a.regInfo.RealRegName(desired))
							}
							if r != RealRegInvalid {
								// If the value is already in a different real register, we release it to change the state.
								// Otherwise, multiple registers might have the same values at the end, which results in
								// messing up the merge state reconciliation.
								s.releaseRealReg(r)
							}
							r = desired
							s.releaseRealReg(r)
							s.useRealReg(r, vState)
						}
					}
				}

				// Allocate a new real register if `def` is not currently assigned one.
				// It can happen when multiple instructions define the same VReg (e.g. const loads).
				if r == RealRegInvalid {
					if instr.IsCopy() {
						copySrc := instr.Uses(&a.vs)[0].RealReg()
						if a.allocatableSet.has(copySrc) && !s.regsInUse.has(copySrc) {
							r = copySrc
						}
					}
					if r == RealRegInvalid {
						typ := def.RegType()
						r = a.findOrSpillAllocatable(s, a.regInfo.AllocatableRegisters[typ], RegSet(0), RealRegInvalid)
					}
					s.useRealReg(r, vState)
				}
				dr := def.SetRealReg(r)
				instr.AssignDef(dr)
				if wazevoapi.RegAllocLoggingEnabled {
					fmt.Printf("\tdefining v%d with %s\n", def.ID(), a.regInfo.RealRegName(r))
				}
				if vState.isPhi {
					if vState.desiredLoc.stack() { // Stack based phi value.
						f.StoreRegisterAfter(dr, instr)
						// Release the real register as it's not used anymore.
						s.releaseRealReg(r)
					} else {
						// Only the register based phis are necessary to track the defining instructions
						// since the stack-based phis are already having stores inserted ^.
						n := a.phiDefInstListPool.Allocate()
						n.instr = instr
						n.next = vState.phiDefInstList
						n.v = dr
						vState.phiDefInstList = n
					}
				} else {
					vState.defInstr = instr
					vState.defBlk = blk
				}
			}
		}
		if wazevoapi.RegAllocLoggingEnabled {
			fmt.Println(instr)
		}
		pc++
	}

	s.regsInUse.range_(func(allocated RealReg, v *vrState[I, B, F]) { currentBlkState.endRegs.add(allocated, v) })

	currentBlkState.visited = true
	if wazevoapi.RegAllocLoggingEnabled {
		currentBlkState.dump(a.regInfo)
	}

	// Reset the desired end location.
	for _, vs := range desiredUpdated {
		vs.desiredLoc = desiredLocUnspecified
	}
	a.ss = desiredUpdated[:0]

	for i := 0; i < blk.Succs(); i++ {
		succ := f.Succ(blk, i)
		if succ == nilBlk {
			continue
		}
		// If the successor is not visited yet, finalize the start state.
		a.finalizeStartReg(f, succ)
	}
}

func (a *Allocator[I, B, F]) releaseCallerSavedRegs(addrReg RealReg) {
	s := &a.state

	for allocated := RealReg(0); allocated < 64; allocated++ {
		if allocated == addrReg { // If this is the call indirect, we should not touch the addr register.
			continue
		}
		if vs := s.regsInUse.get(allocated); vs != nil {
			if vs.v.IsRealReg() {
				continue // This is the argument register as it's already used by VReg backed by the corresponding RealReg.
			}
			if !a.regInfo.CallerSavedRegisters.has(allocated) {
				// If this is not a caller-saved register, it is safe to keep it across the call.
				continue
			}
			s.releaseRealReg(allocated)
		}
	}
}

func (a *Allocator[I, B, F]) fixMergeState(f F, blk B) {
	preds := blk.Preds()
	if preds <= 1 {
		return
	}

	s := &a.state

	// Restores the state at the beginning of the block.
	bID := blk.ID()
	blkSt := a.getOrAllocateBlockState(bID)
	desiredOccupants := &blkSt.startRegs
	var desiredOccupantsSet RegSet
	for i, v := range desiredOccupants {
		if v != nil {
			desiredOccupantsSet = desiredOccupantsSet.add(RealReg(i))
		}
	}

	if wazevoapi.RegAllocLoggingEnabled {
		fmt.Println("fixMergeState", blk.ID(), ":", desiredOccupants.format(a.regInfo))
	}

	s.currentBlockID = bID
	a.updateLiveInVRState(blkSt)

	for i := 0; i < preds; i++ {
		if i == blkSt.startFromPredIndex {
			continue
		}

		pred := f.Pred(blk, i)
		predSt := a.getOrAllocateBlockState(pred.ID())

		s.resetAt(predSt)

		// Finds the free registers if any.
		intTmp, floatTmp := VRegInvalid, VRegInvalid
		if intFree := s.findAllocatable(
			a.regInfo.AllocatableRegisters[RegTypeInt], desiredOccupantsSet,
		); intFree != RealRegInvalid {
			intTmp = FromRealReg(intFree, RegTypeInt)
		}
		if floatFree := s.findAllocatable(
			a.regInfo.AllocatableRegisters[RegTypeFloat], desiredOccupantsSet,
		); floatFree != RealRegInvalid {
			floatTmp = FromRealReg(floatFree, RegTypeFloat)
		}

		for r := RealReg(0); r < 64; r++ {
			desiredVReg := desiredOccupants.get(r)
			if desiredVReg == nil {
				continue
			}

			currentVReg := s.regsInUse.get(r)
			if currentVReg != nil && desiredVReg.v.ID() == currentVReg.v.ID() {
				continue
			}

			typ := desiredVReg.v.RegType()
			var tmpRealReg VReg
			if typ == RegTypeInt {
				tmpRealReg = intTmp
			} else {
				tmpRealReg = floatTmp
			}
			a.reconcileEdge(f, r, pred, currentVReg, desiredVReg, tmpRealReg, typ)
		}
	}
}

// reconcileEdge reconciles the register state between the current block and the predecessor for the real register `r`.
//
//   - currentVReg is the current VReg value that sits on the register `r`. This can be VRegInvalid if the register is not used at the end of the predecessor.
//   - desiredVReg is the desired VReg value that should be on the register `r`.
//   - freeReg is the temporary register that can be used to swap the values, which may or may not be used.
//   - typ is the register type of the `r`.
func (a *Allocator[I, B, F]) reconcileEdge(f F,
	r RealReg,
	pred B,
	currentState, desiredState *vrState[I, B, F],
	freeReg VReg,
	typ RegType,
) {
	desiredVReg := desiredState.v
	currentVReg := VRegInvalid
	if currentState != nil {
		currentVReg = currentState.v
	}
	// There are four cases to consider:
	// 1. currentVReg is valid, but desiredVReg is on the stack.
	// 2. Both currentVReg and desiredVReg are valid.
	// 3. Desired is on a different register than `r` and currentReg is not valid.
	// 4. Desired is on the stack and currentReg is not valid.

	s := &a.state
	if currentVReg.Valid() {
		er := desiredState.r
		if er == RealRegInvalid {
			// Case 1: currentVReg is valid, but desiredVReg is on the stack.
			if wazevoapi.RegAllocLoggingEnabled {
				fmt.Printf("\t\tv%d is desired to be on %s, but currently on the stack\n",
					desiredVReg.ID(), a.regInfo.RealRegName(r),
				)
			}
			// We need to move the current value to the stack, and reload the desired value into the register.
			// TODO: we can do better here.
			f.StoreRegisterBefore(currentVReg.SetRealReg(r), pred.LastInstrForInsertion())
			s.releaseRealReg(r)

			desiredState.recordReload(f, pred)
			f.ReloadRegisterBefore(desiredVReg.SetRealReg(r), pred.LastInstrForInsertion())
			s.useRealReg(r, desiredState)
			return
		} else {
			// Case 2: Both currentVReg and desiredVReg are valid.
			if wazevoapi.RegAllocLoggingEnabled {
				fmt.Printf("\t\tv%d is desired to be on %s, but currently on %s\n",
					desiredVReg.ID(), a.regInfo.RealRegName(r), a.regInfo.RealRegName(er),
				)
			}
			// This case, we need to swap the values between the current and desired values.
			f.SwapBefore(
				currentVReg.SetRealReg(r),
				desiredVReg.SetRealReg(er),
				freeReg,
				pred.LastInstrForInsertion(),
			)
			s.allocatedRegSet = s.allocatedRegSet.add(freeReg.RealReg())
			s.releaseRealReg(r)
			s.releaseRealReg(er)
			s.useRealReg(r, desiredState)
			s.useRealReg(er, currentState)
			if wazevoapi.RegAllocLoggingEnabled {
				fmt.Printf("\t\tv%d previously on %s moved to %s\n", currentVReg.ID(), a.regInfo.RealRegName(r), a.regInfo.RealRegName(er))
			}
		}
	} else {
		if wazevoapi.RegAllocLoggingEnabled {
			fmt.Printf("\t\tv%d is desired to be on %s, current not used\n",
				desiredVReg.ID(), a.regInfo.RealRegName(r),
			)
		}
		if currentReg := desiredState.r; currentReg != RealRegInvalid {
			// Case 3: Desired is on a different register than `r` and currentReg is not valid.
			// We simply need to move the desired value to the register.
			f.InsertMoveBefore(
				FromRealReg(r, typ),
				desiredVReg.SetRealReg(currentReg),
				pred.LastInstrForInsertion(),
			)
			s.releaseRealReg(currentReg)
		} else {
			// Case 4: Both currentVReg and desiredVReg are not valid.
			// We simply need to reload the desired value into the register.
			desiredState.recordReload(f, pred)
			f.ReloadRegisterBefore(desiredVReg.SetRealReg(r), pred.LastInstrForInsertion())
		}
		s.useRealReg(r, desiredState)
	}
}

func (a *Allocator[I, B, F]) scheduleSpills(f F) {
	states := a.state.vrStates
	for i := 0; i <= states.MaxIDEncountered(); i++ {
		vs := states.Get(i)
		if vs == nil {
			continue
		}
		if vs.spilled {
			a.scheduleSpill(f, vs)
		}
	}
}

func (a *Allocator[I, B, F]) scheduleSpill(f F, vs *vrState[I, B, F]) {
	v := vs.v
	// If the value is the phi value, we need to insert a spill after each phi definition.
	if vs.isPhi {
		for defInstr := vs.phiDefInstList; defInstr != nil; defInstr = defInstr.next {
			f.StoreRegisterAfter(defInstr.v, defInstr.instr)
		}
		return
	}

	pos := vs.lca
	definingBlk := vs.defBlk
	r := RealRegInvalid
	var nilBlk B
	if definingBlk == nilBlk {
		panic(fmt.Sprintf("BUG: definingBlk should not be nil for %s. This is likley a bug in backend lowering logic", vs.v.String()))
	}
	if pos == nilBlk {
		panic(fmt.Sprintf("BUG: pos should not be nil for %s. This is likley a bug in backend lowering logic", vs.v.String()))
	}

	if wazevoapi.RegAllocLoggingEnabled {
		fmt.Printf("v%d is spilled in blk%d, lca=blk%d\n", v.ID(), definingBlk.ID(), pos.ID())
	}
	for pos != definingBlk {
		st := a.getOrAllocateBlockState(pos.ID())
		for rr := RealReg(0); rr < 64; rr++ {
			if vs := st.startRegs.get(rr); vs != nil && vs.v == v {
				r = rr
				// Already in the register, so we can place the spill at the beginning of the block.
				break
			}
		}

		if r != RealRegInvalid {
			break
		}

		pos = f.Idom(pos)
	}

	if pos == definingBlk {
		defInstr := vs.defInstr
		defInstr.Defs(&a.vs)
		if wazevoapi.RegAllocLoggingEnabled {
			fmt.Printf("schedule spill v%d after %v\n", v.ID(), defInstr)
		}
		f.StoreRegisterAfter(a.vs[0], defInstr)
	} else {
		// Found an ancestor block that holds the value in the register at the beginning of the block.
		// We need to insert a spill before the last use.
		first := pos.FirstInstr()
		if wazevoapi.RegAllocLoggingEnabled {
			fmt.Printf("schedule spill v%d before %v\n", v.ID(), first)
		}
		f.StoreRegisterAfter(v.SetRealReg(r), first)
	}
}

// Reset resets the allocator's internal state so that it can be reused.
func (a *Allocator[I, B, F]) Reset() {
	a.state.reset()
	a.blockStates.Reset()
	a.phiDefInstListPool.Reset()
	a.vs = a.vs[:0]
}
