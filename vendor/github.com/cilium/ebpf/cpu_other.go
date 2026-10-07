//go:build !windows

package ebpf

import (
	"sync"

	"github.com/cilium/ebpf/internal/linux"
)

var possibleCPU = sync.OnceValues(func() (int, error) {
	return linux.ParseCPUsFromFile("/sys/devices/system/cpu/possible")
})
