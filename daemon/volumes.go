package daemon

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Sirupsen/logrus"
	dockererrors "github.com/docker/docker/api/errors"
	"github.com/docker/docker/api/types"
	containertypes "github.com/docker/docker/api/types/container"
	mounttypes "github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/container"
	"github.com/docker/docker/volume"
	"github.com/docker/docker/volume/drivers"
)

var (
	// ErrVolumeReadonly is used to signal an error when trying to copy data into
	// a volume mount that is not writable.
	ErrVolumeReadonly = errors.New("mounted volume is marked read-only")
)

type mounts []container.Mount

// volumeToAPIType converts a volume.Volume to the type used by the Engine API
func volumeToAPIType(v volume.Volume) *types.Volume {
	tv := &types.Volume{
		Name:   v.Name(),
		Driver: v.DriverName(),
	}
	if v, ok := v.(volume.DetailedVolume); ok {
		tv.Labels = v.Labels()
		tv.Options = v.Options()
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

	dereferenceIfExists := func(destination string) {
		if v, ok := mountPoints[destination]; ok {
			logrus.Debugf("Duplicate mount point '%s'", destination)
			if v.Volume != nil {
				daemon.volumes.Dereference(v.Volume, container.ID)
			}
		}
	}

	// 1. Read already configured mount points.
	for destination, point := range container.MountPoints {
		mountPoints[destination] = point
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
				Spec:        m.Spec,
				CopyData:    false,
			}

			if len(cp.Source) == 0 {
				v, err := daemon.volumes.GetWithRef(cp.Name, cp.Driver, container.ID)
				if err != nil {
					return err
				}
				cp.Volume = v
			}
			dereferenceIfExists(cp.Destination)
			mountPoints[cp.Destination] = cp
		}
	}

	// 3. Read bind mounts
	for _, b := range hostConfig.Binds {
		bind, err := volume.ParseMountRaw(b, hostConfig.VolumeDriver)
		if err != nil {
			return err
		}

		// #10618
		_, tmpfsExists := hostConfig.Tmpfs[bind.Destination]
		if binds[bind.Destination] || tmpfsExists {
			return fmt.Errorf("Duplicate mount point '%s'", bind.Destination)
		}

		if bind.Type == mounttypes.TypeVolume {
			// create the volume
			v, err := daemon.volumes.CreateWithRef(bind.Name, bind.Driver, container.ID, nil, nil)
			if err != nil {
				return err
			}
			bind.Volume = v
			bind.Source = v.Path()
			// bind.Name is an already existing volume, we need to use that here
			bind.Driver = v.DriverName()
			if bind.Driver == volume.DefaultDriverName {
				setBindModeIfNull(bind)
			}
		}

		binds[bind.Destination] = true
		dereferenceIfExists(bind.Destination)
		mountPoints[bind.Destination] = bind
	}

	for _, cfg := range hostConfig.Mounts {
		mp, err := volume.ParseMountSpec(cfg)
		if err != nil {
			return dockererrors.NewBadRequestError(err)
		}

		if binds[mp.Destination] {
			return fmt.Errorf("Duplicate mount point '%s'", cfg.Target)
		}

		if mp.Type == mounttypes.TypeVolume {
			var v volume.Volume
			if cfg.VolumeOptions != nil {
				var driverOpts map[string]string
				if cfg.VolumeOptions.DriverConfig != nil {
					driverOpts = cfg.VolumeOptions.DriverConfig.Options
				}
				v, err = daemon.volumes.CreateWithRef(mp.Name, mp.Driver, container.ID, driverOpts, cfg.VolumeOptions.Labels)
			} else {
				v, err = daemon.volumes.CreateWithRef(mp.Name, mp.Driver, container.ID, nil, nil)
			}
			if err != nil {
				return err
			}

			mp.Volume = v
			mp.Name = v.Name()
			mp.Driver = v.DriverName()

			// only use the cached path here since getting the path is not necessary right now and calling `Path()` may be slow
			if cv, ok := v.(interface {
				CachedPath() string
			}); ok {
				mp.Source = cv.CachedPath()
			}
		}

		binds[mp.Destination] = true
		dereferenceIfExists(mp.Destination)
		mountPoints[mp.Destination] = mp
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

func backportMountSpec(container *container.Container) error {
	for target, m := range container.MountPoints {
		if m.Spec.Type != "" {
			// if type is set on even one mount, no need to migrate
			return nil
		}
		if m.Name != "" {
			m.Type = mounttypes.TypeVolume
			m.Spec.Type = mounttypes.TypeVolume

			// make sure this is not an anonymous volume before setting the spec source
			if _, exists := container.Config.Volumes[target]; !exists {
				m.Spec.Source = m.Name
			}
			if container.HostConfig.VolumeDriver != "" {
				m.Spec.VolumeOptions = &mounttypes.VolumeOptions{
					DriverConfig: &mounttypes.Driver{Name: container.HostConfig.VolumeDriver},
				}
			}
			if strings.Contains(m.Mode, "nocopy") {
				if m.Spec.VolumeOptions == nil {
					m.Spec.VolumeOptions = &mounttypes.VolumeOptions{}
				}
				m.Spec.VolumeOptions.NoCopy = true
			}
		} else {
			m.Type = mounttypes.TypeBind
			m.Spec.Type = mounttypes.TypeBind
			m.Spec.Source = m.Source
			if m.Propagation != "" {
				m.Spec.BindOptions = &mounttypes.BindOptions{
					Propagation: m.Propagation,
				}
			}
		}

		m.Spec.Target = m.Destination
		if !m.RW {
			m.Spec.ReadOnly = true
		}
	}
	return container.ToDiskLocking()
}

func (daemon *Daemon) traverseLocalVolumes(fn func(volume.Volume) error) error {
	localVolumeDriver, err := volumedrivers.GetDriver(volume.DefaultDriverName)
	if err != nil {
		return fmt.Errorf("can't retrieve local volume driver: %v", err)
	}
	vols, err := localVolumeDriver.List()
	if err != nil {
		return fmt.Errorf("can't retrieve local volumes: %v", err)
	}

	for _, v := range vols {
		name := v.Name()
		_, err := daemon.volumes.Get(name)
		if err != nil {
			logrus.Warnf("failed to retrieve volume %s from store: %v", name, err)
		}

		err = fn(v)
		if err != nil {
			return err
		}
	}

	return nil
}
