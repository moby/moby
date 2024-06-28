package daemon // import "github.com/docker/docker/daemon"

import (
	"os"

	"github.com/docker/docker/errdefs"
)

func (daemon *Daemon) ContainerLogsClear(containerName string) error {
	container, err := daemon.GetContainer(containerName)
	if err != nil {
		return errdefs.System(err)
	}

	f, err := os.OpenFile(container.LogPath, os.O_WRONLY, 0755)
	if err != nil {
		return errdefs.System(err)
	}

	defer f.Close()

	err = f.Truncate(0)
	if err != nil {
		return errdefs.System(err)
	}

	return nil
}
