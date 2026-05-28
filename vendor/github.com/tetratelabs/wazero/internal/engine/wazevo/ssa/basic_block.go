package ssa

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tetratelabs/wazero/internal/engine/wazevo/wazevoapi"
)

// BasicBlock represents the Basic Block of an SSA function.
// Each BasicBlock always ends with branching instructions (e.g. Branch, Return, etc.),
// and at most two branches are allowed. If there's two branches, these two are placed together at the end of the block.
// In other words, there's no branching instruction in the middle of the block.
//
// Note: we use the "block argument" variant of SSA, instead of PHI functions. See the package level doc comments.
//
// Note: we use "parameter/param" as a placeholder which represents a variant of PHI, and "argument/arg" as an actual
// Value passed to that "parameter/param".
type BasicBlock interface {
	// ID returns the unique ID of this block.
	ID() BasicBlockID

	// Name returns the unique string ID of this block. e.g. blk0, blk1, ...
	Name() string

	// AddParam adds the parameter to the block whose type specified by `t`.
	AddParam(b Builder, t Type) Value

	// Params returns the number of parameters to this block.
	Params() int

	// Param returns (Variable, Value) which corresponds to the i-th parameter of this block.
	// The returned Value is the definition of the param in this block.
	Param(i int) Value

	// Root returns the root instruction of this block.
	Root() *Instruction

	// Tail returns the tail instruction of this block.
	Tail() *Instruction

	// EntryBlock returns true if this block represents the function entry.
	EntryBlock() bool

	// ReturnBlock returns ture if this block represents the function return.
	ReturnBlock() bool

	// Valid is true if this block is still valid even after optimizations.
	Valid() bool

	// Sealed is true if this block has been sealed.
	Sealed() bool

	// Preds returns the number of predecessors of this block.
	Preds() int

	// Pred returns the i-th predecessor of this block.
	Pred(i int) BasicBlock

	// Succs returns the number of successors of this block.
	Succs() int

	// Succ returns the i-th successor of this block.
	Succ(i int) BasicBlock

	// LoopHeader returns true if this block is a loop header.
	LoopHeader() bool

	// LoopNestingForestChildren returns the children of this block in the loop nesting forest.
	LoopNestingForestChildren() []BasicBlock
}

type (
	// basicBlock is a basic block in a SSA-transformed function.
	basicBlock struct {
		id                      BasicBlockID
		rootInstr, currentInstr *Instruction
		// params are Values that represent parameters to a basicBlock.
		// Each parameter can be considered as an output of PHI instruction in traditional SSA.
		params  Values
		preds   []basicBlockPredecessorInfo
		success []*basicBlock
		// singlePred is the alias to preds[0] for fast lookup, and only set after Seal is called.
		singlePred *basicBlock
		// lastDefinitions maps Variable to its last definition in this block.
		lastDefinitions map[Variable]Value
		// unknownsValues are used in builder.findValue. The usage is well-described in the paper.
		unknownValues []unknownValue
		// invalid is true if this block is made invalid during optimizations.
		invalid bool
		// sealed is true if this is sealed (all the predecessors are known).
		sealed bool
		// loopHeader is true if this block is a loop header:
		//
		// > A loop header (sometimes called the entry point of the loop) is a dominator that is the target
		// > of a loop-forming back edge. The loop header dominates all blocks in the loop body.
		// > A block may be a loop header for more than one loop. A loop may have multiple entry points,
		// > in which case it has no "loop header".
		//
		// See https://en.wikipedia.org/wiki/Control-flow_graph for more details.
		//
		// This is modified during the subPassLoopDetection pass.
		loopHeader bool

		// loopNestingForestChildren holds the children of this block in the loop nesting forest.
		// Non-empty if and only if this block is a loop header (i.e. loopHeader=true)
		loopNestingForestChildren wazevoapi.VarLength[BasicBlock]

		// reversePostOrder is used to sort all the blocks in the function in reverse post order.
		// This is used in builder.LayoutBlocks.
		reversePostOrder int32

		// visited is used during various traversals.
		visited int32

		// child and sibling are the ones in the dominator tree.
		child, sibling *basicBlock
	}
	// BasicBlockID is the unique ID of a basicBlock.
	BasicBlockID uint32

	unknownValue struct {
		// variable is the variable that this unknownValue represents.
		variable Variable
		// value is the value that this unknownValue represents.
		value Value
	}
)

// basicBlockVarLengthNil is the default nil value for basicBlock.loopNestingForestChildren.
var basicBlockVarLengthNil = wazevoapi.NewNilVarLength[BasicBlock]()

const basicBlockIDReturnBlock = 0xffffffff

// Name implements BasicBlock.Name.
func (bb *basicBlock) Name() string {
	if bb.id == basicBlockIDReturnBlock {
		return "blk_ret"
	} else {
		return fmt.Sprintf("blk%d", bb.id)
	}
}

// String implements fmt.Stringer for debugging.
func (bid BasicBlockID) String() string {
	if bid == basicBlockIDReturnBlock {
		return "blk_ret"
	} else {
		return fmt.Sprintf("blk%d", bid)
	}
}

// ID implements BasicBlock.ID.
func (bb *basicBlock) ID() BasicBlockID {
	return bb.id
}

// basicBlockPredecessorInfo is the information of a predecessor of a basicBlock.
// predecessor is determined by a pair of block and the branch instruction used to jump to the successor.
type basicBlockPredecessorInfo struct {
	blk    *basicBlock
	branch *Instruction
}

// EntryBlock implements BasicBlock.EntryBlock.
func (bb *basicBlock) EntryBlock() bool {
	return bb.id == 0
}

// ReturnBlock implements BasicBlock.ReturnBlock.
func (bb *basicBlock) ReturnBlock() bool {
	return bb.id == basicBlockIDReturnBlock
}

// AddParam implements BasicBlock.AddParam.
func (bb *basicBlock) AddParam(b Builder, typ Type) Value {
	paramValue := b.allocateValue(typ)
	bb.params = bb.params.Append(&b.(*builder).varLengthPool, paramValue)
	return paramValue
}

// addParamOn adds a parameter to this block whose value is already allocated.
func (bb *basicBlock) addParamOn(b *builder, value Value) {
	bb.params = bb.params.Append(&b.varLengthPool, value)
}

// Params implements BasicBlock.Params.
func (bb *basicBlock) Params() int {
	return len(bb.params.View())
}

// Param implements BasicBlock.Param.
func (bb *basicBlock) Param(i int) Value {
	return bb.params.View()[i]
}

// Valid implements BasicBlock.Valid.
func (bb *basicBlock) Valid() bool {
	return !bb.invalid
}

// Sealed implements BasicBlock.Sealed.
func (bb *basicBlock) Sealed() bool {
	return bb.sealed
}

// insertInstruction implements BasicBlock.InsertInstruction.
func (bb *basicBlock) insertInstruction(b *builder, next *Instruction) {
	current := bb.currentInstr
	if current != nil {
		current.next = next
		next.prev = current
	} else {
		bb.rootInstr = next
	}
	bb.currentInstr = next

	switch next.opcode {
	case OpcodeJump, OpcodeBrz, OpcodeBrnz:
		target := BasicBlockID(next.rValue)
		b.basicBlock(target).addPred(bb, next)
	case OpcodeBrTable:
		for _, _target := range next.rValues.View() {
			target := BasicBlockID(_target)
			b.basicBlock(target).addPred(bb, next)
		}
	}
}

// NumPreds implements BasicBlock.NumPreds.
func (bb *basicBlock) NumPreds() int {
	return len(bb.preds)
}

// Preds implements BasicBlock.Preds.
func (bb *basicBlock) Preds() int {
	return len(bb.preds)
}

// Pred implements BasicBlock.Pred.
func (bb *basicBlock) Pred(i int) BasicBlock {
	return bb.preds[i].blk
}

// Succs implements BasicBlock.Succs.
func (bb *basicBlock) Succs() int {
	return len(bb.success)
}

// Succ implements BasicBlock.Succ.
func (bb *basicBlock) Succ(i int) BasicBlock {
	return bb.success[i]
}

// Root implements BasicBlock.Root.
func (bb *basicBlock) Root() *Instruction {
	return bb.rootInstr
}

// Tail implements BasicBlock.Tail.
func (bb *basicBlock) Tail() *Instruction {
	return bb.currentInstr
}

// reset resets the basicBlock to its initial state so that it can be reused for another function.
func resetBasicBlock(bb *basicBlock) {
	bb.params = ValuesNil
	bb.rootInstr, bb.currentInstr = nil, nil
	bb.preds = bb.preds[:0]
	bb.success = bb.success[:0]
	bb.invalid, bb.sealed = false, false
	bb.singlePred = nil
	bb.unknownValues = bb.unknownValues[:0]
	bb.lastDefinitions = wazevoapi.ResetMap(bb.lastDefinitions)
	bb.reversePostOrder = -1
	bb.visited = 0
	bb.loopNestingForestChildren = basicBlockVarLengthNil
	bb.loopHeader = false
	bb.sibling = nil
	bb.child = nil
}

// addPred adds a predecessor to this block specified by the branch instruction.
func (bb *basicBlock) addPred(blk BasicBlock, branch *Instruction) {
	if bb.sealed {
		panic("BUG: trying to add predecessor to a sealed block: " + bb.Name())
	}

	pred := blk.(*basicBlock)
	for i := range bb.preds {
		existingPred := &bb.preds[i]
		if existingPred.blk == pred && existingPred.branch != branch {
			// If the target is already added, then this must come from the same BrTable,
			// otherwise such redundant branch should be eliminated by the frontend. (which should be simpler).
			panic(fmt.Sprintf("BUG: redundant non BrTable jumps in %s whose targes are the same", bb.Name()))
		}
	}

	bb.preds = append(bb.preds, basicBlockPredecessorInfo{
		blk:    pred,
		branch: branch,
	})

	pred.success = append(pred.success, bb)
}

// formatHeader returns the string representation of the header of the basicBlock.
func (bb *basicBlock) formatHeader(b Builder) string {
	ps := make([]string, len(bb.params.View()))
	for i, p := range bb.params.View() {
		ps[i] = p.formatWithType(b)
	}

	if len(bb.preds) > 0 {
		preds := make([]string, 0, len(bb.preds))
		for _, pred := range bb.preds {
			if pred.blk.invalid {
				continue
			}
			preds = append(preds, fmt.Sprintf("blk%d", pred.blk.id))

		}
		return fmt.Sprintf("blk%d: (%s) <-- (%s)",
			bb.id, strings.Join(ps, ","), strings.Join(preds, ","))
	} else {
		return fmt.Sprintf("blk%d: (%s)", bb.id, strings.Join(ps, ", "))
	}
}

// validates validates the basicBlock for debugging purpose.
func (bb *basicBlock) validate(b *builder) {
	if bb.invalid {
		panic("BUG: trying to validate an invalid block: " + bb.Name())
	}
	if len(bb.preds) > 0 {
		for _, pred := range bb.preds {
			if pred.branch.opcode != OpcodeBrTable {
				blockID := int(pred.branch.rValue)
				target := b.basicBlocksPool.View(blockID)
				if target != bb {
					panic(fmt.Sprintf("BUG: '%s' is not branch to %s, but to %s",
						pred.branch.Format(b), bb.Name(), target.Name()))
				}
			}

			var exp int
			if bb.ReturnBlock() {
				exp = len(b.currentSignature.Results)
			} else {
				exp = len(bb.params.View())
			}

			if len(pred.branch.vs.View()) != exp {
				panic(fmt.Sprintf(
					"BUG: len(argument at %s) != len(params at %s): %d != %d: %s",
					pred.blk.Name(), bb.Name(),
					len(pred.branch.vs.View()), len(bb.params.View()), pred.branch.Format(b),
				))
			}

		}
	}
}

// String implements fmt.Stringer for debugging purpose only.
func (bb *basicBlock) String() string {
	return strconv.Itoa(int(bb.id))
}

// LoopNestingForestChildren implements BasicBlock.LoopNestingForestChildren.
func (bb *basicBlock) LoopNestingForestChildren() []BasicBlock {
	return bb.loopNestingForestChildren.View()
}

// LoopHeader implements BasicBlock.LoopHeader.
func (bb *basicBlock) LoopHeader() bool {
	return bb.loopHeader
}
