// +build !windows

package daemon

import (
	"os"
	"sort"
	"strconv"

	"github.com/docker/docker/container"
	"github.com/docker/docker/volume"
)

// setupMounts iterates through each of the mount points for a container and
// calls Setup() on each. It also looks to see if is a network mount such as
// /etc/resolv.conf, and if it is not, appends it to the array of mounts.
func (daemon *Daemon) setupMounts(c *container.Container) ([]container.Mount, error) {
	var mounts []container.Mount
	for _, m := range c.MountPoints {
		if err := daemon.lazyInitializeVolume(c.ID, m); err != nil {
			return nil, err
		}
		path, err := m.Setup(c.MountLabel)
		if err != nil {
			return nil, err
		}
		if !c.TrySetNetworkMount(m.Destination, path) {
			mnt := container.Mount{
				Source:      path,
				Destination: m.Destination,
				Writable:    m.RW,
				Propagation: m.Propagation,
			}
			if m.Volume != nil {
				attributes := map[string]string{
					"driver":      m.Volume.DriverName(),
					"container":   c.ID,
					"destination": m.Destination,
					"read/write":  strconv.FormatBool(m.RW),
					"propagation": m.Propagation,
				}
				daemon.LogVolumeEvent(m.Volume.Name(), "mount", attributes)
			}
			mounts = append(mounts, mnt)
		}
	}

	mounts = sortMounts(mounts)
	netMounts := c.NetworkMounts()
	// if we are going to mount any of the network files from container
	// metadata, the ownership must be set properly for potential container
	// remapped root (user namespaces)
	rootUID, rootGID := daemon.GetRemappedUIDGID()
	for _, mount := range netMounts {
		if err := os.Chown(mount.Source, rootUID, rootGID); err != nil {
			return nil, err
		}
	}
	return append(mounts, netMounts...), nil
}

// sortMounts sorts an array of mounts in lexicographic order. This ensure that
// when mounting, the mounts don't shadow other mounts. For example, if mounting
// /etc and /etc/resolv.conf, /etc/resolv.conf must not be mounted first.
func sortMounts(m []container.Mount) []container.Mount {
	sort.Sort(mounts(m))
	return m
}

// setBindModeIfNull is platform specific processing to ensure the
// shared mode is set to 'z' if it is null. This is called in the case
// of processing a named volume and not a typical bind.
func setBindModeIfNull(bind *volume.MountPoint) *volume.MountPoint {
	if bind.Mode == "" {
		bind.Mode = "z"
	}
	return bind
}
