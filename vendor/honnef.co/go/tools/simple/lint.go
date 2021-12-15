// Package simple contains a linter for Go source code.
package simple // import "honnef.co/go/tools/simple"

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"reflect"
	"strings"

	. "honnef.co/go/tools/arg"
	"honnef.co/go/tools/internal/sharedcheck"
	"honnef.co/go/tools/lint"
	. "honnef.co/go/tools/lint/lintdsl"

	"golang.org/x/tools/go/types/typeutil"
)

type Checker struct {
	CheckGenerated bool
	MS             *typeutil.MethodSetCache
}

func NewChecker() *Checker {
	return &Checker{
		MS: &typeutil.MethodSetCache{},
	}
}

func (*Checker) Name() string   { return "gosimple" }
func (*Checker) Prefix() string { return "S" }

func (c *Checker) Init(prog *lint.Program) {}

func (c *Checker) Checks() []lint.Check {
	return []lint.Check{
		{ID: "S1000", FilterGenerated: true, Fn: c.LintSingleCaseSelect, Doc: docS1000},
		{ID: "S1001", FilterGenerated: true, Fn: c.LintLoopCopy, Doc: docS1001},
		{ID: "S1002", FilterGenerated: true, Fn: c.LintIfBoolCmp, Doc: docS1002},
		{ID: "S1003", FilterGenerated: true, Fn: c.LintStringsContains, Doc: docS1003},
		{ID: "S1004", FilterGenerated: true, Fn: c.LintBytesCompare, Doc: docS1004},
		{ID: "S1005", FilterGenerated: true, Fn: c.LintUnnecessaryBlank, Doc: docS1005},
		{ID: "S1006", FilterGenerated: true, Fn: c.LintForTrue, Doc: docS1006},
		{ID: "S1007", FilterGenerated: true, Fn: c.LintRegexpRaw, Doc: docS1007},
		{ID: "S1008", FilterGenerated: true, Fn: c.LintIfReturn, Doc: docS1008},
		{ID: "S1009", FilterGenerated: true, Fn: c.LintRedundantNilCheckWithLen, Doc: docS1009},
		{ID: "S1010", FilterGenerated: true, Fn: c.LintSlicing, Doc: docS1010},
		{ID: "S1011", FilterGenerated: true, Fn: c.LintLoopAppend, Doc: docS1011},
		{ID: "S1012", FilterGenerated: true, Fn: c.LintTimeSince, Doc: docS1012},
		{ID: "S1016", FilterGenerated: true, Fn: c.LintSimplerStructConversion, Doc: docS1016},
		{ID: "S1017", FilterGenerated: true, Fn: c.LintTrim, Doc: docS1017},
		{ID: "S1018", FilterGenerated: true, Fn: c.LintLoopSlide, Doc: docS1018},
		{ID: "S1019", FilterGenerated: true, Fn: c.LintMakeLenCap, Doc: docS1019},
		{ID: "S1020", FilterGenerated: true, Fn: c.LintAssertNotNil, Doc: docS1020},
		{ID: "S1021", FilterGenerated: true, Fn: c.LintDeclareAssign, Doc: docS1021},
		{ID: "S1023", FilterGenerated: true, Fn: c.LintRedundantBreak, Doc: docS1023},
		{ID: "S1024", FilterGenerated: true, Fn: c.LintTimeUntil, Doc: docS1024},
		{ID: "S1025", FilterGenerated: true, Fn: c.LintRedundantSprintf, Doc: docS1025},
		{ID: "S1028", FilterGenerated: true, Fn: c.LintErrorsNewSprintf, Doc: docS1028},
		{ID: "S1029", FilterGenerated: false, Fn: c.LintRangeStringRunes, Doc: docS1029},
		{ID: "S1030", FilterGenerated: true, Fn: c.LintBytesBufferConversions, Doc: docS1030},
		{ID: "S1031", FilterGenerated: true, Fn: c.LintNilCheckAroundRange, Doc: docS1031},
		{ID: "S1032", FilterGenerated: true, Fn: c.LintSortHelpers, Doc: docS1032},
		{ID: "S1033", FilterGenerated: true, Fn: c.LintGuardedDelete, Doc: ``},
		{ID: "S1034", FilterGenerated: true, Fn: c.LintSimplifyTypeSwitch, Doc: ``},
	}
}

func (c *Checker) LintSingleCaseSelect(j *lint.Job) {
	isSingleSelect := func(node ast.Node) bool {
		v, ok := node.(*ast.SelectStmt)
		if !ok {
			return false
		}
		return len(v.Body.List) == 1
	}

	seen := map[ast.Node]struct{}{}
	fn := func(node ast.Node) {
		switch v := node.(type) {
		case *ast.ForStmt:
			if len(v.Body.List) != 1 {
				return
			}
			if !isSingleSelect(v.Body.List[0]) {
				return
			}
			if _, ok := v.Body.List[0].(*ast.SelectStmt).Body.List[0].(*ast.CommClause).Comm.(*ast.SendStmt); ok {
				// Don't suggest using range for channel sends
				return
			}
			seen[v.Body.List[0]] = struct{}{}
			j.Errorf(node, "should use for range instead of for { select {} }")
		case *ast.SelectStmt:
			if _, ok := seen[v]; ok {
				return
			}
			if !isSingleSelect(v) {
				return
			}
			j.Errorf(node, "should use a simple channel send/receive instead of select with a single case")
		}
	}
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.ForStmt)(nil), (*ast.SelectStmt)(nil)}, fn)
}

func (c *Checker) LintLoopCopy(j *lint.Job) {
	fn := func(node ast.Node) {
		loop := node.(*ast.RangeStmt)

		if loop.Key == nil {
			return
		}
		if len(loop.Body.List) != 1 {
			return
		}
		stmt, ok := loop.Body.List[0].(*ast.AssignStmt)
		if !ok {
			return
		}
		if stmt.Tok != token.ASSIGN || len(stmt.Lhs) != 1 || len(stmt.Rhs) != 1 {
			return
		}
		lhs, ok := stmt.Lhs[0].(*ast.IndexExpr)
		if !ok {
			return
		}

		if _, ok := j.Pkg.TypesInfo.TypeOf(lhs.X).(*types.Slice); !ok {
			return
		}
		lidx, ok := lhs.Index.(*ast.Ident)
		if !ok {
			return
		}
		key, ok := loop.Key.(*ast.Ident)
		if !ok {
			return
		}
		if j.Pkg.TypesInfo.TypeOf(lhs) == nil || j.Pkg.TypesInfo.TypeOf(stmt.Rhs[0]) == nil {
			return
		}
		if j.Pkg.TypesInfo.ObjectOf(lidx) != j.Pkg.TypesInfo.ObjectOf(key) {
			return
		}
		if !types.Identical(j.Pkg.TypesInfo.TypeOf(lhs), j.Pkg.TypesInfo.TypeOf(stmt.Rhs[0])) {
			return
		}
		if _, ok := j.Pkg.TypesInfo.TypeOf(loop.X).(*types.Slice); !ok {
			return
		}

		if rhs, ok := stmt.Rhs[0].(*ast.IndexExpr); ok {
			rx, ok := rhs.X.(*ast.Ident)
			_ = rx
			if !ok {
				return
			}
			ridx, ok := rhs.Index.(*ast.Ident)
			if !ok {
				return
			}
			if j.Pkg.TypesInfo.ObjectOf(ridx) != j.Pkg.TypesInfo.ObjectOf(key) {
				return
			}
		} else if rhs, ok := stmt.Rhs[0].(*ast.Ident); ok {
			value, ok := loop.Value.(*ast.Ident)
			if !ok {
				return
			}
			if j.Pkg.TypesInfo.ObjectOf(rhs) != j.Pkg.TypesInfo.ObjectOf(value) {
				return
			}
		} else {
			return
		}
		j.Errorf(loop, "should use copy() instead of a loop")
	}
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.RangeStmt)(nil)}, fn)
}

func (c *Checker) LintIfBoolCmp(j *lint.Job) {
	fn := func(node ast.Node) {
		expr := node.(*ast.BinaryExpr)
		if expr.Op != token.EQL && expr.Op != token.NEQ {
			return
		}
		x := IsBoolConst(j, expr.X)
		y := IsBoolConst(j, expr.Y)
		if !x && !y {
			return
		}
		var other ast.Expr
		var val bool
		if x {
			val = BoolConst(j, expr.X)
			other = expr.Y
		} else {
			val = BoolConst(j, expr.Y)
			other = expr.X
		}
		basic, ok := j.Pkg.TypesInfo.TypeOf(other).Underlying().(*types.Basic)
		if !ok || basic.Kind() != types.Bool {
			return
		}
		op := ""
		if (expr.Op == token.EQL && !val) || (expr.Op == token.NEQ && val) {
			op = "!"
		}
		r := op + Render(j, other)
		l1 := len(r)
		r = strings.TrimLeft(r, "!")
		if (l1-len(r))%2 == 1 {
			r = "!" + r
		}
		if IsInTest(j, node) {
			return
		}
		j.Errorf(expr, "should omit comparison to bool constant, can be simplified to %s", r)
	}
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.BinaryExpr)(nil)}, fn)
}

func (c *Checker) LintBytesBufferConversions(j *lint.Job) {
	fn := func(node ast.Node) {
		call := node.(*ast.CallExpr)
		if len(call.Args) != 1 {
			return
		}

		argCall, ok := call.Args[0].(*ast.CallExpr)
		if !ok {
			return
		}
		sel, ok := argCall.Fun.(*ast.SelectorExpr)
		if !ok {
			return
		}

		typ := j.Pkg.TypesInfo.TypeOf(call.Fun)
		if typ == types.Universe.Lookup("string").Type() && IsCallToAST(j, call.Args[0], "(*bytes.Buffer).Bytes") {
			j.Errorf(call, "should use %v.String() instead of %v", Render(j, sel.X), Render(j, call))
		} else if typ, ok := typ.(*types.Slice); ok && typ.Elem() == types.Universe.Lookup("byte").Type() && IsCallToAST(j, call.Args[0], "(*bytes.Buffer).String") {
			j.Errorf(call, "should use %v.Bytes() instead of %v", Render(j, sel.X), Render(j, call))
		}

	}
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.CallExpr)(nil)}, fn)
}

func (c *Checker) LintStringsContains(j *lint.Job) {
	// map of value to token to bool value
	allowed := map[int64]map[token.Token]bool{
		-1: {token.GTR: true, token.NEQ: true, token.EQL: false},
		0:  {token.GEQ: true, token.LSS: false},
	}
	fn := func(node ast.Node) {
		expr := node.(*ast.BinaryExpr)
		switch expr.Op {
		case token.GEQ, token.GTR, token.NEQ, token.LSS, token.EQL:
		default:
			return
		}

		value, ok := ExprToInt(j, expr.Y)
		if !ok {
			return
		}

		allowedOps, ok := allowed[value]
		if !ok {
			return
		}
		b, ok := allowedOps[expr.Op]
		if !ok {
			return
		}

		call, ok := expr.X.(*ast.CallExpr)
		if !ok {
			return
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return
		}
		pkgIdent, ok := sel.X.(*ast.Ident)
		if !ok {
			return
		}
		funIdent := sel.Sel
		if pkgIdent.Name != "strings" && pkgIdent.Name != "bytes" {
			return
		}
		newFunc := ""
		switch funIdent.Name {
		case "IndexRune":
			newFunc = "ContainsRune"
		case "IndexAny":
			newFunc = "ContainsAny"
		case "Index":
			newFunc = "Contains"
		default:
			return
		}

		prefix := ""
		if !b {
			prefix = "!"
		}
		j.Errorf(node, "should use %s%s.%s(%s) instead", prefix, pkgIdent.Name, newFunc, RenderArgs(j, call.Args))
	}
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.BinaryExpr)(nil)}, fn)
}

func (c *Checker) LintBytesCompare(j *lint.Job) {
	fn := func(node ast.Node) {
		expr := node.(*ast.BinaryExpr)
		if expr.Op != token.NEQ && expr.Op != token.EQL {
			return
		}
		call, ok := expr.X.(*ast.CallExpr)
		if !ok {
			return
		}
		if !IsCallToAST(j, call, "bytes.Compare") {
			return
		}
		value, ok := ExprToInt(j, expr.Y)
		if !ok || value != 0 {
			return
		}
		args := RenderArgs(j, call.Args)
		prefix := ""
		if expr.Op == token.NEQ {
			prefix = "!"
		}
		j.Errorf(node, "should use %sbytes.Equal(%s) instead", prefix, args)
	}
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.BinaryExpr)(nil)}, fn)
}

func (c *Checker) LintForTrue(j *lint.Job) {
	fn := func(node ast.Node) {
		loop := node.(*ast.ForStmt)
		if loop.Init != nil || loop.Post != nil {
			return
		}
		if !IsBoolConst(j, loop.Cond) || !BoolConst(j, loop.Cond) {
			return
		}
		j.Errorf(loop, "should use for {} instead of for true {}")
	}
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.ForStmt)(nil)}, fn)
}

func (c *Checker) LintRegexpRaw(j *lint.Job) {
	fn := func(node ast.Node) {
		call := node.(*ast.CallExpr)
		if !IsCallToAST(j, call, "regexp.MustCompile") &&
			!IsCallToAST(j, call, "regexp.Compile") {
			return
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return
		}
		if len(call.Args) != 1 {
			// invalid function call
			return
		}
		lit, ok := call.Args[Arg("regexp.Compile.expr")].(*ast.BasicLit)
		if !ok {
			// TODO(dominikh): support string concat, maybe support constants
			return
		}
		if lit.Kind != token.STRING {
			// invalid function call
			return
		}
		if lit.Value[0] != '"' {
			// already a raw string
			return
		}
		val := lit.Value
		if !strings.Contains(val, `\\`) {
			return
		}
		if strings.Contains(val, "`") {
			return
		}

		bs := false
		for _, c := range val {
			if !bs && c == '\\' {
				bs = true
				continue
			}
			if bs && c == '\\' {
				bs = false
				continue
			}
			if bs {
				// backslash followed by non-backslash -> escape sequence
				return
			}
		}

		j.Errorf(call, "should use raw string (`...`) with regexp.%s to avoid having to escape twice", sel.Sel.Name)
	}
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.CallExpr)(nil)}, fn)
}

func (c *Checker) LintIfReturn(j *lint.Job) {
	fn := func(node ast.Node) {
		block := node.(*ast.BlockStmt)
		l := len(block.List)
		if l < 2 {
			return
		}
		n1, n2 := block.List[l-2], block.List[l-1]

		if len(block.List) >= 3 {
			if _, ok := block.List[l-3].(*ast.IfStmt); ok {
				// Do not flag a series of if statements
				return
			}
		}
		// if statement with no init, no else, a single condition
		// checking an identifier or function call and just a return
		// statement in the body, that returns a boolean constant
		ifs, ok := n1.(*ast.IfStmt)
		if !ok {
			return
		}
		if ifs.Else != nil || ifs.Init != nil {
			return
		}
		if len(ifs.Body.List) != 1 {
			return
		}
		if op, ok := ifs.Cond.(*ast.BinaryExpr); ok {
			switch op.Op {
			case token.EQL, token.LSS, token.GTR, token.NEQ, token.LEQ, token.GEQ:
			default:
				return
			}
		}
		ret1, ok := ifs.Body.List[0].(*ast.ReturnStmt)
		if !ok {
			return
		}
		if len(ret1.Results) != 1 {
			return
		}
		if !IsBoolConst(j, ret1.Results[0]) {
			return
		}

		ret2, ok := n2.(*ast.ReturnStmt)
		if !ok {
			return
		}
		if len(ret2.Results) != 1 {
			return
		}
		if !IsBoolConst(j, ret2.Results[0]) {
			return
		}
		j.Errorf(n1, "should use 'return <expr>' instead of 'if <expr> { return <bool> }; return <bool>'")
	}
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.BlockStmt)(nil)}, fn)
}

// LintRedundantNilCheckWithLen checks for the following reduntant nil-checks:
//
//   if x == nil || len(x) == 0 {}
//   if x != nil && len(x) != 0 {}
//   if x != nil && len(x) == N {} (where N != 0)
//   if x != nil && len(x) > N {}
//   if x != nil && len(x) >= N {} (where N != 0)
//
func (c *Checker) LintRedundantNilCheckWithLen(j *lint.Job) {
	isConstZero := func(expr ast.Expr) (isConst bool, isZero bool) {
		_, ok := expr.(*ast.BasicLit)
		if ok {
			return true, IsZero(expr)
		}
		id, ok := expr.(*ast.Ident)
		if !ok {
			return false, false
		}
		c, ok := j.Pkg.TypesInfo.ObjectOf(id).(*types.Const)
		if !ok {
			return false, false
		}
		return true, c.Val().Kind() == constant.Int && c.Val().String() == "0"
	}

	fn := func(node ast.Node) {
		// check that expr is "x || y" or "x && y"
		expr := node.(*ast.BinaryExpr)
		if expr.Op != token.LOR && expr.Op != token.LAND {
			return
		}
		eqNil := expr.Op == token.LOR

		// check that x is "xx == nil" or "xx != nil"
		x, ok := expr.X.(*ast.BinaryExpr)
		if !ok {
			return
		}
		if eqNil && x.Op != token.EQL {
			return
		}
		if !eqNil && x.Op != token.NEQ {
			return
		}
		xx, ok := x.X.(*ast.Ident)
		if !ok {
			return
		}
		if !IsNil(j, x.Y) {
			return
		}

		// check that y is "len(xx) == 0" or "len(xx) ... "
		y, ok := expr.Y.(*ast.BinaryExpr)
		if !ok {
			return
		}
		if eqNil && y.Op != token.EQL { // must be len(xx) *==* 0
			return
		}
		yx, ok := y.X.(*ast.CallExpr)
		if !ok {
			return
		}
		yxFun, ok := yx.Fun.(*ast.Ident)
		if !ok || yxFun.Name != "len" || len(yx.Args) != 1 {
			return
		}
		yxArg, ok := yx.Args[Arg("len.v")].(*ast.Ident)
		if !ok {
			return
		}
		if yxArg.Name != xx.Name {
			return
		}

		if eqNil && !IsZero(y.Y) { // must be len(x) == *0*
			return
		}

		if !eqNil {
			isConst, isZero := isConstZero(y.Y)
			if !isConst {
				return
			}
			switch y.Op {
			case token.EQL:
				// avoid false positive for "xx != nil && len(xx) == 0"
				if isZero {
					return
				}
			case token.GEQ:
				// avoid false positive for "xx != nil && len(xx) >= 0"
				if isZero {
					return
				}
			case token.NEQ:
				// avoid false positive for "xx != nil && len(xx) != <non-zero>"
				if !isZero {
					return
				}
			case token.GTR:
				// ok
			default:
				return
			}
		}

		// finally check that xx type is one of array, slice, map or chan
		// this is to prevent false positive in case if xx is a pointer to an array
		var nilType string
		switch j.Pkg.TypesInfo.TypeOf(xx).(type) {
		case *types.Slice:
			nilType = "nil slices"
		case *types.Map:
			nilType = "nil maps"
		case *types.Chan:
			nilType = "nil channels"
		default:
			return
		}
		j.Errorf(expr, "should omit nil check; len() for %s is defined as zero", nilType)
	}
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.BinaryExpr)(nil)}, fn)
}

func (c *Checker) LintSlicing(j *lint.Job) {
	fn := func(node ast.Node) {
		n := node.(*ast.SliceExpr)
		if n.Max != nil {
			return
		}
		s, ok := n.X.(*ast.Ident)
		if !ok || s.Obj == nil {
			return
		}
		call, ok := n.High.(*ast.CallExpr)
		if !ok || len(call.Args) != 1 || call.Ellipsis.IsValid() {
			return
		}
		fun, ok := call.Fun.(*ast.Ident)
		if !ok || fun.Name != "len" {
			return
		}
		if _, ok := j.Pkg.TypesInfo.ObjectOf(fun).(*types.Builtin); !ok {
			return
		}
		arg, ok := call.Args[Arg("len.v")].(*ast.Ident)
		if !ok || arg.Obj != s.Obj {
			return
		}
		j.Errorf(n, "should omit second index in slice, s[a:len(s)] is identical to s[a:]")
	}
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.SliceExpr)(nil)}, fn)
}

func refersTo(j *lint.Job, expr ast.Expr, ident *ast.Ident) bool {
	found := false
	fn := func(node ast.Node) bool {
		ident2, ok := node.(*ast.Ident)
		if !ok {
			return true
		}
		if j.Pkg.TypesInfo.ObjectOf(ident) == j.Pkg.TypesInfo.ObjectOf(ident2) {
			found = true
			return false
		}
		return true
	}
	ast.Inspect(expr, fn)
	return found
}

func (c *Checker) LintLoopAppend(j *lint.Job) {
	fn := func(node ast.Node) {
		loop := node.(*ast.RangeStmt)
		if !IsBlank(loop.Key) {
			return
		}
		val, ok := loop.Value.(*ast.Ident)
		if !ok {
			return
		}
		if len(loop.Body.List) != 1 {
			return
		}
		stmt, ok := loop.Body.List[0].(*ast.AssignStmt)
		if !ok {
			return
		}
		if stmt.Tok != token.ASSIGN || len(stmt.Lhs) != 1 || len(stmt.Rhs) != 1 {
			return
		}
		if refersTo(j, stmt.Lhs[0], val) {
			return
		}
		call, ok := stmt.Rhs[0].(*ast.CallExpr)
		if !ok {
			return
		}
		if len(call.Args) != 2 || call.Ellipsis.IsValid() {
			return
		}
		fun, ok := call.Fun.(*ast.Ident)
		if !ok {
			return
		}
		obj := j.Pkg.TypesInfo.ObjectOf(fun)
		fn, ok := obj.(*types.Builtin)
		if !ok || fn.Name() != "append" {
			return
		}

		src := j.Pkg.TypesInfo.TypeOf(loop.X)
		dst := j.Pkg.TypesInfo.TypeOf(call.Args[Arg("append.slice")])
		// TODO(dominikh) remove nil check once Go issue #15173 has
		// been fixed
		if src == nil {
			return
		}
		if !types.Identical(src, dst) {
			return
		}

		if Render(j, stmt.Lhs[0]) != Render(j, call.Args[Arg("append.slice")]) {
			return
		}

		el, ok := call.Args[Arg("append.elems")].(*ast.Ident)
		if !ok {
			return
		}
		if j.Pkg.TypesInfo.ObjectOf(val) != j.Pkg.TypesInfo.ObjectOf(el) {
			return
		}
		j.Errorf(loop, "should replace loop with %s = append(%s, %s...)",
			Render(j, stmt.Lhs[0]), Render(j, call.Args[Arg("append.slice")]), Render(j, loop.X))
	}
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.RangeStmt)(nil)}, fn)
}

func (c *Checker) LintTimeSince(j *lint.Job) {
	fn := func(node ast.Node) {
		call := node.(*ast.CallExpr)
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return
		}
		if !IsCallToAST(j, sel.X, "time.Now") {
			return
		}
		if sel.Sel.Name != "Sub" {
			return
		}
		j.Errorf(call, "should use time.Since instead of time.Now().Sub")
	}
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.CallExpr)(nil)}, fn)
}

func (c *Checker) LintTimeUntil(j *lint.Job) {
	if !IsGoVersion(j, 8) {
		return
	}
	fn := func(node ast.Node) {
		call := node.(*ast.CallExpr)
		if !IsCallToAST(j, call, "(time.Time).Sub") {
			return
		}
		if !IsCallToAST(j, call.Args[Arg("(time.Time).Sub.u")], "time.Now") {
			return
		}
		j.Errorf(call, "should use time.Until instead of t.Sub(time.Now())")
	}
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.CallExpr)(nil)}, fn)
}

func (c *Checker) LintUnnecessaryBlank(j *lint.Job) {
	fn1 := func(node ast.Node) {
		assign := node.(*ast.AssignStmt)
		if len(assign.Lhs) != 2 || len(assign.Rhs) != 1 {
			return
		}
		if !IsBlank(assign.Lhs[1]) {
			return
		}
		switch rhs := assign.Rhs[0].(type) {
		case *ast.IndexExpr:
			// The type-checker should make sure that it's a map, but
			// let's be safe.
			if _, ok := j.Pkg.TypesInfo.TypeOf(rhs.X).Underlying().(*types.Map); !ok {
				return
			}
		case *ast.UnaryExpr:
			if rhs.Op != token.ARROW {
				return
			}
		default:
			return
		}
		cp := *assign
		cp.Lhs = cp.Lhs[0:1]
		j.Errorf(assign, "should write %s instead of %s", Render(j, &cp), Render(j, assign))
	}

	fn2 := func(node ast.Node) {
		stmt := node.(*ast.AssignStmt)
		if len(stmt.Lhs) != len(stmt.Rhs) {
			return
		}
		for i, lh := range stmt.Lhs {
			rh := stmt.Rhs[i]
			if !IsBlank(lh) {
				continue
			}
			expr, ok := rh.(*ast.UnaryExpr)
			if !ok {
				continue
			}
			if expr.Op != token.ARROW {
				continue
			}
			j.Errorf(lh, "'_ = <-ch' can be simplified to '<-ch'")
		}
	}

	fn3 := func(node ast.Node) {
		rs := node.(*ast.RangeStmt)

		// for x, _
		if !IsBlank(rs.Key) && IsBlank(rs.Value) {
			j.Errorf(rs.Value, "should omit value from range; this loop is equivalent to `for %s %s range ...`", Render(j, rs.Key), rs.Tok)
		}
		// for _, _ || for _
		if IsBlank(rs.Key) && (IsBlank(rs.Value) || rs.Value == nil) {
			j.Errorf(rs.Key, "should omit values from range; this loop is equivalent to `for range ...`")
		}
	}

	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.AssignStmt)(nil)}, fn1)
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.AssignStmt)(nil)}, fn2)
	if IsGoVersion(j, 4) {
		j.Pkg.Inspector.Preorder([]ast.Node{(*ast.RangeStmt)(nil)}, fn3)
	}
}

func (c *Checker) LintSimplerStructConversion(j *lint.Job) {
	var skip ast.Node
	fn := func(node ast.Node) {
		// Do not suggest type conversion between pointers
		if unary, ok := node.(*ast.UnaryExpr); ok && unary.Op == token.AND {
			if lit, ok := unary.X.(*ast.CompositeLit); ok {
				skip = lit
			}
			return
		}

		if node == skip {
			return
		}

		lit, ok := node.(*ast.CompositeLit)
		if !ok {
			return
		}
		typ1, _ := j.Pkg.TypesInfo.TypeOf(lit.Type).(*types.Named)
		if typ1 == nil {
			return
		}
		s1, ok := typ1.Underlying().(*types.Struct)
		if !ok {
			return
		}

		var typ2 *types.Named
		var ident *ast.Ident
		getSelType := func(expr ast.Expr) (types.Type, *ast.Ident, bool) {
			sel, ok := expr.(*ast.SelectorExpr)
			if !ok {
				return nil, nil, false
			}
			ident, ok := sel.X.(*ast.Ident)
			if !ok {
				return nil, nil, false
			}
			typ := j.Pkg.TypesInfo.TypeOf(sel.X)
			return typ, ident, typ != nil
		}
		if len(lit.Elts) == 0 {
			return
		}
		if s1.NumFields() != len(lit.Elts) {
			return
		}
		for i, elt := range lit.Elts {
			var t types.Type
			var id *ast.Ident
			var ok bool
			switch elt := elt.(type) {
			case *ast.SelectorExpr:
				t, id, ok = getSelType(elt)
				if !ok {
					return
				}
				if i >= s1.NumFields() || s1.Field(i).Name() != elt.Sel.Name {
					return
				}
			case *ast.KeyValueExpr:
				var sel *ast.SelectorExpr
				sel, ok = elt.Value.(*ast.SelectorExpr)
				if !ok {
					return
				}

				if elt.Key.(*ast.Ident).Name != sel.Sel.Name {
					return
				}
				t, id, ok = getSelType(elt.Value)
			}
			if !ok {
				return
			}
			// All fields must be initialized from the same object
			if ident != nil && ident.Obj != id.Obj {
				return
			}
			typ2, _ = t.(*types.Named)
			if typ2 == nil {
				return
			}
			ident = id
		}

		if typ2 == nil {
			return
		}

		if typ1.Obj().Pkg() != typ2.Obj().Pkg() {
			// Do not suggest type conversions between different
			// packages. Types in different packages might only match
			// by coincidence. Furthermore, if the dependency ever
			// adds more fields to its type, it could break the code
			// that relies on the type conversion to work.
			return
		}

		s2, ok := typ2.Underlying().(*types.Struct)
		if !ok {
			return
		}
		if typ1 == typ2 {
			return
		}
		if IsGoVersion(j, 8) {
			if !types.IdenticalIgnoreTags(s1, s2) {
				return
			}
		} else {
			if !types.Identical(s1, s2) {
				return
			}
		}
		j.Errorf(node, "should convert %s (type %s) to %s instead of using struct literal",
			ident.Name, typ2.Obj().Name(), typ1.Obj().Name())
	}
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.UnaryExpr)(nil), (*ast.CompositeLit)(nil)}, fn)
}

func (c *Checker) LintTrim(j *lint.Job) {
	sameNonDynamic := func(node1, node2 ast.Node) bool {
		if reflect.TypeOf(node1) != reflect.TypeOf(node2) {
			return false
		}

		switch node1 := node1.(type) {
		case *ast.Ident:
			return node1.Obj == node2.(*ast.Ident).Obj
		case *ast.SelectorExpr:
			return Render(j, node1) == Render(j, node2)
		case *ast.IndexExpr:
			return Render(j, node1) == Render(j, node2)
		}
		return false
	}

	isLenOnIdent := func(fn ast.Expr, ident ast.Expr) bool {
		call, ok := fn.(*ast.CallExpr)
		if !ok {
			return false
		}
		if fn, ok := call.Fun.(*ast.Ident); !ok || fn.Name != "len" {
			return false
		}
		if len(call.Args) != 1 {
			return false
		}
		return sameNonDynamic(call.Args[Arg("len.v")], ident)
	}

	fn := func(node ast.Node) {
		var pkg string
		var fun string

		ifstmt := node.(*ast.IfStmt)
		if ifstmt.Init != nil {
			return
		}
		if ifstmt.Else != nil {
			return
		}
		if len(ifstmt.Body.List) != 1 {
			return
		}
		condCall, ok := ifstmt.Cond.(*ast.CallExpr)
		if !ok {
			return
		}
		switch {
		case IsCallToAST(j, condCall, "strings.HasPrefix"):
			pkg = "strings"
			fun = "HasPrefix"
		case IsCallToAST(j, condCall, "strings.HasSuffix"):
			pkg = "strings"
			fun = "HasSuffix"
		case IsCallToAST(j, condCall, "strings.Contains"):
			pkg = "strings"
			fun = "Contains"
		case IsCallToAST(j, condCall, "bytes.HasPrefix"):
			pkg = "bytes"
			fun = "HasPrefix"
		case IsCallToAST(j, condCall, "bytes.HasSuffix"):
			pkg = "bytes"
			fun = "HasSuffix"
		case IsCallToAST(j, condCall, "bytes.Contains"):
			pkg = "bytes"
			fun = "Contains"
		default:
			return
		}

		assign, ok := ifstmt.Body.List[0].(*ast.AssignStmt)
		if !ok {
			return
		}
		if assign.Tok != token.ASSIGN {
			return
		}
		if len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
			return
		}
		if !sameNonDynamic(condCall.Args[0], assign.Lhs[0]) {
			return
		}

		switch rhs := assign.Rhs[0].(type) {
		case *ast.CallExpr:
			if len(rhs.Args) < 2 || !sameNonDynamic(condCall.Args[0], rhs.Args[0]) || !sameNonDynamic(condCall.Args[1], rhs.Args[1]) {
				return
			}
			if IsCallToAST(j, condCall, "strings.HasPrefix") && IsCallToAST(j, rhs, "strings.TrimPrefix") ||
				IsCallToAST(j, condCall, "strings.HasSuffix") && IsCallToAST(j, rhs, "strings.TrimSuffix") ||
				IsCallToAST(j, condCall, "strings.Contains") && IsCallToAST(j, rhs, "strings.Replace") ||
				IsCallToAST(j, condCall, "bytes.HasPrefix") && IsCallToAST(j, rhs, "bytes.TrimPrefix") ||
				IsCallToAST(j, condCall, "bytes.HasSuffix") && IsCallToAST(j, rhs, "bytes.TrimSuffix") ||
				IsCallToAST(j, condCall, "bytes.Contains") && IsCallToAST(j, rhs, "bytes.Replace") {
				j.Errorf(ifstmt, "should replace this if statement with an unconditional %s", CallNameAST(j, rhs))
			}
			return
		case *ast.SliceExpr:
			slice := rhs
			if !ok {
				return
			}
			if slice.Slice3 {
				return
			}
			if !sameNonDynamic(slice.X, condCall.Args[0]) {
				return
			}
			var index ast.Expr
			switch fun {
			case "HasPrefix":
				// TODO(dh) We could detect a High that is len(s), but another
				// rule will already flag that, anyway.
				if slice.High != nil {
					return
				}
				index = slice.Low
			case "HasSuffix":
				if slice.Low != nil {
					n, ok := ExprToInt(j, slice.Low)
					if !ok || n != 0 {
						return
					}
				}
				index = slice.High
			}

			switch index := index.(type) {
			case *ast.CallExpr:
				if fun != "HasPrefix" {
					return
				}
				if fn, ok := index.Fun.(*ast.Ident); !ok || fn.Name != "len" {
					return
				}
				if len(index.Args) != 1 {
					return
				}
				id3 := index.Args[Arg("len.v")]
				switch oid3 := condCall.Args[1].(type) {
				case *ast.BasicLit:
					if pkg != "strings" {
						return
					}
					lit, ok := id3.(*ast.BasicLit)
					if !ok {
						return
					}
					s1, ok1 := ExprToString(j, lit)
					s2, ok2 := ExprToString(j, condCall.Args[1])
					if !ok1 || !ok2 || s1 != s2 {
						return
					}
				default:
					if !sameNonDynamic(id3, oid3) {
						return
					}
				}
			case *ast.BasicLit, *ast.Ident:
				if fun != "HasPrefix" {
					return
				}
				if pkg != "strings" {
					return
				}
				string, ok1 := ExprToString(j, condCall.Args[1])
				int, ok2 := ExprToInt(j, slice.Low)
				if !ok1 || !ok2 || int != int64(len(string)) {
					return
				}
			case *ast.BinaryExpr:
				if fun != "HasSuffix" {
					return
				}
				if index.Op != token.SUB {
					return
				}
				if !isLenOnIdent(index.X, condCall.Args[0]) ||
					!isLenOnIdent(index.Y, condCall.Args[1]) {
					return
				}
			default:
				return
			}

			var replacement string
			switch fun {
			case "HasPrefix":
				replacement = "TrimPrefix"
			case "HasSuffix":
				replacement = "TrimSuffix"
			}
			j.Errorf(ifstmt, "should replace this if statement with an unconditional %s.%s", pkg, replacement)
		}
	}
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.IfStmt)(nil)}, fn)
}

func (c *Checker) LintLoopSlide(j *lint.Job) {
	// TODO(dh): detect bs[i+offset] in addition to bs[offset+i]
	// TODO(dh): consider merging this function with LintLoopCopy
	// TODO(dh): detect length that is an expression, not a variable name
	// TODO(dh): support sliding to a different offset than the beginning of the slice

	fn := func(node ast.Node) {
		/*
			for i := 0; i < n; i++ {
				bs[i] = bs[offset+i]
			}

						↓

			copy(bs[:n], bs[offset:offset+n])
		*/

		loop := node.(*ast.ForStmt)
		if len(loop.Body.List) != 1 || loop.Init == nil || loop.Cond == nil || loop.Post == nil {
			return
		}
		assign, ok := loop.Init.(*ast.AssignStmt)
		if !ok || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 || !IsZero(assign.Rhs[0]) {
			return
		}
		initvar, ok := assign.Lhs[0].(*ast.Ident)
		if !ok {
			return
		}
		post, ok := loop.Post.(*ast.IncDecStmt)
		if !ok || post.Tok != token.INC {
			return
		}
		postvar, ok := post.X.(*ast.Ident)
		if !ok || j.Pkg.TypesInfo.ObjectOf(postvar) != j.Pkg.TypesInfo.ObjectOf(initvar) {
			return
		}
		bin, ok := loop.Cond.(*ast.BinaryExpr)
		if !ok || bin.Op != token.LSS {
			return
		}
		binx, ok := bin.X.(*ast.Ident)
		if !ok || j.Pkg.TypesInfo.ObjectOf(binx) != j.Pkg.TypesInfo.ObjectOf(initvar) {
			return
		}
		biny, ok := bin.Y.(*ast.Ident)
		if !ok {
			return
		}

		assign, ok = loop.Body.List[0].(*ast.AssignStmt)
		if !ok || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 || assign.Tok != token.ASSIGN {
			return
		}
		lhs, ok := assign.Lhs[0].(*ast.IndexExpr)
		if !ok {
			return
		}
		rhs, ok := assign.Rhs[0].(*ast.IndexExpr)
		if !ok {
			return
		}

		bs1, ok := lhs.X.(*ast.Ident)
		if !ok {
			return
		}
		bs2, ok := rhs.X.(*ast.Ident)
		if !ok {
			return
		}
		obj1 := j.Pkg.TypesInfo.ObjectOf(bs1)
		obj2 := j.Pkg.TypesInfo.ObjectOf(bs2)
		if obj1 != obj2 {
			return
		}
		if _, ok := obj1.Type().Underlying().(*types.Slice); !ok {
			return
		}

		index1, ok := lhs.Index.(*ast.Ident)
		if !ok || j.Pkg.TypesInfo.ObjectOf(index1) != j.Pkg.TypesInfo.ObjectOf(initvar) {
			return
		}
		index2, ok := rhs.Index.(*ast.BinaryExpr)
		if !ok || index2.Op != token.ADD {
			return
		}
		add1, ok := index2.X.(*ast.Ident)
		if !ok {
			return
		}
		add2, ok := index2.Y.(*ast.Ident)
		if !ok || j.Pkg.TypesInfo.ObjectOf(add2) != j.Pkg.TypesInfo.ObjectOf(initvar) {
			return
		}

		j.Errorf(loop, "should use copy(%s[:%s], %s[%s:]) instead", Render(j, bs1), Render(j, biny), Render(j, bs1), Render(j, add1))
	}
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.ForStmt)(nil)}, fn)
}

func (c *Checker) LintMakeLenCap(j *lint.Job) {
	fn := func(node ast.Node) {
		call := node.(*ast.CallExpr)
		if fn, ok := call.Fun.(*ast.Ident); !ok || fn.Name != "make" {
			// FIXME check whether make is indeed the built-in function
			return
		}
		switch len(call.Args) {
		case 2:
			// make(T, len)
			if _, ok := j.Pkg.TypesInfo.TypeOf(call.Args[Arg("make.t")]).Underlying().(*types.Slice); ok {
				break
			}
			if IsZero(call.Args[Arg("make.size[0]")]) {
				j.Errorf(call.Args[Arg("make.size[0]")], "should use make(%s) instead", Render(j, call.Args[Arg("make.t")]))
			}
		case 3:
			// make(T, len, cap)
			if Render(j, call.Args[Arg("make.size[0]")]) == Render(j, call.Args[Arg("make.size[1]")]) {
				j.Errorf(call.Args[Arg("make.size[0]")],
					"should use make(%s, %s) instead",
					Render(j, call.Args[Arg("make.t")]), Render(j, call.Args[Arg("make.size[0]")]))
			}
		}
	}
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.CallExpr)(nil)}, fn)
}

func (c *Checker) LintAssertNotNil(j *lint.Job) {
	isNilCheck := func(ident *ast.Ident, expr ast.Expr) bool {
		xbinop, ok := expr.(*ast.BinaryExpr)
		if !ok || xbinop.Op != token.NEQ {
			return false
		}
		xident, ok := xbinop.X.(*ast.Ident)
		if !ok || xident.Obj != ident.Obj {
			return false
		}
		if !IsNil(j, xbinop.Y) {
			return false
		}
		return true
	}
	isOKCheck := func(ident *ast.Ident, expr ast.Expr) bool {
		yident, ok := expr.(*ast.Ident)
		if !ok || yident.Obj != ident.Obj {
			return false
		}
		return true
	}
	fn1 := func(node ast.Node) {
		ifstmt := node.(*ast.IfStmt)
		assign, ok := ifstmt.Init.(*ast.AssignStmt)
		if !ok || len(assign.Lhs) != 2 || len(assign.Rhs) != 1 || !IsBlank(assign.Lhs[0]) {
			return
		}
		assert, ok := assign.Rhs[0].(*ast.TypeAssertExpr)
		if !ok {
			return
		}
		binop, ok := ifstmt.Cond.(*ast.BinaryExpr)
		if !ok || binop.Op != token.LAND {
			return
		}
		assertIdent, ok := assert.X.(*ast.Ident)
		if !ok {
			return
		}
		assignIdent, ok := assign.Lhs[1].(*ast.Ident)
		if !ok {
			return
		}
		if !(isNilCheck(assertIdent, binop.X) && isOKCheck(assignIdent, binop.Y)) &&
			!(isNilCheck(assertIdent, binop.Y) && isOKCheck(assignIdent, binop.X)) {
			return
		}
		j.Errorf(ifstmt, "when %s is true, %s can't be nil", Render(j, assignIdent), Render(j, assertIdent))
	}
	fn2 := func(node ast.Node) {
		// Check that outer ifstmt is an 'if x != nil {}'
		ifstmt := node.(*ast.IfStmt)
		if ifstmt.Init != nil {
			return
		}
		if ifstmt.Else != nil {
			return
		}
		if len(ifstmt.Body.List) != 1 {
			return
		}
		binop, ok := ifstmt.Cond.(*ast.BinaryExpr)
		if !ok {
			return
		}
		if binop.Op != token.NEQ {
			return
		}
		lhs, ok := binop.X.(*ast.Ident)
		if !ok {
			return
		}
		if !IsNil(j, binop.Y) {
			return
		}

		// Check that inner ifstmt is an `if _, ok := x.(T); ok {}`
		ifstmt, ok = ifstmt.Body.List[0].(*ast.IfStmt)
		if !ok {
			return
		}
		assign, ok := ifstmt.Init.(*ast.AssignStmt)
		if !ok || len(assign.Lhs) != 2 || len(assign.Rhs) != 1 || !IsBlank(assign.Lhs[0]) {
			return
		}
		assert, ok := assign.Rhs[0].(*ast.TypeAssertExpr)
		if !ok {
			return
		}
		assertIdent, ok := assert.X.(*ast.Ident)
		if !ok {
			return
		}
		if lhs.Obj != assertIdent.Obj {
			return
		}
		assignIdent, ok := assign.Lhs[1].(*ast.Ident)
		if !ok {
			return
		}
		if !isOKCheck(assignIdent, ifstmt.Cond) {
			return
		}
		j.Errorf(ifstmt, "when %s is true, %s can't be nil", Render(j, assignIdent), Render(j, assertIdent))
	}
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.IfStmt)(nil)}, fn1)
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.IfStmt)(nil)}, fn2)
}

func (c *Checker) LintDeclareAssign(j *lint.Job) {
	hasMultipleAssignments := func(root ast.Node, ident *ast.Ident) bool {
		num := 0
		ast.Inspect(root, func(node ast.Node) bool {
			if num >= 2 {
				return false
			}
			assign, ok := node.(*ast.AssignStmt)
			if !ok {
				return true
			}
			for _, lhs := range assign.Lhs {
				if oident, ok := lhs.(*ast.Ident); ok {
					if oident.Obj == ident.Obj {
						num++
					}
				}
			}

			return true
		})
		return num >= 2
	}
	fn := func(node ast.Node) {
		block := node.(*ast.BlockStmt)
		if len(block.List) < 2 {
			return
		}
		for i, stmt := range block.List[:len(block.List)-1] {
			_ = i
			decl, ok := stmt.(*ast.DeclStmt)
			if !ok {
				continue
			}
			gdecl, ok := decl.Decl.(*ast.GenDecl)
			if !ok || gdecl.Tok != token.VAR || len(gdecl.Specs) != 1 {
				continue
			}
			vspec, ok := gdecl.Specs[0].(*ast.ValueSpec)
			if !ok || len(vspec.Names) != 1 || len(vspec.Values) != 0 {
				continue
			}

			assign, ok := block.List[i+1].(*ast.AssignStmt)
			if !ok || assign.Tok != token.ASSIGN {
				continue
			}
			if len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
				continue
			}
			ident, ok := assign.Lhs[0].(*ast.Ident)
			if !ok {
				continue
			}
			if vspec.Names[0].Obj != ident.Obj {
				continue
			}

			if refersTo(j, assign.Rhs[0], ident) {
				continue
			}
			if hasMultipleAssignments(block, ident) {
				continue
			}

			j.Errorf(decl, "should merge variable declaration with assignment on next line")
		}
	}
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.BlockStmt)(nil)}, fn)
}

func (c *Checker) LintRedundantBreak(j *lint.Job) {
	fn1 := func(node ast.Node) {
		clause := node.(*ast.CaseClause)
		if len(clause.Body) < 2 {
			return
		}
		branch, ok := clause.Body[len(clause.Body)-1].(*ast.BranchStmt)
		if !ok || branch.Tok != token.BREAK || branch.Label != nil {
			return
		}
		j.Errorf(branch, "redundant break statement")
	}
	fn2 := func(node ast.Node) {
		var ret *ast.FieldList
		var body *ast.BlockStmt
		switch x := node.(type) {
		case *ast.FuncDecl:
			ret = x.Type.Results
			body = x.Body
		case *ast.FuncLit:
			ret = x.Type.Results
			body = x.Body
		default:
			panic(fmt.Sprintf("unreachable: %T", node))
		}
		// if the func has results, a return can't be redundant.
		// similarly, if there are no statements, there can be
		// no return.
		if ret != nil || body == nil || len(body.List) < 1 {
			return
		}
		rst, ok := body.List[len(body.List)-1].(*ast.ReturnStmt)
		if !ok {
			return
		}
		// we don't need to check rst.Results as we already
		// checked x.Type.Results to be nil.
		j.Errorf(rst, "redundant return statement")
	}
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.CaseClause)(nil)}, fn1)
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.FuncDecl)(nil), (*ast.FuncLit)(nil)}, fn2)
}

func isStringer(T types.Type) bool {
	ms := types.NewMethodSet(T)
	sel := ms.Lookup(nil, "String")
	if sel == nil {
		return false
	}
	fn, ok := sel.Obj().(*types.Func)
	if !ok {
		// should be unreachable
		return false
	}
	sig := fn.Type().(*types.Signature)
	if sig.Params().Len() != 0 {
		return false
	}
	if sig.Results().Len() != 1 {
		return false
	}
	if !IsType(sig.Results().At(0).Type(), "string") {
		return false
	}
	return true
}

func (c *Checker) LintRedundantSprintf(j *lint.Job) {
	fn := func(node ast.Node) {
		call := node.(*ast.CallExpr)
		if !IsCallToAST(j, call, "fmt.Sprintf") {
			return
		}
		if len(call.Args) != 2 {
			return
		}
		if s, ok := ExprToString(j, call.Args[Arg("fmt.Sprintf.format")]); !ok || s != "%s" {
			return
		}
		arg := call.Args[Arg("fmt.Sprintf.a[0]")]
		typ := j.Pkg.TypesInfo.TypeOf(arg)

		if isStringer(typ) {
			j.Errorf(call, "should use String() instead of fmt.Sprintf")
			return
		}

		if typ.Underlying() == types.Universe.Lookup("string").Type() {
			if typ == types.Universe.Lookup("string").Type() {
				j.Errorf(call, "the argument is already a string, there's no need to use fmt.Sprintf")
			} else {
				j.Errorf(call, "the argument's underlying type is a string, should use a simple conversion instead of fmt.Sprintf")
			}
		}
	}
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.CallExpr)(nil)}, fn)
}

func (c *Checker) LintErrorsNewSprintf(j *lint.Job) {
	fn := func(node ast.Node) {
		if !IsCallToAST(j, node, "errors.New") {
			return
		}
		call := node.(*ast.CallExpr)
		if !IsCallToAST(j, call.Args[Arg("errors.New.text")], "fmt.Sprintf") {
			return
		}
		j.Errorf(node, "should use fmt.Errorf(...) instead of errors.New(fmt.Sprintf(...))")
	}
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.CallExpr)(nil)}, fn)
}

func (c *Checker) LintRangeStringRunes(j *lint.Job) {
	sharedcheck.CheckRangeStringRunes(j)
}

func (c *Checker) LintNilCheckAroundRange(j *lint.Job) {
	fn := func(node ast.Node) {
		ifstmt := node.(*ast.IfStmt)
		cond, ok := ifstmt.Cond.(*ast.BinaryExpr)
		if !ok {
			return
		}

		if cond.Op != token.NEQ || !IsNil(j, cond.Y) || len(ifstmt.Body.List) != 1 {
			return
		}

		loop, ok := ifstmt.Body.List[0].(*ast.RangeStmt)
		if !ok {
			return
		}
		ifXIdent, ok := cond.X.(*ast.Ident)
		if !ok {
			return
		}
		rangeXIdent, ok := loop.X.(*ast.Ident)
		if !ok {
			return
		}
		if ifXIdent.Obj != rangeXIdent.Obj {
			return
		}
		switch j.Pkg.TypesInfo.TypeOf(rangeXIdent).(type) {
		case *types.Slice, *types.Map:
			j.Errorf(node, "unnecessary nil check around range")
		}
	}
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.IfStmt)(nil)}, fn)
}

func isPermissibleSort(j *lint.Job, node ast.Node) bool {
	call := node.(*ast.CallExpr)
	typeconv, ok := call.Args[0].(*ast.CallExpr)
	if !ok {
		return true
	}

	sel, ok := typeconv.Fun.(*ast.SelectorExpr)
	if !ok {
		return true
	}
	name := SelectorName(j, sel)
	switch name {
	case "sort.IntSlice", "sort.Float64Slice", "sort.StringSlice":
	default:
		return true
	}

	return false
}

func (c *Checker) LintSortHelpers(j *lint.Job) {
	fn := func(node ast.Node) {
		var body *ast.BlockStmt
		switch node := node.(type) {
		case *ast.FuncLit:
			body = node.Body
		case *ast.FuncDecl:
			body = node.Body
		default:
			panic(fmt.Sprintf("unreachable: %T", node))
		}
		if body == nil {
			return
		}

		type Error struct {
			node lint.Positioner
			msg  string
		}
		var errors []Error
		permissible := false
		fnSorts := func(node ast.Node) bool {
			if permissible {
				return false
			}
			if !IsCallToAST(j, node, "sort.Sort") {
				return true
			}
			if isPermissibleSort(j, node) {
				permissible = true
				return false
			}
			call := node.(*ast.CallExpr)
			typeconv := call.Args[Arg("sort.Sort.data")].(*ast.CallExpr)
			sel := typeconv.Fun.(*ast.SelectorExpr)
			name := SelectorName(j, sel)

			switch name {
			case "sort.IntSlice":
				errors = append(errors, Error{node, "should use sort.Ints(...) instead of sort.Sort(sort.IntSlice(...))"})
			case "sort.Float64Slice":
				errors = append(errors, Error{node, "should use sort.Float64s(...) instead of sort.Sort(sort.Float64Slice(...))"})
			case "sort.StringSlice":
				errors = append(errors, Error{node, "should use sort.Strings(...) instead of sort.Sort(sort.StringSlice(...))"})
			}
			return true
		}
		ast.Inspect(body, fnSorts)

		if permissible {
			return
		}
		for _, err := range errors {
			j.Errorf(err.node, "%s", err.msg)
		}
		return
	}
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.FuncLit)(nil), (*ast.FuncDecl)(nil)}, fn)
}

func (c *Checker) LintGuardedDelete(j *lint.Job) {
	isCommaOkMapIndex := func(stmt ast.Stmt) (b *ast.Ident, m ast.Expr, key ast.Expr, ok bool) {
		// Has to be of the form `_, <b:*ast.Ident> = <m:*types.Map>[<key>]

		assign, ok := stmt.(*ast.AssignStmt)
		if !ok {
			return nil, nil, nil, false
		}
		if len(assign.Lhs) != 2 || len(assign.Rhs) != 1 {
			return nil, nil, nil, false
		}
		if !IsBlank(assign.Lhs[0]) {
			return nil, nil, nil, false
		}
		ident, ok := assign.Lhs[1].(*ast.Ident)
		if !ok {
			return nil, nil, nil, false
		}
		index, ok := assign.Rhs[0].(*ast.IndexExpr)
		if !ok {
			return nil, nil, nil, false
		}
		if _, ok := j.Pkg.TypesInfo.TypeOf(index.X).(*types.Map); !ok {
			return nil, nil, nil, false
		}
		key = index.Index
		return ident, index.X, key, true
	}
	fn := func(node ast.Node) {
		stmt := node.(*ast.IfStmt)
		if len(stmt.Body.List) != 1 {
			return
		}
		if stmt.Else != nil {
			return
		}
		expr, ok := stmt.Body.List[0].(*ast.ExprStmt)
		if !ok {
			return
		}
		call, ok := expr.X.(*ast.CallExpr)
		if !ok {
			return
		}
		if !IsCallToAST(j, call, "delete") {
			return
		}
		b, m, key, ok := isCommaOkMapIndex(stmt.Init)
		if !ok {
			return
		}
		if cond, ok := stmt.Cond.(*ast.Ident); !ok || j.Pkg.TypesInfo.ObjectOf(cond) != j.Pkg.TypesInfo.ObjectOf(b) {
			return
		}
		if Render(j, call.Args[0]) != Render(j, m) || Render(j, call.Args[1]) != Render(j, key) {
			return
		}
		j.Errorf(stmt, "unnecessary guard around call to delete")
	}
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.IfStmt)(nil)}, fn)
}

func (c *Checker) LintSimplifyTypeSwitch(j *lint.Job) {
	fn := func(node ast.Node) {
		stmt := node.(*ast.TypeSwitchStmt)
		if stmt.Init != nil {
			// bailing out for now, can't anticipate how type switches with initializers are being used
			return
		}
		expr, ok := stmt.Assign.(*ast.ExprStmt)
		if !ok {
			// the user is in fact assigning the result
			return
		}
		assert := expr.X.(*ast.TypeAssertExpr)
		ident, ok := assert.X.(*ast.Ident)
		if !ok {
			return
		}
		x := j.Pkg.TypesInfo.ObjectOf(ident)
		var allOffenders []ast.Node
		for _, clause := range stmt.Body.List {
			clause := clause.(*ast.CaseClause)
			if len(clause.List) != 1 {
				continue
			}
			hasUnrelatedAssertion := false
			var offenders []ast.Node
			ast.Inspect(clause, func(node ast.Node) bool {
				assert2, ok := node.(*ast.TypeAssertExpr)
				if !ok {
					return true
				}
				ident, ok := assert2.X.(*ast.Ident)
				if !ok {
					hasUnrelatedAssertion = true
					return false
				}
				if j.Pkg.TypesInfo.ObjectOf(ident) != x {
					hasUnrelatedAssertion = true
					return false
				}

				if !types.Identical(j.Pkg.TypesInfo.TypeOf(clause.List[0]), j.Pkg.TypesInfo.TypeOf(assert2.Type)) {
					hasUnrelatedAssertion = true
					return false
				}
				offenders = append(offenders, assert2)
				return true
			})
			if !hasUnrelatedAssertion {
				// don't flag cases that have other type assertions
				// unrelated to the one in the case clause. often
				// times, this is done for symmetry, when two
				// different values have to be asserted to the same
				// type.
				allOffenders = append(allOffenders, offenders...)
			}
		}
		if len(allOffenders) != 0 {
			at := ""
			for _, offender := range allOffenders {
				pos := lint.DisplayPosition(j.Pkg.Fset, offender.Pos())
				at += "\n\t" + pos.String()
			}
			j.Errorf(expr, "assigning the result of this type assertion to a variable (switch %s := %s.(type)) could eliminate the following type assertions:%s", Render(j, ident), Render(j, ident), at)
		}
	}
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.TypeSwitchStmt)(nil)}, fn)
}
