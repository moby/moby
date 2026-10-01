package ebpf

import (
	"sync"

	"golang.org/x/sys/windows"
)

var possibleCPU = sync.OnceValues(func() (int, error) {
	return int(windows.GetMaximumProcessorCount(windows.ALL_PROCESSOR_GROUPS)), nil
})
