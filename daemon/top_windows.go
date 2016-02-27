package daemon

import (
	"fmt"

	"github.com/docker/engine-api/types"
)

// ContainerTop is not supported on Windows and returns an error.
func (daemon *Daemon) ContainerTop(name string, psArgs string) (*types.ContainerProcessList, error) {
	return nil, fmt.Errorf("Top is not supported on Windows")
}
