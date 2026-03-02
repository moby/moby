package regalloc

import "fmt"

// These interfaces are implemented by ISA-specific backends to abstract away the details, and allow the register
// allocators to work on any ISA.

type (
	// Function is the top-level interface to do register allocation, which corresponds to a CFG containing
	// Blocks(s).
	//
	// I is the type of the instruction, and B is the type of the basic block.
	Function[I Instr, B Block[I]] interface {
		// PostOrderBlockIteratorBegin returns the first block in the post-order traversal of the CFG.
		// In other words, the last blocks in the CFG will be returned first.
		PostOrderBlockIteratorBegin() B
		// PostOrderBlockIteratorNext returns the next block in the post-order traversal of the CFG.
		PostOrderBlockIteratorNext() B
		// ReversePostOrderBlockIteratorBegin returns the first block in the reverse post-order traversal of the CFG.
		// In other words, the first blocks in the CFG will be returned first.
		ReversePostOrderBlockIteratorBegin() B
		// ReversePostOrderBlockIteratorNext returns the next block in the reverse post-order traversal of the CFG.
		ReversePostOrderBlockIteratorNext() B
		// ClobberedRegisters tell the clobbered registers by this function.
		ClobberedRegisters([]VReg)
		// LoopNestingForestRoots returns the number of roots of the loop nesting forest in a function.
		LoopNestingForestRoots() int
		// LoopNestingForestRoot returns the i-th root of the loop nesting forest in a function.
		LoopNestingForestRoot(i int) B
		// LowestCommonAncestor returns the lowest common ancestor of two blocks in the dominator tree.
		LowestCommonAncestor(blk1, blk2 B) B
		// Idom returns the immediate dominator of the given block.
		Idom(blk B) B

		// LoopNestingForestChild returns the i-th child of the block in the loop nesting forest.
		LoopNestingForestChild(b B, i int) B
		// Pred returns the i-th predecessor of the block in the CFG.
		Pred(b B, i int) B
		// Succ returns the i-th successor of the block in the CFG.
		Succ(b B, i int) B
		// BlockParams returns the virtual registers used as the parameters of this block.
		BlockParams(B, *[]VReg) []VReg

		// Followings are for rewriting the function.

		// SwapBefore swaps the two virtual registers at the end of the given block.
		SwapBefore(x1, x2, tmp VReg, instr I)
		// StoreRegisterBefore inserts store instruction(s) before the given instruction for the given virtual register.
		StoreRegisterBefore(v VReg, instr I)
		// StoreRegisterAfter inserts store instruction(s) after the given instruction for the given virtual register.
		StoreRegisterAfter(v VReg, instr I)
		// ReloadRegisterBefore inserts reload instruction(s) before the given instruction for the given virtual register.
		ReloadRegisterBefore(v VReg, instr I)
		// ReloadRegisterAfter inserts reload instruction(s) after the given instruction for the given virtual register.
		ReloadRegisterAfter(v VReg, instr I)
		// InsertMoveBefore inserts move instruction(s) before the given instruction for the given virtual registers.
		InsertMoveBefore(dst, src VReg, instr I)
	}

	// Block is a basic block in the CFG of a function, and it consists of multiple instructions, and predecessor Block(s).
	// Right now, this corresponds to a ssa.BasicBlock lowered to the machine level.
	Block[I Instr] interface {
		comparable
		// ID returns the unique identifier of this block which is ordered in the reverse post-order traversal of the CFG.
		ID() int32
		// InstrIteratorBegin returns the first instruction in this block. Instructions added after lowering must be skipped.
		// Note: multiple Instr(s) will not be held at the same time, so it's safe to use the same impl for the return Instr.
		InstrIteratorBegin() I
		// InstrIteratorNext returns the next instruction in this block. Instructions added after lowering must be skipped.
		// Note: multiple Instr(s) will not be held at the same time, so it's safe to use the same impl for the return Instr.
		InstrIteratorNext() I
		// InstrRevIteratorBegin is the same as InstrIteratorBegin, but in the reverse order.
		InstrRevIteratorBegin() I
		// InstrRevIteratorNext is the same as InstrIteratorNext, but in the reverse order.
		InstrRevIteratorNext() I
		// FirstInstr returns the fist instruction in this block where instructions will be inserted after it.
		FirstInstr() I
		// LastInstrForInsertion returns the last instruction in this block where instructions will be inserted before it.
		// Such insertions only happen when we need to insert spill/reload instructions to adjust the merge edges.
		// At the time of register allocation, all the critical edges are already split, so there is no need
		// to worry about the case where branching instruction has multiple successors.
		// Therefore, usually, it is the nop instruction, but if the block ends with an unconditional branching, then it returns
		// the unconditional branch, not the nop. In other words it is either nop or unconditional branch.
		LastInstrForInsertion() I
		// Preds returns the number of predecessors of this block in the CFG.
		Preds() int
		// Entry returns true if the block is for the entry block.
		Entry() bool
		// Succs returns the number of successors of this block in the CFG.
		Succs() int
		// LoopHeader returns true if this block is a loop header.
		LoopHeader() bool
		// LoopNestingForestChildren returns the number of children of this block in the loop nesting forest.
		LoopNestingForestChildren() int
	}

	// Instr is an instruction in a block, abstracting away the underlying ISA.
	Instr interface {
		comparable
		fmt.Stringer
		// Defs returns the virtual registers defined by this instruction.
		Defs(*[]VReg) []VReg
		// Uses returns the virtual registers used by this instruction.
		// Note: multiple returned []VReg will not be held at the same time, so it's safe to use the same slice for this.
		Uses(*[]VReg) []VReg
		// AssignUse assigns the RealReg-allocated virtual register used by this instruction at the given index.
		AssignUse(index int, v VReg)
		// AssignDef assigns a RealReg-allocated virtual register defined by this instruction.
		// This only accepts one register because we don't allocate registers for multi-def instructions (i.e. call instruction)
		AssignDef(VReg)
		// IsCopy returns true if this instruction is a move instruction between two registers.
		// If true, the instruction is of the form of dst = src, and if the src and dst do not interfere with each other,
		// we could coalesce them, and hence the copy can be eliminated from the final code.
		IsCopy() bool
		// IsCall returns true if this instruction is a call instruction. The result is used to insert
		// caller saved register spills and restores.
		IsCall() bool
		// IsIndirectCall returns true if this instruction is an indirect call instruction which calls a function pointer.
		//  The result is used to insert caller saved register spills and restores.
		IsIndirectCall() bool
		// IsReturn returns true if this instruction is a return instruction.
		IsReturn() bool
	}
)
