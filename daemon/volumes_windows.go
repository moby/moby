package daemon // import "github.com/docker/docker/daemon"

import (
	"context"
	"sort"

	"github.com/containerd/log"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/container"
	"github.com/docker/docker/internal/cleanups"
	"github.com/docker/docker/pkg/idtools"
	volumemounts "github.com/docker/docker/volume/mounts"
)

// setupMounts configures the mount points for a container by appending each
// of the configured mounts on the container to the OCI mount structure
// which will ultimately be passed into the oci runtime during container creation.
// It also ensures each of the mounts are lexicographically sorted.
//
// The cleanup function should be called as soon as the container has been
// started.
//
// BUGBUG TODO Windows containerd. This would be much better if it returned
// an array of runtime spec mounts, not container mounts. Then no need to
// do multiple transitions.
func (daemon *Daemon) setupMounts(ctx context.Context, c *container.Container) ([]container.Mount, func(context.Context) error, error) {
	mntCleanups := cleanups.Composite{}
	defer func() {
		if err := mntCleanups.Call(context.WithoutCancel(ctx)); err != nil {
			log.G(ctx).WithError(err).Warn("failed to cleanup temporary mounts created by MountPoint.Setup")
		}
	}()

	var mnts []container.Mount
	for _, mp := range c.MountPoints { // type is volumemounts.MountPoint
		if err := daemon.lazyInitializeVolume(c.ID, mp); err != nil {
			return nil, nil, err
		}
		s, c, err := mp.Setup(ctx, c.MountLabel, idtools.Identity{}, nil)
		if err != nil {
			return nil, nil, err
		}
		mntCleanups.Add(c)

		mnts = append(mnts, container.Mount{
			Source:      s,
			Destination: mp.Destination,
			Writable:    mp.RW,
		})
	}

	sort.Sort(mounts(mnts))
	return mnts, mntCleanups.Release(), nil
}

// setBindModeIfNull is platform specific processing which is a no-op on
// Windows.
func setBindModeIfNull(bind *volumemounts.MountPoint) {
	return
}

func (daemon *Daemon) validateBindDaemonRoot(m mount.Mount) (bool, error) {
	return false, nil
}
