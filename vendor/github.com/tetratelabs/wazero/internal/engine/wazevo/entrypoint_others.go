//go:build (!arm64 && !amd64) || tinygo

package wazevo

import (
	"runtime"
)

func entrypoint(preambleExecutable, functionExecutable *byte, executionContextPtr uintptr, moduleContextPtr *byte, paramResultStackPtr *uint64, goAllocatedStackSlicePtr uintptr) {
	panic(runtime.GOARCH)
}

func afterGoFunctionCallEntrypoint(executable *byte, executionContextPtr uintptr, stackPointer, framePointer uintptr) {
	panic(runtime.GOARCH)
}
