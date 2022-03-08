//go:build !windows
// +build !windows

package daemon // import "github.com/docker/docker/daemon"

import (
	"fmt"
	"os/user"
	"strconv"

	"github.com/docker/docker/container"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/idtools"
)

func (daemon *Daemon) tarCopyOptions(container *container.Container, noOverwriteDirNonDir bool) (*archive.TarOptions, error) {
	if container.Config.User == "" {
		return daemon.defaultTarCopyOptions(noOverwriteDirNonDir), nil
	}

	user, err := user.Lookup(container.Config.User)
	if err != nil {
		return nil, err
	}

	uid, err := strconv.Atoi(user.Uid)
	if err != nil {
		// Should always be numeric on POSIX systems.
		panic(fmt.Errorf("uid is not numeric: %w", err))
	}
	gid, err := strconv.Atoi(user.Gid)
	if err != nil {
		panic(fmt.Errorf("gid is not numeric: %w", err))
	}
	identity := idtools.Identity{UID: uid, GID: gid}

	return &archive.TarOptions{
		NoOverwriteDirNonDir: noOverwriteDirNonDir,
		ChownOpts:            &identity,
	}, nil
}
