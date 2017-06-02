// +build !windows

package daemon

import (
	"github.com/docker/docker/container"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/fsutils"
)

func (daemon *Daemon) tarCopyOptions(container *container.Container, noOverwriteDirNonDir bool) (*archive.TarOptions, error) {
	if container.Config.User == "" {
		return daemon.defaultTarCopyOptions(noOverwriteDirNonDir), nil
	}

	user, err := fsutils.LookupUser(container.Config.User)
	if err != nil {
		return nil, err
	}

	return &archive.TarOptions{
		NoOverwriteDirNonDir: noOverwriteDirNonDir,
		ChownOpts:            &fsutils.IDPair{UID: user.Uid, GID: user.Gid},
	}, nil
}
