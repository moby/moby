package ssa

import (
	"fmt"

	"github.com/tetratelabs/wazero/internal/engine/wazevo/wazevoapi"
)

// RunPasses implements Builder.RunPasses.
//
// The order here matters; some pass depends on the previous ones.
//
// Note that passes suffixed with "Opt" are the optimization passes, meaning that they edit the instructions and blocks
// while the other passes are not, like passEstimateBranchProbabilities does not edit them, but only calculates the additional information.
func (b *builder) RunPasses() {
	b.runPreBlockLayoutPasses()
	b.runBlockLayoutPass()
	b.runPostBlockLayoutPasses()
	b.runFinalizingPasses()
}

func (b *builder) runPreBlockLayoutPasses() {
	passSortSuccessors(b)
	passDeadBlockEliminationOpt(b)
	// The result of passCalculateImmediateDominators will be used by various passes below.
	passCalculateImmediateDominators(b)
	passRedundantPhiEliminationOpt(b)
	passNopInstElimination(b)

	// TODO: implement either conversion of irreducible CFG into reducible one, or irreducible CFG detection where we panic.
	// 	WebAssembly program shouldn't result in irreducible CFG, but we should handle it properly in just in case.
	// 	See FixIrreducible pass in LLVM: https://llvm.org/doxygen/FixIrreducible_8cpp_source.html

	// TODO: implement more optimization passes like:
	// 	block coalescing.
	// 	Copy-propagation.
	// 	Constant folding.
	// 	Common subexpression elimination.
	// 	Arithmetic simplifications.
	// 	and more!

	// passDeadCodeEliminationOpt could be more accurate if we do this after other optimizations.
	passDeadCodeEliminationOpt(b)
	b.donePreBlockLayoutPasses = true
}

func (b *builder) runBlockLayoutPass() {
	if !b.donePreBlockLayoutPasses {
		panic("runBlockLayoutPass must be called after all pre passes are done")
	}
	passLayoutBlocks(b)
	b.doneBlockLayout = true
}

// runPostBlockLayoutPasses runs the post block layout passes. After this point, CFG is somewhat stable,
// but still can be modified before finalizing passes. At this point, critical edges are split by passLayoutBlocks.
func (b *builder) runPostBlockLayoutPasses() {
	if !b.doneBlockLayout {
		panic("runPostBlockLayoutPasses must be called after block layout pass is done")
	}
	// TODO: Do more. e.g. tail duplication, loop unrolling, etc.

	b.donePostBlockLayoutPasses = true
}

// runFinalizingPasses runs the finalizing passes. After this point, CFG should not be modified.
func (b *builder) runFinalizingPasses() {
	if !b.donePostBlockLayoutPasses {
		panic("runFinalizingPasses must be called after post block layout passes are done")
	}
	// Critical edges are split, so we fix the loop nesting forest.
	passBuildLoopNestingForest(b)
	passBuildDominatorTree(b)
	// Now that we know the final placement of the blocks, we can explicitly mark the fallthrough jumps.
	b.markFallthroughJumps()
}

// passDeadBlockEliminationOpt searches the unreachable blocks, and sets the basicBlock.invalid flag true if so.
func passDeadBlockEliminationOpt(b *builder) {
	entryBlk := b.entryBlk()
	b.blkStack = append(b.blkStack, entryBlk)
	for len(b.blkStack) > 0 {
		reachableBlk := b.blkStack[len(b.blkStack)-1]
		b.blkStack = b.blkStack[:len(b.blkStack)-1]
		reachableBlk.visited = 1

		if !reachableBlk.sealed && !reachableBlk.ReturnBlock() {
			panic(fmt.Sprintf("%s is not sealed", reachableBlk))
		}

		if wazevoapi.SSAValidationEnabled {
			reachableBlk.validate(b)
		}

		for _, succ := range reachableBlk.success {
			if succ.visited == 1 {
				continue
			}
			b.blkStack = append(b.blkStack, succ)
		}
	}

	for blk := b.blockIteratorBegin(); blk != nil; blk = b.blockIteratorNext() {
		if blk.visited != 1 {
			blk.invalid = true
		}
		blk.visited = 0
	}
}

// passRedundantPhiEliminationOpt eliminates the redundant PHIs (in our terminology, parameters of a block).
// This requires the reverse post-order traversal to be calculated before calling this function,
// hence passCalculateImmediateDominators must be called before this.
func passRedundantPhiEliminationOpt(b *builder) {
	redundantParams := b.redundantParams[:0] // reuse the slice from previous iterations.

	// TODO: this might be costly for large programs, but at least, as far as I did the experiment, it's almost the
	//  same as the single iteration version in terms of the overall compilation time. That *might be* mostly thanks to the fact
	//  that removing many PHIs results in the reduction of the total instructions, not because of this indefinite iteration is
	//  relatively small. For example, sqlite speedtest binary results in the large number of redundant PHIs,
	//  the maximum number of iteration was 22, which seems to be acceptable but not that small either since the
	//  complexity here is O(BlockNum * Iterations) at the worst case where BlockNum might be the order of thousands.
	//  -- Note --
	// 	Currently, each iteration can run in any order of blocks, but it empirically converges quickly in practice when
	// 	running on the reverse post-order. It might be possible to optimize this further by using the dominator tree.
	for {
		changed := false
		_ = b.blockIteratorReversePostOrderBegin() // skip entry block!
		// Below, we intentionally use the named iteration variable name, as this comes with inevitable nested for loops!
		for blk := b.blockIteratorReversePostOrderNext(); blk != nil; blk = b.blockIteratorReversePostOrderNext() {
			params := blk.params.View()
			paramNum := len(params)

			for paramIndex := 0; paramIndex < paramNum; paramIndex++ {
				phiValue := params[paramIndex]
				redundant := true

				nonSelfReferencingValue := ValueInvalid
				for predIndex := range blk.preds {
					br := blk.preds[predIndex].branch
					// Resolve the alias in the arguments so that we could use the previous iteration's result.
					b.resolveArgumentAlias(br)
					pred := br.vs.View()[paramIndex]
					if pred == phiValue {
						// This is self-referencing: PHI from the same PHI.
						continue
					}

					if !nonSelfReferencingValue.Valid() {
						nonSelfReferencingValue = pred
						continue
					}

					if nonSelfReferencingValue != pred {
						redundant = false
						break
					}
				}

				if !nonSelfReferencingValue.Valid() {
					// This shouldn't happen, and must be a bug in builder.go.
					panic("BUG: params added but only self-referencing")
				}

				if redundant {
					redundantParams = append(redundantParams, redundantParam{
						index: paramIndex, uniqueValue: nonSelfReferencingValue,
					})
				}
			}

			if len(redundantParams) == 0 {
				continue
			}
			changed = true

			// Remove the redundant PHIs from the argument list of branching instructions.
			for predIndex := range blk.preds {
				redundantParamsCur, predParamCur := 0, 0
				predBlk := blk.preds[predIndex]
				branchInst := predBlk.branch
				view := branchInst.vs.View()
				for argIndex, value := range view {
					if len(redundantParams) == redundantParamsCur ||
						redundantParams[redundantParamsCur].index != argIndex {
						view[predParamCur] = value
						predParamCur++
					} else {
						redundantParamsCur++
					}
				}
				branchInst.vs.Cut(predParamCur)
			}

			// Still need to have the definition of the value of the PHI (previously as the parameter).
			for i := range redundantParams {
				redundantValue := &redundantParams[i]
				phiValue := params[redundantValue.index]
				// Create an alias in this block from the only phi argument to the phi value.
				b.alias(phiValue, redundantValue.uniqueValue)
			}

			// Finally, Remove the param from the blk.
			paramsCur, redundantParamsCur := 0, 0
			for paramIndex := 0; paramIndex < paramNum; paramIndex++ {
				param := params[paramIndex]
				if len(redundantParams) == redundantParamsCur || redundantParams[redundantParamsCur].index != paramIndex {
					params[paramsCur] = param
					paramsCur++
				} else {
					redundantParamsCur++
				}
			}
			blk.params.Cut(paramsCur)

			// Clears the map for the next iteration.
			redundantParams = redundantParams[:0]
		}

		if !changed {
			break
		}
	}

	// Reuse the slice for the future passes.
	b.redundantParams = redundantParams
}

// passDeadCodeEliminationOpt traverses all the instructions, and calculates the reference count of each Value, and
// eliminates all the unnecessary instructions whose ref count is zero.
// The results are stored at builder.valueRefCounts. This also assigns a InstructionGroupID to each Instruction
// during the process. This is the last SSA-level optimization pass and after this,
// the SSA function is ready to be used by backends.
//
// TODO: the algorithm here might not be efficient. Get back to this later.
func passDeadCodeEliminationOpt(b *builder) {
	nvid := int(b.nextValueID)
	if nvid >= len(b.valuesInfo) {
		l := nvid - len(b.valuesInfo) + 1
		b.valuesInfo = append(b.valuesInfo, make([]ValueInfo, l)...)
		view := b.valuesInfo[len(b.valuesInfo)-l:]
		for i := range view {
			view[i].alias = ValueInvalid
		}
	}

	// First, we gather all the instructions with side effects.
	liveInstructions := b.instStack[:0]
	// During the process, we will assign InstructionGroupID to each instruction, which is not
	// relevant to dead code elimination, but we need in the backend.
	var gid InstructionGroupID
	for blk := b.blockIteratorBegin(); blk != nil; blk = b.blockIteratorNext() {
		for cur := blk.rootInstr; cur != nil; cur = cur.next {
			cur.gid = gid
			switch cur.sideEffect() {
			case sideEffectTraps:
				// The trappable should always be alive.
				liveInstructions = append(liveInstructions, cur)
			case sideEffectStrict:
				liveInstructions = append(liveInstructions, cur)
				// The strict side effect should create different instruction groups.
				gid++
			}
		}
	}

	// Find all the instructions referenced by live instructions transitively.
	for len(liveInstructions) > 0 {
		tail := len(liveInstructions) - 1
		live := liveInstructions[tail]
		liveInstructions = liveInstructions[:tail]
		if live.live {
			// If it's already marked alive, this is referenced multiple times,
			// so we can skip it.
			continue
		}
		live.live = true

		// Before we walk, we need to resolve the alias first.
		b.resolveArgumentAlias(live)

		v1, v2, v3, vs := live.Args()
		if v1.Valid() {
			producingInst := b.InstructionOfValue(v1)
			if producingInst != nil {
				liveInstructions = append(liveInstructions, producingInst)
			}
		}

		if v2.Valid() {
			producingInst := b.InstructionOfValue(v2)
			if producingInst != nil {
				liveInstructions = append(liveInstructions, producingInst)
			}
		}

		if v3.Valid() {
			producingInst := b.InstructionOfValue(v3)
			if producingInst != nil {
				liveInstructions = append(liveInstructions, producingInst)
			}
		}

		for _, v := range vs {
			producingInst := b.InstructionOfValue(v)
			if producingInst != nil {
				liveInstructions = append(liveInstructions, producingInst)
			}
		}
	}

	// Now that all the live instructions are flagged as live=true, we eliminate all dead instructions.
	for blk := b.blockIteratorBegin(); blk != nil; blk = b.blockIteratorNext() {
		for cur := blk.rootInstr; cur != nil; cur = cur.next {
			if !cur.live {
				// Remove the instruction from the list.
				if prev := cur.prev; prev != nil {
					prev.next = cur.next
				} else {
					blk.rootInstr = cur.next
				}
				if next := cur.next; next != nil {
					next.prev = cur.prev
				}
				continue
			}

			// If the value alive, we can be sure that arguments are used definitely.
			// Hence, we can increment the value reference counts.
			v1, v2, v3, vs := cur.Args()
			if v1.Valid() {
				b.incRefCount(v1.ID(), cur)
			}
			if v2.Valid() {
				b.incRefCount(v2.ID(), cur)
			}
			if v3.Valid() {
				b.incRefCount(v3.ID(), cur)
			}
			for _, v := range vs {
				b.incRefCount(v.ID(), cur)
			}
		}
	}

	b.instStack = liveInstructions // we reuse the stack for the next iteration.
}

func (b *builder) incRefCount(id ValueID, from *Instruction) {
	if wazevoapi.SSALoggingEnabled {
		fmt.Printf("v%d referenced from %v\n", id, from.Format(b))
	}
	info := &b.valuesInfo[id]
	info.RefCount++
}

// passNopInstElimination eliminates the instructions which is essentially a no-op.
func passNopInstElimination(b *builder) {
	for blk := b.blockIteratorBegin(); blk != nil; blk = b.blockIteratorNext() {
		for cur := blk.rootInstr; cur != nil; cur = cur.next {
			switch cur.Opcode() {
			// TODO: add more logics here.
			case OpcodeIshl, OpcodeSshr, OpcodeUshr:
				x, amount := cur.Arg2()
				definingInst := b.InstructionOfValue(amount)
				if definingInst == nil {
					// If there's no defining instruction, that means the amount is coming from the parameter.
					continue
				}
				if definingInst.Constant() {
					v := definingInst.ConstantVal()

					if x.Type().Bits() == 64 {
						v = v % 64
					} else {
						v = v % 32
					}
					if v == 0 {
						b.alias(cur.Return(), x)
					}
				}
			}
		}
	}
}

// passSortSuccessors sorts the successors of each block in the natural program order.
func passSortSuccessors(b *builder) {
	for i := 0; i < b.basicBlocksPool.Allocated(); i++ {
		blk := b.basicBlocksPool.View(i)
		sortBlocks(blk.success)
	}
}
