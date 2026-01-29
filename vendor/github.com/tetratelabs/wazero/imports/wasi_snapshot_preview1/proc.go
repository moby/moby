package wasi_snapshot_preview1

import (
	"context"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/wasip1"
	"github.com/tetratelabs/wazero/internal/wasm"
	"github.com/tetratelabs/wazero/sys"
)

// procExit is the WASI function named ProcExitName that terminates the
// execution of the module with an exit code. The only successful exit code is
// zero.
//
// # Parameters
//
//   - exitCode: exit code.
//
// See https://github.com/WebAssembly/WASI/blob/main/phases/snapshot/docs.md#proc_exit
var procExit = &wasm.HostFunc{
	ExportName: wasip1.ProcExitName,
	Name:       wasip1.ProcExitName,
	ParamTypes: []api.ValueType{i32},
	ParamNames: []string{"rval"},
	Code:       wasm.Code{GoFunc: api.GoModuleFunc(procExitFn)},
}

func procExitFn(ctx context.Context, mod api.Module, params []uint64) {
	exitCode := uint32(params[0])

	// Ensure other callers see the exit code.
	_ = mod.CloseWithExitCode(ctx, exitCode)

	// Prevent any code from executing after this function. For example, LLVM
	// inserts unreachable instructions after calls to exit.
	// See: https://github.com/emscripten-core/emscripten/issues/12322
	panic(sys.NewExitError(exitCode))
}

// procRaise is stubbed and will never be supported, as it was removed.
//
// See https://github.com/WebAssembly/WASI/pull/136
var procRaise = stubFunction(wasip1.ProcRaiseName, []api.ValueType{i32}, "sig")
