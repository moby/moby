package functions

import (
	"go/token"
	"go/types"

	"honnef.co/go/tools/ssa"
)

func concreteReturnTypes(fn *ssa.Function) []*types.Tuple {
	res := fn.Signature.Results()
	if res == nil {
		return nil
	}
	ifaces := make([]bool, res.Len())
	any := false
	for i := 0; i < res.Len(); i++ {
		_, ifaces[i] = res.At(i).Type().Underlying().(*types.Interface)
		any = any || ifaces[i]
	}
	if !any {
		return []*types.Tuple{res}
	}
	var out []*types.Tuple
	for _, block := range fn.Blocks {
		if len(block.Instrs) == 0 {
			continue
		}
		ret, ok := block.Instrs[len(block.Instrs)-1].(*ssa.Return)
		if !ok {
			continue
		}
		vars := make([]*types.Var, res.Len())
		for i, v := range ret.Results {
			var typ types.Type
			if !ifaces[i] {
				typ = res.At(i).Type()
			} else if mi, ok := v.(*ssa.MakeInterface); ok {
				// TODO(dh): if mi.X is a function call that returns
				// an interface, call concreteReturnTypes on that
				// function (or, really, go through Descriptions,
				// avoid infinite recursion etc, just like nil error
				// detection)

				// TODO(dh): support Phi nodes
				typ = mi.X.Type()
			} else {
				typ = res.At(i).Type()
			}
			vars[i] = types.NewParam(token.NoPos, nil, "", typ)
		}
		out = append(out, types.NewTuple(vars...))
	}
	// TODO(dh): deduplicate out
	return out
}
