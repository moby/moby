package sysinfo // import "github.com/docker/docker/pkg/sysinfo"

import (
	"runtime"
)

// NumCPU returns the number of CPUs. It's the equivalent of [runtime.NumCPU].
//
// Deprecated: Use [runtime.NumCPU] instead. It will be removed in the next release.
func NumCPU() int {
	return runtime.NumCPU()
}
