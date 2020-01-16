// +build !windows

package daemon // import "github.com/moby/moby/daemon"

import (
	"github.com/moby/moby/container"
	"github.com/moby/moby/pkg/archive"
	"github.com/moby/moby/pkg/idtools"
)

func (daemon *Daemon) tarCopyOptions(container *container.Container, noOverwriteDirNonDir bool) (*archive.TarOptions, error) {
	if container.Config.User == "" {
		return daemon.defaultTarCopyOptions(noOverwriteDirNonDir), nil
	}

	user, err := idtools.LookupUser(container.Config.User)
	if err != nil {
		return nil, err
	}

	identity := idtools.Identity{UID: user.Uid, GID: user.Gid}

	return &archive.TarOptions{
		NoOverwriteDirNonDir: noOverwriteDirNonDir,
		ChownOpts:            &identity,
	}, nil
}
