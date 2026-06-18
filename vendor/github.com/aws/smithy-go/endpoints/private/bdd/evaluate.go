package bdd

const resultOffset int32 = 100_000_000
const intsPerNode = 3

// Evaluate traverses a compiled BDD node array and returns the result index.
// nodes is a flat array of [condIdx, hi, lo] triples (1-indexed).
// root is the root node reference. evalCond returns true/false for condition index.
func Evaluate(nodes []int32, root int32, evalCond func(int) bool) int32 {
	ref := root
	for {
		if ref >= resultOffset {
			return ref - resultOffset
		}
		if ref == 1 || ref == -1 {
			return 0 // NoMatchRule
		}

		complement := ref < 0
		nodeIdx := ref
		if complement {
			nodeIdx = -ref
		}
		base := (nodeIdx - 1) * intsPerNode
		condIdx := nodes[base]
		hi := nodes[base+1]
		lo := nodes[base+2]

		if complement != evalCond(int(condIdx)) {
			ref = hi
		} else {
			ref = lo
		}
	}
}
