package backend

import (
	"github.com/tetratelabs/wazero/internal/engine/wazevo/backend/regalloc"
	"github.com/tetratelabs/wazero/internal/engine/wazevo/ssa"
)

// Lower implements Compiler.Lower.
func (c *compiler) Lower() {
	c.assignVirtualRegisters()
	c.mach.SetCurrentABI(c.GetFunctionABI(c.ssaBuilder.Signature()))
	c.mach.StartLoweringFunction(c.ssaBuilder.BlockIDMax())
	c.lowerBlocks()
}

// lowerBlocks lowers each block in the ssa.Builder.
func (c *compiler) lowerBlocks() {
	builder := c.ssaBuilder
	for blk := builder.BlockIteratorReversePostOrderBegin(); blk != nil; blk = builder.BlockIteratorReversePostOrderNext() {
		c.lowerBlock(blk)
	}

	// After lowering all blocks, we need to link adjacent blocks to layout one single instruction list.
	var prev ssa.BasicBlock
	for next := builder.BlockIteratorReversePostOrderBegin(); next != nil; next = builder.BlockIteratorReversePostOrderNext() {
		if prev != nil {
			c.mach.LinkAdjacentBlocks(prev, next)
		}
		prev = next
	}
}

func (c *compiler) lowerBlock(blk ssa.BasicBlock) {
	mach := c.mach
	mach.StartBlock(blk)

	// We traverse the instructions in reverse order because we might want to lower multiple
	// instructions together.
	cur := blk.Tail()

	// First gather the branching instructions at the end of the blocks.
	var br0, br1 *ssa.Instruction
	if cur.IsBranching() {
		br0 = cur
		cur = cur.Prev()
		if cur != nil && cur.IsBranching() {
			br1 = cur
			cur = cur.Prev()
		}
	}

	if br0 != nil {
		c.lowerBranches(br0, br1)
	}

	if br1 != nil && br0 == nil {
		panic("BUG? when a block has conditional branch but doesn't end with an unconditional branch?")
	}

	// Now start lowering the non-branching instructions.
	for ; cur != nil; cur = cur.Prev() {
		c.setCurrentGroupID(cur.GroupID())
		if cur.Lowered() {
			continue
		}

		switch cur.Opcode() {
		case ssa.OpcodeReturn:
			rets := cur.ReturnVals()
			if len(rets) > 0 {
				c.mach.LowerReturns(rets)
			}
			c.mach.InsertReturn()
		default:
			mach.LowerInstr(cur)
		}
		mach.FlushPendingInstructions()
	}

	// Finally, if this is the entry block, we have to insert copies of arguments from the real location to the VReg.
	if blk.EntryBlock() {
		c.lowerFunctionArguments(blk)
	}

	mach.EndBlock()
}

// lowerBranches is called right after StartBlock and before any LowerInstr call if
// there are branches to the given block. br0 is the very end of the block and b1 is the before the br0 if it exists.
// At least br0 is not nil, but br1 can be nil if there's no branching before br0.
//
// See ssa.Instruction IsBranching, and the comment on ssa.BasicBlock.
func (c *compiler) lowerBranches(br0, br1 *ssa.Instruction) {
	mach := c.mach

	c.setCurrentGroupID(br0.GroupID())
	c.mach.LowerSingleBranch(br0)
	mach.FlushPendingInstructions()
	if br1 != nil {
		c.setCurrentGroupID(br1.GroupID())
		c.mach.LowerConditionalBranch(br1)
		mach.FlushPendingInstructions()
	}

	if br0.Opcode() == ssa.OpcodeJump {
		_, args, targetBlockID := br0.BranchData()
		argExists := len(args) != 0
		if argExists && br1 != nil {
			panic("BUG: critical edge split failed")
		}
		target := c.ssaBuilder.BasicBlock(targetBlockID)
		if argExists && target.ReturnBlock() {
			if len(args) > 0 {
				c.mach.LowerReturns(args)
			}
		} else if argExists {
			c.lowerBlockArguments(args, target)
		}
	}
	mach.FlushPendingInstructions()
}

func (c *compiler) lowerFunctionArguments(entry ssa.BasicBlock) {
	mach := c.mach

	c.tmpVals = c.tmpVals[:0]
	data := c.ssaBuilder.ValuesInfo()
	for i := 0; i < entry.Params(); i++ {
		p := entry.Param(i)
		if data[p.ID()].RefCount > 0 {
			c.tmpVals = append(c.tmpVals, p)
		} else {
			// If the argument is not used, we can just pass an invalid value.
			c.tmpVals = append(c.tmpVals, ssa.ValueInvalid)
		}
	}
	mach.LowerParams(c.tmpVals)
	mach.FlushPendingInstructions()
}

// lowerBlockArguments lowers how to pass arguments to the given successor block.
func (c *compiler) lowerBlockArguments(args []ssa.Value, succ ssa.BasicBlock) {
	if len(args) != succ.Params() {
		panic("BUG: mismatched number of arguments")
	}

	c.varEdges = c.varEdges[:0]
	c.varEdgeTypes = c.varEdgeTypes[:0]
	c.constEdges = c.constEdges[:0]
	for i := 0; i < len(args); i++ {
		dst := succ.Param(i)
		src := args[i]

		dstReg := c.VRegOf(dst)
		srcInstr := c.ssaBuilder.InstructionOfValue(src)
		if srcInstr != nil && srcInstr.Constant() {
			c.constEdges = append(c.constEdges, struct {
				cInst *ssa.Instruction
				dst   regalloc.VReg
			}{cInst: srcInstr, dst: dstReg})
		} else {
			srcReg := c.VRegOf(src)
			// Even when the src=dst, insert the move so that we can keep such registers keep-alive.
			c.varEdges = append(c.varEdges, [2]regalloc.VReg{srcReg, dstReg})
			c.varEdgeTypes = append(c.varEdgeTypes, src.Type())
		}
	}

	// Check if there's an overlap among the dsts and srcs in varEdges.
	c.vRegIDs = c.vRegIDs[:0]
	for _, edge := range c.varEdges {
		src := edge[0].ID()
		if int(src) >= len(c.vRegSet) {
			c.vRegSet = append(c.vRegSet, make([]bool, src+1)...)
		}
		c.vRegSet[src] = true
		c.vRegIDs = append(c.vRegIDs, src)
	}
	separated := true
	for _, edge := range c.varEdges {
		dst := edge[1].ID()
		if int(dst) >= len(c.vRegSet) {
			c.vRegSet = append(c.vRegSet, make([]bool, dst+1)...)
		} else {
			if c.vRegSet[dst] {
				separated = false
				break
			}
		}
	}
	for _, id := range c.vRegIDs {
		c.vRegSet[id] = false // reset for the next use.
	}

	if separated {
		// If there's no overlap, we can simply move the source to destination.
		for i, edge := range c.varEdges {
			src, dst := edge[0], edge[1]
			c.mach.InsertMove(dst, src, c.varEdgeTypes[i])
		}
	} else {
		// Otherwise, we allocate a temporary registers and move the source to the temporary register,
		//
		// First move all of them to temporary registers.
		c.tempRegs = c.tempRegs[:0]
		for i, edge := range c.varEdges {
			src := edge[0]
			typ := c.varEdgeTypes[i]
			temp := c.AllocateVReg(typ)
			c.tempRegs = append(c.tempRegs, temp)
			c.mach.InsertMove(temp, src, typ)
		}
		// Then move the temporary registers to the destination.
		for i, edge := range c.varEdges {
			temp := c.tempRegs[i]
			dst := edge[1]
			c.mach.InsertMove(dst, temp, c.varEdgeTypes[i])
		}
	}

	// Finally, move the constants.
	for _, edge := range c.constEdges {
		cInst, dst := edge.cInst, edge.dst
		c.mach.InsertLoadConstantBlockArg(cInst, dst)
	}
}
