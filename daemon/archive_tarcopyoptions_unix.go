// +build !windows

package daemon // import "github.com/docker/docker/daemon"

import (
	"github.com/docker/docker/container"
	"github.com/docker/docker/pkg/archive"
)

func (daemon *Daemon) tarCopyOptions(container *container.Container, noOverwriteDirNonDir bool) (*archive.TarOptions, error) {
	tarOptions := daemon.defaultTarCopyOptions(noOverwriteDirNonDir)
	tarOptions.NoLchown = false
	return tarOptions, nil
}
