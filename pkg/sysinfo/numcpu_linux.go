package sysinfo // import "github.com/docker/docker/pkg/sysinfo"

import "golang.org/x/sys/unix"

// numCPU queries the system for the count of threads available
// for use to this process.
//
// Returns 0 on errors. Use |runtime.NumCPU| in that case.
func numCPU() int {
	// Gets the affinity mask for a process: The very one invoking this function.
	var mask unix.CPUSet
	err := unix.SchedGetaffinity(0, &mask)
	if err != nil {
		return 0
	}
	return mask.Count()
}
