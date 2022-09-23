package source

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"runtime"
	"strings"
)

// Update is set by the -update flag. It indicates the user running the tests
// would like to update any golden values.
var Update bool

func init() {
	flag.BoolVar(&Update, "update", false, "update golden values")
}

// ErrNotFound indicates that UpdateExpectedValue failed to find the
// variable to update, likely because it is not a package level variable.
var ErrNotFound = fmt.Errorf("failed to find variable for update of golden value")

// UpdateExpectedValue looks for a package-level variable with a name that
// starts with expected in the arguments to the caller. If the variable is
// found, the value of the variable will be updated to value of the other
// argument to the caller.
func UpdateExpectedValue(stackIndex int, x, y interface{}) error {
	_, filename, line, ok := runtime.Caller(stackIndex + 1)
	if !ok {
		return errors.New("failed to get call stack")
	}
	debug("call stack position: %s:%d", filename, line)

	fileset := token.NewFileSet()
	astFile, err := parser.ParseFile(fileset, filename, nil, parser.AllErrors|parser.ParseComments)
	if err != nil {
		return fmt.Errorf("failed to parse source file %s: %w", filename, err)
	}

	expr, err := getCallExprArgs(fileset, astFile, line)
	if err != nil {
		return fmt.Errorf("call from %s:%d: %w", filename, line, err)
	}

	if len(expr) < 3 {
		debug("not enough arguments %d: %v",
			len(expr), debugFormatNode{Node: &ast.CallExpr{Args: expr}})
		return ErrNotFound
	}

	argIndex, varName := getVarNameForExpectedValueArg(expr)
	if argIndex < 0 || varName == "" {
		debug("no arguments started with the word 'expected': %v",
			debugFormatNode{Node: &ast.CallExpr{Args: expr}})
		return ErrNotFound
	}

	value := x
	if argIndex == 1 {
		value = y
	}

	strValue, ok := value.(string)
	if !ok {
		debug("value must be type string, got %T", value)
		return ErrNotFound
	}
	return UpdateVariable(filename, fileset, astFile, varName, strValue)
}

// UpdateVariable writes to filename the contents of astFile with the value of
// the variable updated to value.
func UpdateVariable(
	filename string,
	fileset *token.FileSet,
	astFile *ast.File,
	varName string,
	value string,
) error {
	obj := astFile.Scope.Objects[varName]
	if obj == nil {
		return ErrNotFound
	}
	if obj.Kind != ast.Con && obj.Kind != ast.Var {
		debug("can only update var and const, found %v", obj.Kind)
		return ErrNotFound
	}

	spec, ok := obj.Decl.(*ast.ValueSpec)
	if !ok {
		debug("can only update *ast.ValueSpec, found %T", obj.Decl)
		return ErrNotFound
	}
	if len(spec.Names) != 1 {
		debug("more than one name in ast.ValueSpec")
		return ErrNotFound
	}

	spec.Values[0] = &ast.BasicLit{
		Kind:  token.STRING,
		Value: "`" + value + "`",
	}

	var buf bytes.Buffer
	if err := format.Node(&buf, fileset, astFile); err != nil {
		return fmt.Errorf("failed to format file after update: %w", err)
	}

	fh, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to open file %v: %w", filename, err)
	}
	if _, err = fh.Write(buf.Bytes()); err != nil {
		return fmt.Errorf("failed to write file %v: %w", filename, err)
	}
	if err := fh.Sync(); err != nil {
		return fmt.Errorf("failed to sync file %v: %w", filename, err)
	}
	return nil
}

func getVarNameForExpectedValueArg(expr []ast.Expr) (int, string) {
	for i := 1; i < 3; i++ {
		switch e := expr[i].(type) {
		case *ast.Ident:
			if strings.HasPrefix(strings.ToLower(e.Name), "expected") {
				return i, e.Name
			}
		}
	}
	return -1, ""
}
