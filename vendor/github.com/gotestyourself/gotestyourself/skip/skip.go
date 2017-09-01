/*Package skip provides functions for skipping based on a condition.
 */
package skip

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/ioutil"
	"path"
	"reflect"
	"runtime"
	"strings"

	"github.com/pkg/errors"
)

type skipT interface {
	Skip(args ...interface{})
	Log(args ...interface{})
}

// If skips the test if the check function returns true. The skip message will
// contain the name of the check function. Extra message text can be passed as a
// format string with args
func If(t skipT, check func() bool, msgAndArgs ...interface{}) {
	if check() {
		t.Skip(formatWithCustomMessage(
			getFunctionName(check),
			formatMessage(msgAndArgs...)))
	}
}

func getFunctionName(function func() bool) string {
	funcPath := runtime.FuncForPC(reflect.ValueOf(function).Pointer()).Name()
	return strings.SplitN(path.Base(funcPath), ".", 2)[1]
}

// IfCondition skips the test if the condition is true. The skip message will
// contain the source of the expression passed as the condition. Extra message
// text can be passed as a format string with args.
func IfCondition(t skipT, condition bool, msgAndArgs ...interface{}) {
	if !condition {
		return
	}
	source, err := getConditionSource()
	if err != nil {
		t.Log(err.Error())
		t.Skip(formatMessage(msgAndArgs...))
	}
	t.Skip(formatWithCustomMessage(source, formatMessage(msgAndArgs...)))
}

func getConditionSource() (string, error) {
	const callstackIndex = 3
	lines, err := getSourceLine(callstackIndex)
	if err != nil {
		return "", err
	}

	for i := range lines {
		source := strings.Join(lines[len(lines)-i-1:], "\n")
		node, err := parser.ParseExpr(source)
		if err == nil {
			return getConditionArgFromAST(node)
		}
	}
	return "", errors.Wrapf(err, "failed to parse source")
}

// maxContextLines is the maximum number of lines to scan for a complete
// skip.If() statement
const maxContextLines = 10

// getSourceLines returns the source line which called skip.If() along with a
// few preceding lines. To properly parse the AST a complete statement is
// required, and that statement may be split across multiple lines, so include
// up to maxContextLines.
func getSourceLine(stackIndex int) ([]string, error) {
	_, filename, line, ok := runtime.Caller(stackIndex)
	if !ok {
		return nil, errors.New("failed to get caller info")
	}

	raw, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read source file: %s", filename)
	}

	lines := strings.Split(string(raw), "\n")
	if len(lines) < line {
		return nil, errors.Errorf("file %s does not have line %d", filename, line)
	}
	firstLine := line - maxContextLines
	if firstLine < 0 {
		firstLine = 0
	}
	return lines[firstLine:line], nil
}

func getConditionArgFromAST(node ast.Expr) (string, error) {
	switch expr := node.(type) {
	case *ast.CallExpr:
		buf := new(bytes.Buffer)
		err := format.Node(buf, token.NewFileSet(), expr.Args[1])
		return buf.String(), err
	}
	return "", errors.New("unexpected ast")
}

func formatMessage(msgAndArgs ...interface{}) string {
	switch len(msgAndArgs) {
	case 0:
		return ""
	case 1:
		return msgAndArgs[0].(string)
	default:
		return fmt.Sprintf(msgAndArgs[0].(string), msgAndArgs[1:]...)
	}
}

func formatWithCustomMessage(source, custom string) string {
	switch {
	case custom == "":
		return source
	case source == "":
		return custom
	}
	return fmt.Sprintf("%s: %s", source, custom)
}
