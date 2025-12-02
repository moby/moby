// Package sys includes constants and types used by both public and internal APIs.
package sys

import (
	"context"
	"fmt"
)

// These two special exit codes are reserved by wazero for context Cancel and Timeout integrations.
// The assumption here is that well-behaving Wasm programs won't use these two exit codes.
const (
	// ExitCodeContextCanceled corresponds to context.Canceled and returned by ExitError.ExitCode in that case.
	ExitCodeContextCanceled uint32 = 0xffffffff
	// ExitCodeDeadlineExceeded corresponds to context.DeadlineExceeded and returned by ExitError.ExitCode in that case.
	ExitCodeDeadlineExceeded uint32 = 0xefffffff
)

// ExitError is returned to a caller of api.Function when api.Module CloseWithExitCode was invoked,
// or context.Context passed to api.Function Call was canceled or reached the Timeout.
//
// ExitCode zero value means success while any other value is an error.
//
// Here's an example of how to get the exit code:
//
//	main := module.ExportedFunction("main")
//	if err := main(ctx); err != nil {
//		if exitErr, ok := err.(*sys.ExitError); ok {
//			// This means your module exited with non-zero code!
//		}
//	--snip--
//
// Note: While possible the reason of this was "proc_exit" from "wasi_snapshot_preview1", it could be from other host
// functions, for example an AssemblyScript's abort handler, or any arbitrary caller of CloseWithExitCode.
//
// See https://github.com/WebAssembly/WASI/blob/main/phases/snapshot/docs.md#proc_exit and
// https://www.assemblyscript.org/concepts.html#special-imports
//
// Note: In the case of context cancellation or timeout, the api.Module from which the api.Function created is closed.
type ExitError struct {
	// Note: this is a struct not a uint32 type as it was originally one and
	// we don't want to break call-sites that cast into it.
	exitCode uint32
}

var exitZero = &ExitError{}

func NewExitError(exitCode uint32) *ExitError {
	if exitCode == 0 {
		return exitZero
	}
	return &ExitError{exitCode: exitCode}
}

// ExitCode returns zero on success, and an arbitrary value otherwise.
func (e *ExitError) ExitCode() uint32 {
	return e.exitCode
}

// Error implements the error interface.
func (e *ExitError) Error() string {
	switch e.exitCode {
	case ExitCodeContextCanceled:
		return fmt.Sprintf("module closed with %s", context.Canceled)
	case ExitCodeDeadlineExceeded:
		return fmt.Sprintf("module closed with %s", context.DeadlineExceeded)
	default:
		return fmt.Sprintf("module closed with exit_code(%d)", e.exitCode)
	}
}

// Is allows use via errors.Is
func (e *ExitError) Is(err error) bool {
	if target, ok := err.(*ExitError); ok {
		return e.exitCode == target.exitCode
	}
	if e.exitCode == ExitCodeContextCanceled && err == context.Canceled {
		return true
	}
	if e.exitCode == ExitCodeDeadlineExceeded && err == context.DeadlineExceeded {
		return true
	}
	return false
}
