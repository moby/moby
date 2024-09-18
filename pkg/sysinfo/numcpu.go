package sysinfo // import "github.com/docker/docker/pkg/sysinfo"

import (
	"runtime"
)

// NumCPU returns the number of CPUs. On Linux and Windows, it returns
// the number of CPUs which are currently online. On other platforms,
// it's the equivalent of [runtime.NumCPU].
func NumCPU() int {
	if ncpu := numCPU(); ncpu > 0 {
		return ncpu
	}
	return runtime.NumCPU()
}
