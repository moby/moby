package sharedcheck

import (
	"go/ast"
	"go/types"

	"honnef.co/go/tools/lint"
	. "honnef.co/go/tools/lint/lintdsl"
	"honnef.co/go/tools/ssa"
)

func CheckRangeStringRunes(j *lint.Job) {
	for _, ssafn := range j.Pkg.InitialFunctions {
		fn := func(node ast.Node) bool {
			rng, ok := node.(*ast.RangeStmt)
			if !ok || !IsBlank(rng.Key) {
				return true
			}

			v, _ := ssafn.ValueForExpr(rng.X)

			// Check that we're converting from string to []rune
			val, _ := v.(*ssa.Convert)
			if val == nil {
				return true
			}
			Tsrc, ok := val.X.Type().(*types.Basic)
			if !ok || Tsrc.Kind() != types.String {
				return true
			}
			Tdst, ok := val.Type().(*types.Slice)
			if !ok {
				return true
			}
			TdstElem, ok := Tdst.Elem().(*types.Basic)
			if !ok || TdstElem.Kind() != types.Int32 {
				return true
			}

			// Check that the result of the conversion is only used to
			// range over
			refs := val.Referrers()
			if refs == nil {
				return true
			}

			// Expect two refs: one for obtaining the length of the slice,
			// one for accessing the elements
			if len(FilterDebug(*refs)) != 2 {
				// TODO(dh): right now, we check that only one place
				// refers to our slice. This will miss cases such as
				// ranging over the slice twice. Ideally, we'd ensure that
				// the slice is only used for ranging over (without
				// accessing the key), but that is harder to do because in
				// SSA form, ranging over a slice looks like an ordinary
				// loop with index increments and slice accesses. We'd
				// have to look at the associated AST node to check that
				// it's a range statement.
				return true
			}

			j.Errorf(rng, "should range over string, not []rune(string)")

			return true
		}
		Inspect(ssafn.Syntax(), fn)
	}
}
