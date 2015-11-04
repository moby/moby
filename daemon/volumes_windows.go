// +build windows

package daemon

import (
	"sort"

	"github.com/docker/docker/daemon/execdriver"
	derr "github.com/docker/docker/errors"
	"github.com/docker/docker/volume"
)

// setupMounts configures the mount points for a container by appending each
// of the configured mounts on the container to the execdriver mount structure
// which will ultimately be passed into the exec driver during container creation.
// It also ensures each of the mounts are lexographically sorted.
func (daemon *Daemon) setupMounts(container *Container) ([]execdriver.Mount, error) {
	var mnts []execdriver.Mount
	for _, mount := range container.MountPoints { // type is volume.MountPoint
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

// verifyVolumesInfo ports volumes configured for the containers pre docker 1.7.
// As the Windows daemon was not supported before 1.7, this is a no-op
func (daemon *Daemon) verifyVolumesInfo(container *Container) error {
	return nil
}

// setBindModeIfNull is platform specific processing which is a no-op on
// Windows.
func setBindModeIfNull(bind *volume.MountPoint) *volume.MountPoint {
	return bind
}

// configureBackCompatStructures is platform specific processing for
// registering mount points to populate old structures. This is a no-op on Windows.
func configureBackCompatStructures(*Daemon, *Container, map[string]*volume.MountPoint) (map[string]string, map[string]bool) {
	return nil, nil
}

// setBackCompatStructures is a platform specific helper function to set
// backwards compatible structures in the container when registering volumes.
// This is a no-op on Windows.
func setBackCompatStructures(*Container, map[string]string, map[string]bool) {
}
