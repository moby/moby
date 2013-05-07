package docker

import (
	"fmt"
)

func getKernelVersion() (*KernelVersionInfo, error) {
	return nil, fmt.Errorf("Kernel version detection is not available on darwin")
}
