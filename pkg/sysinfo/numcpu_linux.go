package sysinfo // import "github.com/docker/docker/pkg/sysinfo"

import (
	"runtime"

	"golang.org/x/sys/unix"
)

// numCPU queries the system for the count of threads available
// for use to this process.
//
// Issues two syscalls.
// Returns 0 on errors. Use |runtime.NumCPU| in that case.
func numCPU() int {
	// Gets the affinity mask for a process: The very one invoking this function.
	pid := unix.Getpid()

	var mask unix.CPUSet
	err := unix.SchedGetaffinity(pid, &mask)
	if err != nil {
		return 0
	}

	return mask.Count()
}

// NumCPU returns the number of CPUs which are currently online
func NumCPU() int {
	if ncpu := numCPU(); ncpu > 0 {
		return ncpu
	}
	return runtime.NumCPU()
}
