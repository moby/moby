package btf

import (
	"fmt"
	"iter"
)

// Functions to traverse a cyclic graph of types. The below was very useful:
// https://eli.thegreenplace.net/2015/directed-graph-traversal-orderings-and-applications-to-data-flow-analysis/#post-order-and-reverse-post-order

// postorder yields all types reachable from root in post order.
func postorder(root Type, visited map[Type]struct{}) iter.Seq[Type] {
	return func(yield func(Type) bool) {
		visitInPostorder(root, visited, yield)
	}
}

// visitInPostorder is a separate function to avoid arguments escaping
// to the heap. Don't change the setup without re-running the benchmarks.
func visitInPostorder(root Type, visited map[Type]struct{}, yield func(typ Type) bool) bool {
	if _, ok := visited[root]; ok {
		return true
	}
	if visited == nil {
		visited = make(map[Type]struct{})
	}
	visited[root] = struct{}{}

	for child := range children(root) {
		if !visitInPostorder(*child, visited, yield) {
			return false
		}
	}

	return yield(root)
}

// children yields all direct descendants of typ.
func children(typ Type) iter.Seq[*Type] {
	return func(yield func(*Type) bool) {
		// Explicitly type switch on the most common types to allow the inliner to
		// do its work. This avoids allocating intermediate slices from walk() on
		// the heap.
		var tags []string
		switch v := typ.(type) {
		case *Void, *Int, *Enum, *Fwd, *Float, *declTag:
			// No children to traverse.
			// declTags is declared as a leaf type since it's parsed into .Tags fields of other types
			// during unmarshaling.
		case *Pointer:
			if !yield(&v.Target) {
				return
			}
		case *Array:
			if !yield(&v.Index) {
				return
			}
			if !yield(&v.Type) {
				return
			}
		case *Struct:
			for i := range v.Members {
				if !yield(&v.Members[i].Type) {
					return
				}
				for _, t := range v.Members[i].Tags {
					var tag Type = &declTag{v, t, i}
					if !yield(&tag) {
						return
					}
				}
			}
			tags = v.Tags
		case *Union:
			for i := range v.Members {
				if !yield(&v.Members[i].Type) {
					return
				}
				for _, t := range v.Members[i].Tags {
					var tag Type = &declTag{v, t, i}
					if !yield(&tag) {
						return
					}
				}
			}
			tags = v.Tags
		case *Typedef:
			if !yield(&v.Type) {
				return
			}
			tags = v.Tags
		case *Volatile:
			if !yield(&v.Type) {
				return
			}
		case *Const:
			if !yield(&v.Type) {
				return
			}
		case *Restrict:
			if !yield(&v.Type) {
				return
			}
		case *Func:
			if !yield(&v.Type) {
				return
			}
			if fp, ok := v.Type.(*FuncProto); ok {
				for i := range fp.Params {
					if len(v.ParamTags) <= i {
						continue
					}
					for _, t := range v.ParamTags[i] {
						var tag Type = &declTag{v, t, i}
						if !yield(&tag) {
							return
						}
					}
				}
			}
			tags = v.Tags
		case *FuncProto:
			if !yield(&v.Return) {
				return
			}
			for i := range v.Params {
				if !yield(&v.Params[i].Type) {
					return
				}
			}
		case *Var:
			if !yield(&v.Type) {
				return
			}
			tags = v.Tags
		case *Datasec:
			for i := range v.Vars {
				if !yield(&v.Vars[i].Type) {
					return
				}
			}
		case *TypeTag:
			if !yield(&v.Type) {
				return
			}
		case *cycle:
			// cycle has children, but we ignore them deliberately.
		default:
			panic(fmt.Sprintf("don't know how to walk Type %T", v))
		}

		for _, t := range tags {
			var tag Type = &declTag{typ, t, -1}
			if !yield(&tag) {
				return
			}
		}
	}
}
