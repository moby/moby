package daemon

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/container"
	"github.com/docker/docker/volume"
	"github.com/docker/engine-api/types"
	containertypes "github.com/docker/engine-api/types/container"
)

var (
	// ErrVolumeReadonly is used to signal an error when trying to copy data into
	// a volume mount that is not writable.
	ErrVolumeReadonly = errors.New("mounted volume is marked read-only")
)

type mounts []container.Mount

// volumeToAPIType converts a volume.Volume to the type used by the remote API
func volumeToAPIType(v volume.Volume) *types.Volume {
	tv := &types.Volume{
		Name:   v.Name(),
		Driver: v.DriverName(),
	}
	if v, ok := v.(volume.LabeledVolume); ok {
		tv.Labels = v.Labels()
	}

	if v, ok := v.(volume.ScopedVolume); ok {
		tv.Scope = v.Scope()
	}
	return tv
}

// Len returns the number of mounts. Used in sorting.
func (m mounts) Len() int {
	return len(m)
}

// Less returns true if the number of parts (a/b/c would be 3 parts) in the
// mount indexed by parameter 1 is less than that of the mount indexed by
// parameter 2. Used in sorting.
func (m mounts) Less(i, j int) bool {
	return m.parts(i) < m.parts(j)
}

// Swap swaps two items in an array of mounts. Used in sorting
func (m mounts) Swap(i, j int) {
	m[i], m[j] = m[j], m[i]
}

// parts returns the number of parts in the destination of a mount. Used in sorting.
func (m mounts) parts(i int) int {
	return strings.Count(filepath.Clean(m[i].Destination), string(os.PathSeparator))
}

// registerMountPoints initializes the container mount points with the configured volumes and bind mounts.
// It follows the next sequence to decide what to mount in each final destination:
//
// 1. Select the previously configured mount points for the containers, if any.
// 2. Select the volumes mounted from another containers. Overrides previously configured mount point destination.
// 3. Select the bind mounts set by the client. Overrides previously configured mount point destinations.
// 4. Cleanup old volumes that are about to be reassigned.
func (daemon *Daemon) registerMountPoints(container *container.Container, hostConfig *containertypes.HostConfig) (retErr error) {
	binds := map[string]bool{}
	mountPoints := map[string]*volume.MountPoint{}
	defer func() {
		// clean up the container mountpoints once return with error
		if retErr != nil {
			for _, m := range mountPoints {
				if m.Volume == nil {
					continue
				}
				daemon.volumes.Dereference(m.Volume, container.ID)
			}
		}
	}()

	// 1. Read already configured mount points.
	for name, point := range container.MountPoints {
		mountPoints[name] = point
	}

	// 2. Read volumes from other containers.
	for _, v := range hostConfig.VolumesFrom {
		containerID, mode, err := volume.ParseVolumesFrom(v)
		if err != nil {
			return err
		}

		c, err := daemon.GetContainer(containerID)
		if err != nil {
			return err
		}

		for _, m := range c.MountPoints {
			cp := &volume.MountPoint{
				Name:        m.Name,
				Source:      m.Source,
				RW:          m.RW && volume.ReadWrite(mode),
				Driver:      m.Driver,
				Destination: m.Destination,
				Propagation: m.Propagation,
				Named:       m.Named,
			}

			if len(cp.Source) == 0 {
				v, err := daemon.volumes.GetWithRef(cp.Name, cp.Driver, container.ID)
				if err != nil {
					return err
				}
				cp.Volume = v
			}

			mountPoints[cp.Destination] = cp
		}
	}

	// 3. Read bind mounts
	for _, b := range hostConfig.Binds {
		// #10618
		bind, err := volume.ParseMountSpec(b, hostConfig.VolumeDriver)
		if err != nil {
			return err
		}

		if binds[bind.Destination] {
			return fmt.Errorf("Duplicate mount point '%s'", bind.Destination)
		}

		if len(bind.Name) > 0 {
			// create the volume
			v, err := daemon.volumes.CreateWithRef(bind.Name, bind.Driver, container.ID, nil, nil)
			if err != nil {
				return err
			}
			bind.Volume = v
			bind.Source = v.Path()
			// bind.Name is an already existing volume, we need to use that here
			bind.Driver = v.DriverName()
			bind.Named = true
			if bind.Driver == "local" {
				bind = setBindModeIfNull(bind)
			}
		}

		binds[bind.Destination] = true
		mountPoints[bind.Destination] = bind
	}

	container.Lock()

	// 4. Cleanup old volumes that are about to be reassigned.
	for _, m := range mountPoints {
		if m.BackwardsCompatible() {
			if mp, exists := container.MountPoints[m.Destination]; exists && mp.Volume != nil {
				daemon.volumes.Dereference(mp.Volume, container.ID)
			}
		}
	}
	container.MountPoints = mountPoints

	container.Unlock()

	return nil
}

// lazyInitializeVolume initializes a mountpoint's volume if needed.
// This happens after a daemon restart.
func (daemon *Daemon) lazyInitializeVolume(containerID string, m *volume.MountPoint) error {
	if len(m.Driver) > 0 && m.Volume == nil {
		v, err := daemon.volumes.GetWithRef(m.Name, m.Driver, containerID)
		if err != nil {
			return err
		}
		m.Volume = v
	}
	return nil
}
