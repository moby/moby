package source // import "gotest.tools/v3/internal/source"

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
	if argPos >= len(args) {
		return "", errors.New("failed to find expression")
	}
	return FormatNode(args[argPos])
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
	debug("found node: %s", debugFormatNode{node})

	return getCallExprArgs(node)
}

func getNodeAtLine(filename string, lineNum int) (ast.Node, error) {
	fileset := token.NewFileSet()
	astFile, err := parser.ParseFile(fileset, filename, nil, parser.AllErrors)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse source file: %s", filename)
	}

	if node := scanToLine(fileset, astFile, lineNum); node != nil {
		return node, nil
	}
	if node := scanToDeferLine(fileset, astFile, lineNum); node != nil {
		node, err := guessDefer(node)
		if err != nil || node != nil {
			return node, err
		}
	}
	return nil, errors.Errorf(
		"failed to find an expression on line %d in %s", lineNum, filename)
}

func scanToLine(fileset *token.FileSet, node ast.Node, lineNum int) ast.Node {
	var matchedNode ast.Node
	ast.Inspect(node, func(node ast.Node) bool {
		switch {
		case node == nil || matchedNode != nil:
			return false
		case nodePosition(fileset, node).Line == lineNum:
			matchedNode = node
			return false
		}
		return true
	})
	return matchedNode
}

// In golang 1.9 the line number changed from being the line where the statement
// ended to the line where the statement began.
func nodePosition(fileset *token.FileSet, node ast.Node) token.Position {
	if goVersionBefore19 {
		return fileset.Position(node.End())
	}
	return fileset.Position(node.Pos())
}

// GoVersionLessThan returns true if runtime.Version() is semantically less than
// version major.minor. Returns false if a release version can not be parsed from
// runtime.Version().
func GoVersionLessThan(major, minor int64) bool {
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
	rMajor, err := strconv.ParseInt(parts[0], 10, 32)
	if err != nil {
		return false
	}
	if rMajor != major {
		return rMajor < major
	}
	rMinor, err := strconv.ParseInt(parts[1], 10, 32)
	if err != nil {
		return false
	}
	return rMinor < minor
}

var goVersionBefore19 = GoVersionLessThan(1, 9)

func getCallExprArgs(node ast.Node) ([]ast.Expr, error) {
	visitor := &callExprVisitor{}
	ast.Walk(visitor, node)
	if visitor.expr == nil {
		return nil, errors.New("failed to find call expression")
	}
	debug("callExpr: %s", debugFormatNode{visitor.expr})
	return visitor.expr.Args, nil
}

type callExprVisitor struct {
	expr *ast.CallExpr
}

func (v *callExprVisitor) Visit(node ast.Node) ast.Visitor {
	if v.expr != nil || node == nil {
		return nil
	}
	debug("visit: %s", debugFormatNode{node})

	switch typed := node.(type) {
	case *ast.CallExpr:
		v.expr = typed
		return nil
	case *ast.DeferStmt:
		ast.Walk(v, typed.Call.Fun)
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

var debugEnabled = os.Getenv("GOTESTTOOLS_DEBUG") != ""

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
	return fmt.Sprintf("(%T) %s", n.Node, out)
}
