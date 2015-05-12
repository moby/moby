package daemon

import (
	"io"

	"github.com/docker/docker/pkg/archive"
)

func (daemon *Daemon) ContainerCopyOut(name string, res string, pause bool) (io.ReadCloser, error) {
	container, err := daemon.Get(name)
	if err != nil {
		return nil, err
	}

	if res[0] == '/' {
		res = res[1:]
	}
	return container.GetFile(res)
}

func (daemon *Daemon) ContainerCopyIn(name, to string, pause bool, data archive.ArchiveReader) error {
	container, err := daemon.Get(name)
	if err != nil {
		return err
	}

	if to[0] == '/' {
		to = to[1:]
	}

	if pause && !container.IsPaused() {
		container.Pause()
		defer container.Unpause()
	}
	return container.PutFile(to, data)
}
