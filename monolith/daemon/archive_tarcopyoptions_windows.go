// +build windows

package daemon

import (
	"github.com/docker/docker/monolith/container"
	"github.com/docker/docker/pkg/archive"
)

func (daemon *Daemon) tarCopyOptions(container *container.Container, noOverwriteDirNonDir bool) (*archive.TarOptions, error) {
	return daemon.defaultTarCopyOptions(noOverwriteDirNonDir), nil
}
