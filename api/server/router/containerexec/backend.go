package containerexec

import (
	"io"

	"github.com/docker/docker/daemon"
	"github.com/docker/docker/runconfig"
)

// Backend has all the methods for container Exec operations
type Backend interface {
	ContainerExecCreate(config *runconfig.ExecConfig) (string, error)
	ContainerExecInspect(id string) (*daemon.ExecConfig, error)
	ContainerExecResize(name string, height, width int) error
	ContainerExecStart(name string, stdin io.ReadCloser, stdout io.Writer, stderr io.Writer) error
	ExecExists(name string) (bool, error)
}
