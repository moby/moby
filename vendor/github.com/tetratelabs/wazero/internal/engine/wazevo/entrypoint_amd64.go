//go:build amd64 && !tinygo

package wazevo

import _ "unsafe"

// entrypoint is implemented by the backend.
//
//go:linkname entrypoint github.com/tetratelabs/wazero/internal/engine/wazevo/backend/isa/amd64.entrypoint
func entrypoint(preambleExecutable, functionExecutable *byte, executionContextPtr uintptr, moduleContextPtr *byte, paramResultStackPtr *uint64, goAllocatedStackSlicePtr uintptr)

// entrypoint is implemented by the backend.
//
//go:linkname afterGoFunctionCallEntrypoint github.com/tetratelabs/wazero/internal/engine/wazevo/backend/isa/amd64.afterGoFunctionCallEntrypoint
func afterGoFunctionCallEntrypoint(executable *byte, executionContextPtr uintptr, stackPointer, framePointer uintptr)
