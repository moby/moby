package platform

import (
	"context"
	"runtime"
	"sync"

	"github.com/containerd/log"
)

var (
	arch     string
	onceArch sync.Once
)

// Architecture returns the runtime architecture of the process.
//
// Unlike [runtime.GOARCH] (which refers to the compiler platform),
// Architecture refers to the running platform.
//
// For example, Architecture reports "x86_64" as architecture, even
// when running a "linux/386" compiled binary on "linux/amd64" hardware.
func Architecture() string {
	onceArch.Do(func() {
		var err error
		arch, err = runtimeArchitecture()
		if err != nil {
			log.G(context.TODO()).WithError(err).Error("Could not read system architecture info")
		}
	})
	return arch
}

// PossibleCPU returns the set of possible CPUs on the host (which is equal or
// larger to the number of CPUs currently online). The returned set may be a
// single CPU number ({0}), or a continuous range of CPU numbers ({0,1,2,3}), or
// a non-continuous range of CPU numbers ({0,1,2,3,12,13,14,15}).
func PossibleCPU() []int {
	if ncpu := possibleCPUs(); ncpu != nil {
		return ncpu
	}

	// Fallback in case possibleCPUs() fails.
	var cpus []int
	ncpu := runtime.NumCPU()
	for i := 0; i <= ncpu; i++ {
		cpus = append(cpus, i)
	}

	return cpus
}
