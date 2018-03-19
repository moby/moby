/*Package skip provides functions for skipping based on a condition.
 */
package skip

import (
	"fmt"
	"path"
	"reflect"
	"runtime"
	"strings"

	"github.com/gotestyourself/gotestyourself/internal/format"
	"github.com/gotestyourself/gotestyourself/internal/source"
)

type skipT interface {
	Skip(args ...interface{})
	Log(args ...interface{})
}

type helperT interface {
	Helper()
}

// BoolOrCheckFunc can be a bool or func() bool, other types will panic
type BoolOrCheckFunc interface{}

// If skips the test if the check function returns true. The skip message will
// contain the name of the check function. Extra message text can be passed as a
// format string with args
func If(t skipT, condition BoolOrCheckFunc, msgAndArgs ...interface{}) {
	if ht, ok := t.(helperT); ok {
		ht.Helper()
	}
	switch check := condition.(type) {
	case bool:
		ifCondition(t, check, msgAndArgs...)
	case func() bool:
		if check() {
			t.Skip(format.WithCustomMessage(getFunctionName(check), msgAndArgs...))
		}
	default:
		panic(fmt.Sprintf("invalid type for condition arg: %T", check))
	}
}

func getFunctionName(function func() bool) string {
	funcPath := runtime.FuncForPC(reflect.ValueOf(function).Pointer()).Name()
	return strings.SplitN(path.Base(funcPath), ".", 2)[1]
}

// IfCondition skips the test if the condition is true. The skip message will
// contain the source of the expression passed as the condition. Extra message
// text can be passed as a format string with args.
//
// Deprecated: Use If() which now accepts bool arguments
func IfCondition(t skipT, condition bool, msgAndArgs ...interface{}) {
	if ht, ok := t.(helperT); ok {
		ht.Helper()
	}
	ifCondition(t, condition, msgAndArgs...)
}

func ifCondition(t skipT, condition bool, msgAndArgs ...interface{}) {
	if ht, ok := t.(helperT); ok {
		ht.Helper()
	}
	if !condition {
		return
	}
	const (
		stackIndex = 2
		argPos     = 1
	)
	source, err := source.FormattedCallExprArg(stackIndex, argPos)
	if err != nil {
		t.Log(err.Error())
		t.Skip(format.Message(msgAndArgs...))
	}
	t.Skip(format.WithCustomMessage(source, msgAndArgs...))
}
