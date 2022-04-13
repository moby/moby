package daemon // import "github.com/docker/docker/daemon"

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	mounttypes "github.com/docker/docker/api/types/mount"
	volumetypes "github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/container"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/volume"
	volumemounts "github.com/docker/docker/volume/mounts"
	"github.com/docker/docker/volume/service"
	volumeopts "github.com/docker/docker/volume/service/opts"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var (
	// ErrVolumeReadonly is used to signal an error when trying to copy data into
	// a volume mount that is not writable.
	ErrVolumeReadonly = errors.New("mounted volume is marked read-only")
)

type mounts []container.Mount

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
	mountPoints := map[string]*volumemounts.MountPoint{}
	parser := volumemounts.NewParser()

	ctx := context.TODO()
	defer func() {
		// clean up the container mountpoints once return with error
		if retErr != nil {
			for _, m := range mountPoints {
				if m.Volume == nil {
					continue
				}
				daemon.volumes.Release(ctx, m.Volume.Name(), container.ID)
			}
		}
	}()

	dereferenceIfExists := func(destination string) {
		if v, ok := mountPoints[destination]; ok {
			logrus.Debugf("Duplicate mount point '%s'", destination)
			if v.Volume != nil {
				daemon.volumes.Release(ctx, v.Volume.Name(), container.ID)
			}
		}
	}

	// 1. Read already configured mount points.
	for destination, point := range container.MountPoints {
		mountPoints[destination] = point
	}

	// 2. Read volumes from other containers.
	for _, v := range hostConfig.VolumesFrom {
		containerID, mode, err := parser.ParseVolumesFrom(v)
		if err != nil {
			return errdefs.InvalidParameter(err)
		}

		c, err := daemon.GetContainer(containerID)
		if err != nil {
			return errdefs.InvalidParameter(err)
		}

		for _, m := range c.MountPoints {
			cp := &volumemounts.MountPoint{
				Type:        m.Type,
				Name:        m.Name,
				Source:      m.Source,
				RW:          m.RW && parser.ReadWrite(mode),
				Driver:      m.Driver,
				Destination: m.Destination,
				Propagation: m.Propagation,
				Spec:        m.Spec,
				CopyData:    false,
			}

			if len(cp.Source) == 0 {
				v, err := daemon.volumes.Get(ctx, cp.Name, volumeopts.WithGetDriver(cp.Driver), volumeopts.WithGetReference(container.ID))
				if err != nil {
					return err
				}
				cp.Volume = &volumeWrapper{v: v, s: daemon.volumes}
			}
			dereferenceIfExists(cp.Destination)
			mountPoints[cp.Destination] = cp
		}
	}

	// 3. Read bind mounts
	for _, b := range hostConfig.Binds {
		bind, err := parser.ParseMountRaw(b, hostConfig.VolumeDriver)
		if err != nil {
			return err
		}
		needsSlavePropagation, err := daemon.validateBindDaemonRoot(bind.Spec)
		if err != nil {
			return err
		}
		if needsSlavePropagation {
			bind.Propagation = mount.PropagationRSlave
		}

		// #10618
		_, tmpfsExists := hostConfig.Tmpfs[bind.Destination]
		if binds[bind.Destination] || tmpfsExists {
			return duplicateMountPointError(bind.Destination)
		}

		if bind.Type == mounttypes.TypeVolume {
			// create the volume
			v, err := daemon.volumes.Create(ctx, bind.Name, bind.Driver, volumeopts.WithCreateReference(container.ID))
			if err != nil {
				return err
			}
			bind.Volume = &volumeWrapper{v: v, s: daemon.volumes}
			bind.Source = v.Mountpoint
			// bind.Name is an already existing volume, we need to use that here
			bind.Driver = v.Driver
			if bind.Driver == volume.DefaultDriverName {
				setBindModeIfNull(bind)
			}
		}

		binds[bind.Destination] = true
		dereferenceIfExists(bind.Destination)
		mountPoints[bind.Destination] = bind
	}

	for _, cfg := range hostConfig.Mounts {
		mp, err := parser.ParseMountSpec(cfg)
		if err != nil {
			return errdefs.InvalidParameter(err)
		}
		needsSlavePropagation, err := daemon.validateBindDaemonRoot(mp.Spec)
		if err != nil {
			return err
		}
		if needsSlavePropagation {
			mp.Propagation = mount.PropagationRSlave
		}

		if binds[mp.Destination] {
			return duplicateMountPointError(cfg.Target)
		}

		if mp.Type == mounttypes.TypeVolume {
			var v *volumetypes.Volume
			if cfg.VolumeOptions != nil {
				var driverOpts map[string]string
				if cfg.VolumeOptions.DriverConfig != nil {
					driverOpts = cfg.VolumeOptions.DriverConfig.Options
				}
				v, err = daemon.volumes.Create(ctx,
					mp.Name,
					mp.Driver,
					volumeopts.WithCreateReference(container.ID),
					volumeopts.WithCreateOptions(driverOpts),
					volumeopts.WithCreateLabels(cfg.VolumeOptions.Labels),
				)
			} else {
				v, err = daemon.volumes.Create(ctx, mp.Name, mp.Driver, volumeopts.WithCreateReference(container.ID))
			}
			if err != nil {
				return err
			}

			mp.Volume = &volumeWrapper{v: v, s: daemon.volumes}
			mp.Name = v.Name
			mp.Driver = v.Driver

			// need to selinux-relabel local mounts
			mp.Source = v.Mountpoint
			if mp.Driver == volume.DefaultDriverName {
				setBindModeIfNull(mp)
			}
		}

		if mp.Type == mounttypes.TypeBind && (cfg.BindOptions == nil || !cfg.BindOptions.CreateMountpoint) {
			mp.SkipMountpointCreation = true
		}

		binds[mp.Destination] = true
		dereferenceIfExists(mp.Destination)
		mountPoints[mp.Destination] = mp
	}

	container.Lock()

	// 4. Cleanup old volumes that are about to be reassigned.
	for _, m := range mountPoints {
		if parser.IsBackwardCompatible(m) {
			if mp, exists := container.MountPoints[m.Destination]; exists && mp.Volume != nil {
				daemon.volumes.Release(ctx, mp.Volume.Name(), container.ID)
			}
		}
	}
	container.MountPoints = mountPoints

	container.Unlock()

	return nil
}

// lazyInitializeVolume initializes a mountpoint's volume if needed.
// This happens after a daemon restart.
func (daemon *Daemon) lazyInitializeVolume(containerID string, m *volumemounts.MountPoint) error {
	if len(m.Driver) > 0 && m.Volume == nil {
		v, err := daemon.volumes.Get(context.TODO(), m.Name, volumeopts.WithGetDriver(m.Driver), volumeopts.WithGetReference(containerID))
		if err != nil {
			return err
		}
		m.Volume = &volumeWrapper{v: v, s: daemon.volumes}
	}
	return nil
}

// VolumesService is used to perform volume operations
func (daemon *Daemon) VolumesService() *service.VolumesService {
	return daemon.volumes
}

type volumeMounter interface {
	Mount(ctx context.Context, v *volumetypes.Volume, ref string) (string, error)
	Unmount(ctx context.Context, v *volumetypes.Volume, ref string) error
}

type volumeWrapper struct {
	v *volumetypes.Volume
	s volumeMounter
}

func (v *volumeWrapper) Name() string {
	return v.v.Name
}

func (v *volumeWrapper) DriverName() string {
	return v.v.Driver
}

func (v *volumeWrapper) Path() string {
	return v.v.Mountpoint
}

func (v *volumeWrapper) Mount(ref string) (string, error) {
	return v.s.Mount(context.TODO(), v.v, ref)
}

func (v *volumeWrapper) Unmount(ref string) error {
	return v.s.Unmount(context.TODO(), v.v, ref)
}

func (v *volumeWrapper) CreatedAt() (time.Time, error) {
	return time.Time{}, errors.New("not implemented")
}

func (v *volumeWrapper) Status() map[string]interface{} {
	return v.v.Status
}
