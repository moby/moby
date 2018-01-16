package source

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"runtime"

	"github.com/pkg/errors"
)

const baseStackIndex = 1

// GetCondition returns the condition string by reading it from the file
// identified in the callstack. In golang 1.9 the line number changed from
// being the line where the statement ended to the line where the statement began.
func GetCondition(stackIndex int, argPos int) (string, error) {
	_, filename, lineNum, ok := runtime.Caller(baseStackIndex + stackIndex)
	if !ok {
		return "", errors.New("failed to get caller info")
	}

	node, err := getNodeAtLine(filename, lineNum)
	if err != nil {
		return "", err
	}
	return getArgSourceFromAST(node, argPos)
}

func getNodeAtLine(filename string, lineNum int) (ast.Node, error) {
	fileset := token.NewFileSet()
	astFile, err := parser.ParseFile(fileset, filename, nil, parser.AllErrors)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse source file: %s", filename)
	}

	node := scanToLine(fileset, astFile, lineNum)
	if node == nil {
		return nil, errors.Wrapf(err,
			"failed to find an expression on line %d in %s", lineNum, filename)
	}
	return node, nil
}

func scanToLine(fileset *token.FileSet, node ast.Node, lineNum int) ast.Node {
	v := &scanToLineVisitor{lineNum: lineNum, fileset: fileset}
	ast.Walk(v, node)
	return v.matchedNode
}

type scanToLineVisitor struct {
	lineNum     int
	matchedNode ast.Node
	fileset     *token.FileSet
}

func (v *scanToLineVisitor) Visit(node ast.Node) ast.Visitor {
	if node == nil || v.matchedNode != nil {
		return nil
	}

	var position token.Position
	switch {
	case runtime.Version() < "go1.9":
		position = v.fileset.Position(node.End())
	default:
		position = v.fileset.Position(node.Pos())
	}

	if position.Line == v.lineNum {
		v.matchedNode = node
		return nil
	}
	return v
}

func getArgSourceFromAST(node ast.Node, argPos int) (string, error) {
	visitor := &callExprVisitor{}
	ast.Walk(visitor, node)
	if visitor.expr == nil {
		return "", errors.Errorf("unexpected ast")
	}

	buf := new(bytes.Buffer)
	err := format.Node(buf, token.NewFileSet(), visitor.expr.Args[argPos])
	return buf.String(), err
}

type callExprVisitor struct {
	expr *ast.CallExpr
}

func (v *callExprVisitor) Visit(node ast.Node) ast.Visitor {
	switch typed := node.(type) {
	case nil:
		return nil
	case *ast.IfStmt:
		ast.Walk(v, typed.Cond)
	case *ast.CallExpr:
		v.expr = typed
	}

	if v.expr != nil {
		return nil
	}
	return v
}
