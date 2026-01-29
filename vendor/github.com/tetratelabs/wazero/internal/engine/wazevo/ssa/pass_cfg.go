package ssa

import (
	"fmt"
	"math"
	"strings"

	"github.com/tetratelabs/wazero/internal/engine/wazevo/wazevoapi"
)

// passCalculateImmediateDominators calculates immediate dominators for each basic block.
// The result is stored in b.dominators. This make it possible for the following passes to
// use builder.isDominatedBy to check if a block is dominated by another block.
//
// At the last of pass, this function also does the loop detection and sets the basicBlock.loop flag.
func passCalculateImmediateDominators(b *builder) {
	reversePostOrder := b.reversePostOrderedBasicBlocks[:0]

	// Store the reverse postorder from the entrypoint into reversePostOrder slice.
	// This calculation of reverse postorder is not described in the paper,
	// so we use heuristic to calculate it so that we could potentially handle arbitrary
	// complex CFGs under the assumption that success is sorted in program's natural order.
	// That means blk.success[i] always appears before blk.success[i+1] in the source program,
	// which is a reasonable assumption as long as SSA Builder is properly used.
	//
	// First we push blocks in postorder iteratively visit successors of the entry block.
	entryBlk := b.entryBlk()
	exploreStack := append(b.blkStack[:0], entryBlk)
	// These flags are used to track the state of the block in the DFS traversal.
	// We temporarily use the reversePostOrder field to store the state.
	const visitStateUnseen, visitStateSeen, visitStateDone = 0, 1, 2
	entryBlk.visited = visitStateSeen
	for len(exploreStack) > 0 {
		tail := len(exploreStack) - 1
		blk := exploreStack[tail]
		exploreStack = exploreStack[:tail]
		switch blk.visited {
		case visitStateUnseen:
			// This is likely a bug in the frontend.
			panic("BUG: unsupported CFG")
		case visitStateSeen:
			// This is the first time to pop this block, and we have to see the successors first.
			// So push this block again to the stack.
			exploreStack = append(exploreStack, blk)
			// And push the successors to the stack if necessary.
			for _, succ := range blk.success {
				if succ.ReturnBlock() || succ.invalid {
					continue
				}
				if succ.visited == visitStateUnseen {
					succ.visited = visitStateSeen
					exploreStack = append(exploreStack, succ)
				}
			}
			// Finally, we could pop this block once we pop all of its successors.
			blk.visited = visitStateDone
		case visitStateDone:
			// Note: at this point we push blk in postorder despite its name.
			reversePostOrder = append(reversePostOrder, blk)
		default:
			panic("BUG")
		}
	}
	// At this point, reversePostOrder has postorder actually, so we reverse it.
	for i := len(reversePostOrder)/2 - 1; i >= 0; i-- {
		j := len(reversePostOrder) - 1 - i
		reversePostOrder[i], reversePostOrder[j] = reversePostOrder[j], reversePostOrder[i]
	}

	for i, blk := range reversePostOrder {
		blk.reversePostOrder = int32(i)
	}

	// Reuse the dominators slice if possible from the previous computation of function.
	b.dominators = b.dominators[:cap(b.dominators)]
	if len(b.dominators) < b.basicBlocksPool.Allocated() {
		// Generously reserve space in the slice because the slice will be reused future allocation.
		b.dominators = append(b.dominators, make([]*basicBlock, b.basicBlocksPool.Allocated())...)
	}
	calculateDominators(reversePostOrder, b.dominators)

	// Reuse the slices for the future use.
	b.blkStack = exploreStack

	// For the following passes.
	b.reversePostOrderedBasicBlocks = reversePostOrder

	// Ready to detect loops!
	subPassLoopDetection(b)
}

// calculateDominators calculates the immediate dominator of each node in the CFG, and store the result in `doms`.
// The algorithm is based on the one described in the paper "A Simple, Fast Dominance Algorithm"
// https://www.cs.rice.edu/~keith/EMBED/dom.pdf which is a faster/simple alternative to the well known Lengauer-Tarjan algorithm.
//
// The following code almost matches the pseudocode in the paper with one exception (see the code comment below).
//
// The result slice `doms` must be pre-allocated with the size larger than the size of dfsBlocks.
func calculateDominators(reversePostOrderedBlks []*basicBlock, doms []*basicBlock) {
	entry, reversePostOrderedBlks := reversePostOrderedBlks[0], reversePostOrderedBlks[1: /* skips entry point */]
	for _, blk := range reversePostOrderedBlks {
		doms[blk.id] = nil
	}
	doms[entry.id] = entry

	changed := true
	for changed {
		changed = false
		for _, blk := range reversePostOrderedBlks {
			var u *basicBlock
			for i := range blk.preds {
				pred := blk.preds[i].blk
				// Skip if this pred is not reachable yet. Note that this is not described in the paper,
				// but it is necessary to handle nested loops etc.
				if doms[pred.id] == nil {
					continue
				}

				if u == nil {
					u = pred
					continue
				} else {
					u = intersect(doms, u, pred)
				}
			}
			if doms[blk.id] != u {
				doms[blk.id] = u
				changed = true
			}
		}
	}
}

// intersect returns the common dominator of blk1 and blk2.
//
// This is the `intersect` function in the paper.
func intersect(doms []*basicBlock, blk1 *basicBlock, blk2 *basicBlock) *basicBlock {
	finger1, finger2 := blk1, blk2
	for finger1 != finger2 {
		// Move the 'finger1' upwards to its immediate dominator.
		for finger1.reversePostOrder > finger2.reversePostOrder {
			finger1 = doms[finger1.id]
		}
		// Move the 'finger2' upwards to its immediate dominator.
		for finger2.reversePostOrder > finger1.reversePostOrder {
			finger2 = doms[finger2.id]
		}
	}
	return finger1
}

// subPassLoopDetection detects loops in the function using the immediate dominators.
//
// This is run at the last of passCalculateImmediateDominators.
func subPassLoopDetection(b *builder) {
	for blk := b.blockIteratorBegin(); blk != nil; blk = b.blockIteratorNext() {
		for i := range blk.preds {
			pred := blk.preds[i].blk
			if pred.invalid {
				continue
			}
			if b.isDominatedBy(pred, blk) {
				blk.loopHeader = true
			}
		}
	}
}

// buildLoopNestingForest builds the loop nesting forest for the function.
// This must be called after branch splitting since it relies on the CFG.
func passBuildLoopNestingForest(b *builder) {
	ent := b.entryBlk()
	doms := b.dominators
	for _, blk := range b.reversePostOrderedBasicBlocks {
		n := doms[blk.id]
		for !n.loopHeader && n != ent {
			n = doms[n.id]
		}

		if n == ent && blk.loopHeader {
			b.loopNestingForestRoots = append(b.loopNestingForestRoots, blk)
		} else if n == ent {
		} else if n.loopHeader {
			n.loopNestingForestChildren = n.loopNestingForestChildren.Append(&b.varLengthBasicBlockPool, blk)
		}
	}

	if wazevoapi.SSALoggingEnabled {
		for _, root := range b.loopNestingForestRoots {
			printLoopNestingForest(root.(*basicBlock), 0)
		}
	}
}

func printLoopNestingForest(root *basicBlock, depth int) {
	fmt.Println(strings.Repeat("\t", depth), "loop nesting forest root:", root.ID())
	for _, child := range root.loopNestingForestChildren.View() {
		fmt.Println(strings.Repeat("\t", depth+1), "child:", child.ID())
		if child.LoopHeader() {
			printLoopNestingForest(child.(*basicBlock), depth+2)
		}
	}
}

type dominatorSparseTree struct {
	time         int32
	euler        []*basicBlock
	first, depth []int32
	table        [][]int32
}

// passBuildDominatorTree builds the dominator tree for the function, and constructs builder.sparseTree.
func passBuildDominatorTree(b *builder) {
	// First we materialize the children of each node in the dominator tree.
	idoms := b.dominators
	for _, blk := range b.reversePostOrderedBasicBlocks {
		parent := idoms[blk.id]
		if parent == nil {
			panic("BUG")
		} else if parent == blk {
			// This is the entry block.
			continue
		}
		if prev := parent.child; prev == nil {
			parent.child = blk
		} else {
			parent.child = blk
			blk.sibling = prev
		}
	}

	// Reset the state from the previous computation.
	n := b.basicBlocksPool.Allocated()
	st := &b.sparseTree
	st.euler = append(st.euler[:0], make([]*basicBlock, 2*n-1)...)
	st.first = append(st.first[:0], make([]int32, n)...)
	for i := range st.first {
		st.first[i] = -1
	}
	st.depth = append(st.depth[:0], make([]int32, 2*n-1)...)
	st.time = 0

	// Start building the sparse tree.
	st.eulerTour(b.entryBlk(), 0)
	st.buildSparseTable()
}

func (dt *dominatorSparseTree) eulerTour(node *basicBlock, height int32) {
	if wazevoapi.SSALoggingEnabled {
		fmt.Println(strings.Repeat("\t", int(height)), "euler tour:", node.ID())
	}
	dt.euler[dt.time] = node
	dt.depth[dt.time] = height
	if dt.first[node.id] == -1 {
		dt.first[node.id] = dt.time
	}
	dt.time++

	for child := node.child; child != nil; child = child.sibling {
		dt.eulerTour(child, height+1)
		dt.euler[dt.time] = node // add the current node again after visiting a child
		dt.depth[dt.time] = height
		dt.time++
	}
}

// buildSparseTable builds a sparse table for RMQ queries.
func (dt *dominatorSparseTree) buildSparseTable() {
	n := len(dt.depth)
	k := int(math.Log2(float64(n))) + 1
	table := dt.table

	if n >= len(table) {
		table = append(table, make([][]int32, n-len(table)+1)...)
	}
	for i := range table {
		if len(table[i]) < k {
			table[i] = append(table[i], make([]int32, k-len(table[i]))...)
		}
		table[i][0] = int32(i)
	}

	for j := 1; 1<<j <= n; j++ {
		for i := 0; i+(1<<j)-1 < n; i++ {
			if dt.depth[table[i][j-1]] < dt.depth[table[i+(1<<(j-1))][j-1]] {
				table[i][j] = table[i][j-1]
			} else {
				table[i][j] = table[i+(1<<(j-1))][j-1]
			}
		}
	}
	dt.table = table
}

// rmq performs a range minimum query on the sparse table.
func (dt *dominatorSparseTree) rmq(l, r int32) int32 {
	table := dt.table
	depth := dt.depth
	j := int(math.Log2(float64(r - l + 1)))
	if depth[table[l][j]] <= depth[table[r-(1<<j)+1][j]] {
		return table[l][j]
	}
	return table[r-(1<<j)+1][j]
}

// findLCA finds the LCA using the Euler tour and RMQ.
func (dt *dominatorSparseTree) findLCA(u, v BasicBlockID) *basicBlock {
	first := dt.first
	if first[u] > first[v] {
		u, v = v, u
	}
	return dt.euler[dt.rmq(first[u], first[v])]
}
