package daemon

import (
	"fmt"

	"github.com/docker/docker/api/types"
)

func (daemon *Daemon) ContainerTop(name string, psArgs string) (*types.ContainerProcessList, error) {
	return nil, fmt.Errorf("Top is not supported on Windows")
}
