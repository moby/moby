package source // import "gotest.tools/internal/source"

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

const baseStackIndex = 1

// FormattedCallExprArg returns the argument from an ast.CallExpr at the
// index in the call stack. The argument is formatted using FormatNode.
func FormattedCallExprArg(stackIndex int, argPos int) (string, error) {
	args, err := CallExprArgs(stackIndex + 1)
	if err != nil {
		return "", err
	}
	return FormatNode(args[argPos])
}

func getNodeAtLine(filename string, lineNum int) (ast.Node, error) {
	fileset := token.NewFileSet()
	astFile, err := parser.ParseFile(fileset, filename, nil, parser.AllErrors)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse source file: %s", filename)
	}

	node := scanToLine(fileset, astFile, lineNum)
	if node == nil {
		return nil, errors.Errorf(
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
	if v.nodePosition(node).Line == v.lineNum {
		v.matchedNode = node
		return nil
	}
	return v
}

// In golang 1.9 the line number changed from being the line where the statement
// ended to the line where the statement began.
func (v *scanToLineVisitor) nodePosition(node ast.Node) token.Position {
	if goVersionBefore19 {
		return v.fileset.Position(node.End())
	}
	return v.fileset.Position(node.Pos())
}

var goVersionBefore19 = isGOVersionBefore19()

func isGOVersionBefore19() bool {
	version := runtime.Version()
	// not a release version
	if !strings.HasPrefix(version, "go") {
		return false
	}
	version = strings.TrimPrefix(version, "go")
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return false
	}
	minor, err := strconv.ParseInt(parts[1], 10, 32)
	return err == nil && parts[0] == "1" && minor < 9
}

func getCallExprArgs(node ast.Node) ([]ast.Expr, error) {
	visitor := &callExprVisitor{}
	ast.Walk(visitor, node)
	if visitor.expr == nil {
		return nil, errors.New("failed to find call expression")
	}
	return visitor.expr.Args, nil
}

type callExprVisitor struct {
	expr *ast.CallExpr
}

func (v *callExprVisitor) Visit(node ast.Node) ast.Visitor {
	if v.expr != nil || node == nil {
		return nil
	}
	debug("visit (%T): %s", node, debugFormatNode{node})

	if callExpr, ok := node.(*ast.CallExpr); ok {
		v.expr = callExpr
		return nil
	}
	return v
}

// FormatNode using go/format.Node and return the result as a string
func FormatNode(node ast.Node) (string, error) {
	buf := new(bytes.Buffer)
	err := format.Node(buf, token.NewFileSet(), node)
	return buf.String(), err
}

// CallExprArgs returns the ast.Expr slice for the args of an ast.CallExpr at
// the index in the call stack.
func CallExprArgs(stackIndex int) ([]ast.Expr, error) {
	_, filename, lineNum, ok := runtime.Caller(baseStackIndex + stackIndex)
	if !ok {
		return nil, errors.New("failed to get call stack")
	}
	debug("call stack position: %s:%d", filename, lineNum)

	node, err := getNodeAtLine(filename, lineNum)
	if err != nil {
		return nil, err
	}
	debug("found node (%T): %s", node, debugFormatNode{node})

	return getCallExprArgs(node)
}

var debugEnabled = os.Getenv("GOTESTYOURSELF_DEBUG") != ""

func debug(format string, args ...interface{}) {
	if debugEnabled {
		fmt.Fprintf(os.Stderr, "DEBUG: "+format+"\n", args...)
	}
}

type debugFormatNode struct {
	ast.Node
}

func (n debugFormatNode) String() string {
	out, err := FormatNode(n.Node)
	if err != nil {
		return fmt.Sprintf("failed to format %s: %s", n.Node, err)
	}
	return out
}
