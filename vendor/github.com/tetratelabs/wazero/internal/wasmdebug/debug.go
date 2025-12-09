// Package wasmdebug contains utilities used to give consistent search keys between stack traces and error messages.
// Note: This is named wasmdebug to avoid conflicts with the normal go module.
// Note: This only imports "api" as importing "wasm" would create a cyclic dependency.
package wasmdebug

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/wasmruntime"
	"github.com/tetratelabs/wazero/sys"
)

// FuncName returns the naming convention of "moduleName.funcName".
//
//   - moduleName is the possibly empty name the module was instantiated with.
//   - funcName is the name in the Custom Name section.
//   - funcIdx is the position in the function index, prefixed with
//     imported functions.
//
// Note: "moduleName.$funcIdx" is used when the funcName is empty, as commonly
// the case in TinyGo.
func FuncName(moduleName, funcName string, funcIdx uint32) string {
	var ret strings.Builder

	// Start module.function
	ret.WriteString(moduleName)
	ret.WriteByte('.')
	if funcName == "" {
		ret.WriteByte('$')
		ret.WriteString(strconv.Itoa(int(funcIdx)))
	} else {
		ret.WriteString(funcName)
	}

	return ret.String()
}

// signature returns a formatted signature similar to how it is defined in Go.
//
// * paramTypes should be from wasm.FunctionType
// * resultTypes should be from wasm.FunctionType
// TODO: add paramNames
func signature(funcName string, paramTypes []api.ValueType, resultTypes []api.ValueType) string {
	var ret strings.Builder
	ret.WriteString(funcName)

	// Start params
	ret.WriteByte('(')
	paramCount := len(paramTypes)
	switch paramCount {
	case 0:
	case 1:
		ret.WriteString(api.ValueTypeName(paramTypes[0]))
	default:
		ret.WriteString(api.ValueTypeName(paramTypes[0]))
		for _, vt := range paramTypes[1:] {
			ret.WriteByte(',')
			ret.WriteString(api.ValueTypeName(vt))
		}
	}
	ret.WriteByte(')')

	// Start results
	resultCount := len(resultTypes)
	switch resultCount {
	case 0:
	case 1:
		ret.WriteByte(' ')
		ret.WriteString(api.ValueTypeName(resultTypes[0]))
	default: // As this is used for errors, don't panic if there are multiple returns, even if that's invalid!
		ret.WriteByte(' ')
		ret.WriteByte('(')
		ret.WriteString(api.ValueTypeName(resultTypes[0]))
		for _, vt := range resultTypes[1:] {
			ret.WriteByte(',')
			ret.WriteString(api.ValueTypeName(vt))
		}
		ret.WriteByte(')')
	}

	return ret.String()
}

// ErrorBuilder helps build consistent errors, particularly adding a WASM stack trace.
//
// AddFrame should be called beginning at the frame that panicked until no more frames exist. Once done, call Format.
type ErrorBuilder interface {
	// AddFrame adds the next frame.
	//
	// * funcName should be from FuncName
	// * paramTypes should be from wasm.FunctionType
	// * resultTypes should be from wasm.FunctionType
	// * sources is the source code information for this frame and can be empty.
	//
	// Note: paramTypes and resultTypes are present because signature misunderstanding, mismatch or overflow are common.
	AddFrame(funcName string, paramTypes, resultTypes []api.ValueType, sources []string)

	// FromRecovered returns an error with the wasm stack trace appended to it.
	FromRecovered(recovered interface{}) error
}

func NewErrorBuilder() ErrorBuilder {
	return &stackTrace{}
}

type stackTrace struct {
	// frameCount is the number of stack frame currently pushed into lines.
	frameCount int
	// lines contains the stack trace and possibly the inlined source code information.
	lines []string
}

// GoRuntimeErrorTracePrefix is the prefix coming before the Go runtime stack trace included in the face of runtime.Error.
// This is exported for testing purpose.
const GoRuntimeErrorTracePrefix = "Go runtime stack trace:"

func (s *stackTrace) FromRecovered(recovered interface{}) error {
	if false {
		debug.PrintStack()
	}

	if exitErr, ok := recovered.(*sys.ExitError); ok { // Don't wrap an exit error!
		return exitErr
	}

	stack := strings.Join(s.lines, "\n\t")

	// If the error was internal, don't mention it was recovered.
	if wasmErr, ok := recovered.(*wasmruntime.Error); ok {
		return fmt.Errorf("wasm error: %w\nwasm stack trace:\n\t%s", wasmErr, stack)
	}

	// If we have a runtime.Error, something severe happened which should include the stack trace. This could be
	// a nil pointer from wazero or a user-defined function from HostModuleBuilder.
	if runtimeErr, ok := recovered.(runtime.Error); ok {
		return fmt.Errorf("%w (recovered by wazero)\nwasm stack trace:\n\t%s\n\n%s\n%s",
			runtimeErr, stack, GoRuntimeErrorTracePrefix, debug.Stack())
	}

	// At this point we expect the error was from a function defined by HostModuleBuilder that intentionally called panic.
	if runtimeErr, ok := recovered.(error); ok { // e.g. panic(errors.New("whoops"))
		return fmt.Errorf("%w (recovered by wazero)\nwasm stack trace:\n\t%s", runtimeErr, stack)
	} else { // e.g. panic("whoops")
		return fmt.Errorf("%v (recovered by wazero)\nwasm stack trace:\n\t%s", recovered, stack)
	}
}

// MaxFrames is the maximum number of frames to include in the stack trace.
const MaxFrames = 30

// AddFrame implements ErrorBuilder.AddFrame
func (s *stackTrace) AddFrame(funcName string, paramTypes, resultTypes []api.ValueType, sources []string) {
	if s.frameCount == MaxFrames {
		return
	}
	s.frameCount++
	sig := signature(funcName, paramTypes, resultTypes)
	s.lines = append(s.lines, sig)
	for _, source := range sources {
		s.lines = append(s.lines, "\t"+source)
	}
	if s.frameCount == MaxFrames {
		s.lines = append(s.lines, "... maybe followed by omitted frames")
	}
}
