package functions

import (
	"go/token"
	"go/types"

	"honnef.co/go/tools/callgraph"
	"honnef.co/go/tools/lint/lintdsl"
	"honnef.co/go/tools/ssa"
)

// IsStub reports whether a function is a stub. A function is
// considered a stub if it has no instructions or exactly one
// instruction, which must be either returning only constant values or
// a panic.
func (d *Descriptions) IsStub(fn *ssa.Function) bool {
	if len(fn.Blocks) == 0 {
		return true
	}
	if len(fn.Blocks) > 1 {
		return false
	}
	instrs := lintdsl.FilterDebug(fn.Blocks[0].Instrs)
	if len(instrs) != 1 {
		return false
	}

	switch instrs[0].(type) {
	case *ssa.Return:
		// Since this is the only instruction, the return value must
		// be a constant. We consider all constants as stubs, not just
		// the zero value. This does not, unfortunately, cover zero
		// initialised structs, as these cause additional
		// instructions.
		return true
	case *ssa.Panic:
		return true
	default:
		return false
	}
}

func (d *Descriptions) IsPure(fn *ssa.Function) bool {
	if fn.Signature.Results().Len() == 0 {
		// A function with no return values is empty or is doing some
		// work we cannot see (for example because of build tags);
		// don't consider it pure.
		return false
	}

	for _, param := range fn.Params {
		if _, ok := param.Type().Underlying().(*types.Basic); !ok {
			return false
		}
	}

	if fn.Blocks == nil {
		return false
	}
	checkCall := func(common *ssa.CallCommon) bool {
		if common.IsInvoke() {
			return false
		}
		builtin, ok := common.Value.(*ssa.Builtin)
		if !ok {
			if common.StaticCallee() != fn {
				if common.StaticCallee() == nil {
					return false
				}
				// TODO(dh): ideally, IsPure wouldn't be responsible
				// for avoiding infinite recursion, but
				// FunctionDescriptions would be.
				node := d.CallGraph.CreateNode(common.StaticCallee())
				if callgraph.PathSearch(node, func(other *callgraph.Node) bool {
					return other.Func == fn
				}) != nil {
					return false
				}
				if !d.Get(common.StaticCallee()).Pure {
					return false
				}
			}
		} else {
			switch builtin.Name() {
			case "len", "cap", "make", "new":
			default:
				return false
			}
		}
		return true
	}
	for _, b := range fn.Blocks {
		for _, ins := range b.Instrs {
			switch ins := ins.(type) {
			case *ssa.Call:
				if !checkCall(ins.Common()) {
					return false
				}
			case *ssa.Defer:
				if !checkCall(&ins.Call) {
					return false
				}
			case *ssa.Select:
				return false
			case *ssa.Send:
				return false
			case *ssa.Go:
				return false
			case *ssa.Panic:
				return false
			case *ssa.Store:
				return false
			case *ssa.FieldAddr:
				return false
			case *ssa.UnOp:
				if ins.Op == token.MUL || ins.Op == token.AND {
					return false
				}
			}
		}
	}
	return true
}
