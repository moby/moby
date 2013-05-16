package docker

import (
	"fmt"
	"github.com/dotcloud/docker/utils"
)

func getKernelVersion() (*utils.KernelVersionInfo, error) {
	return nil, fmt.Errorf("Kernel version detection is not available on darwin")
}
