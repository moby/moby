// TODO(thaJeztah): remove once we are a module; the go:build directive prevents go from downgrading language version to go1.16:
//go:build go1.23

package platform

import (
	"os"
	"strconv"
	"strings"
	"sync"
)

// possibleCPUs returns the set of possible CPUs on the host (which is
// equal or larger to the number of CPUs currently online). The returned
// set may be a single number ({0}), or a continuous range ({0,1,2,3}), or
// a non-continuous range ({0,1,2,3,12,13,14,15})
//
// Returns nil on errors. Assume CPUs are 0 -> runtime.NumCPU() in that case.
var possibleCPUs = sync.OnceValue(func() []int {
	data, err := os.ReadFile("/sys/devices/system/cpu/possible")
	if err != nil {
		return nil
	}
	content := strings.TrimSpace(string(data))
	return parsePossibleCPUs(content)
})

func parsePossibleCPUs(content string) []int {
	ranges := strings.Split(content, ",")

	var cpus []int
	for _, r := range ranges {
		// Each entry is either a single number (e.g., "0") or a continuous range
		// (e.g., "0-3").
		if rStart, rEnd, ok := strings.Cut(r, "-"); !ok {
			cpu, err := strconv.Atoi(rStart)
			if err != nil {
				return nil
			}
			cpus = append(cpus, cpu)
		} else {
			var start, end int
			start, err := strconv.Atoi(rStart)
			if err != nil {
				return nil
			}
			end, err = strconv.Atoi(rEnd)
			if err != nil {
				return nil
			}
			for i := start; i <= end; i++ {
				cpus = append(cpus, i)
			}
		}
	}

	return cpus
}
