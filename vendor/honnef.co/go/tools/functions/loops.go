package functions

import "honnef.co/go/tools/ssa"

type Loop map[*ssa.BasicBlock]bool

func findLoops(fn *ssa.Function) []Loop {
	if fn.Blocks == nil {
		return nil
	}
	tree := fn.DomPreorder()
	var sets []Loop
	for _, h := range tree {
		for _, n := range h.Preds {
			if !h.Dominates(n) {
				continue
			}
			// n is a back-edge to h
			// h is the loop header
			if n == h {
				sets = append(sets, Loop{n: true})
				continue
			}
			set := Loop{h: true, n: true}
			for _, b := range allPredsBut(n, h, nil) {
				set[b] = true
			}
			sets = append(sets, set)
		}
	}
	return sets
}

func allPredsBut(b, but *ssa.BasicBlock, list []*ssa.BasicBlock) []*ssa.BasicBlock {
outer:
	for _, pred := range b.Preds {
		if pred == but {
			continue
		}
		for _, p := range list {
			// TODO improve big-o complexity of this function
			if pred == p {
				continue outer
			}
		}
		list = append(list, pred)
		list = allPredsBut(pred, but, list)
	}
	return list
}
