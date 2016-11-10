// +build windows

package daemon

import (
	"sort"

	"github.com/docker/docker/container"
	"github.com/docker/docker/volume"
)

var (
	// maximumBandwidth is the maximum bandwidth of a volume
	maximumBandwidth = "volume.maximum_bandwidth"
	// maximumIOPS is the maximum IO per second of a volume
	maximumIOPS = "volume.maximum_iops"
)

// setupMounts configures the mount points for a container by appending each
// of the configured mounts on the container to the OCI mount structure
// which will ultimately be passed into the oci runtime during container creation.
// It also ensures each of the mounts are lexographically sorted.

// BUGBUG TODO Windows containerd. This would be much better if it returned
// an array of runtime spec mounts, not container mounts. Then no need to
// do multiple transitions.

func (daemon *Daemon) setupMounts(c *container.Container) ([]container.Mount, error) {
	var mnts []container.Mount
	for _, mount := range c.MountPoints { // type is volume.MountPoint
		if err := daemon.lazyInitializeVolume(c.ID, mount); err != nil {
			return nil, err
		}
		s, err := mount.Setup(c.MountLabel, 0, 0)
		if err != nil {
			return nil, err
		}

		mnts = append(mnts, container.Mount{
			Source:       s,
			Destination:  mount.Destination,
			Writable:     mount.RW,
			MaxBandwidth: mount.MaxBandwidth,
			MaxIOps:      mount.MaxIOps,
		})
	}

	sort.Sort(mounts(mnts))
	return mnts, nil
}

// setBindModeIfNull is platform specific processing which is a no-op on
// Windows.
func setBindModeIfNull(bind *volume.MountPoint) {
	return
}
