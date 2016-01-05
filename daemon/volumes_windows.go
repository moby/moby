// +build windows

package daemon

import (
	"sort"

	"github.com/docker/docker/container"
	"github.com/docker/docker/daemon/execdriver"
	derr "github.com/docker/docker/errors"
	"github.com/docker/docker/volume"
)

// setupMounts configures the mount points for a container by appending each
// of the configured mounts on the container to the execdriver mount structure
// which will ultimately be passed into the exec driver during container creation.
// It also ensures each of the mounts are lexographically sorted.
func (daemon *Daemon) setupMounts(container *container.Container) ([]execdriver.Mount, error) {
	var mnts []execdriver.Mount
	for _, mount := range container.MountPoints { // type is volume.MountPoint
		// Lazy initialize m.Volume if needed.  This happens after a daemon restart
		if len(m.Driver) > 0 && m.Volume == nil {
			v, err := daemon.createVolume(m.Name, m.Driver, nil)
			if err != nil {
				return nil, err
			}
			m.Volume = v
		}
		// If there is no source, take it from the volume path
		s := mount.Source
		if s == "" && mount.Volume != nil {
			s = mount.Volume.Path()
		}
		if s == "" {
			return nil, derr.ErrorCodeVolumeNoSourceForMount.WithArgs(mount.Name, mount.Driver, mount.Destination)
		}
		mnts = append(mnts, execdriver.Mount{
			Source:      s,
			Destination: mount.Destination,
			Writable:    mount.RW,
		})
	}

	sort.Sort(mounts(mnts))
	return mnts, nil
}

// setBindModeIfNull is platform specific processing which is a no-op on
// Windows.
func setBindModeIfNull(bind *volume.MountPoint) *volume.MountPoint {
	return bind
}
