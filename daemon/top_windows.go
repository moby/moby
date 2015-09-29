package daemon

import (
	"github.com/docker/docker/api/types"
	derr "github.com/docker/docker/errors"
)

// ContainerTop is not supported on Windows and returns an error.
func (daemon *Daemon) ContainerTop(name string, psArgs string) (*types.ContainerProcessList, error) {
	return nil, derr.ErrorCodeNoTop
}
