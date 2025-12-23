package daemon

import (
	"context"
	"encoding/hex"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/containerd/log"
	mounttypes "github.com/moby/moby/api/types/mount"
	volumetypes "github.com/moby/moby/api/types/volume"
	"github.com/moby/moby/v2/daemon/container"
	"github.com/moby/moby/v2/daemon/internal/layer"
	"github.com/moby/moby/v2/daemon/server/imagebackend"
	"github.com/moby/moby/v2/daemon/volume"
	volumemounts "github.com/moby/moby/v2/daemon/volume/mounts"
	"github.com/moby/moby/v2/daemon/volume/service"
	volumeopts "github.com/moby/moby/v2/daemon/volume/service/opts"
	"github.com/moby/moby/v2/errdefs"
	"github.com/pkg/errors"
)

var _ volume.LiveRestorer = (*volumeWrapper)(nil)

// mountSort implements [sort.Interface] to sort an array of mounts in
// lexicographic order.
type mountSort []container.Mount

// Len returns the number of mounts. Used in sorting.
func (m mountSort) Len() int {
	return len(m)
}

// Less returns true if the number of parts (a/b/c would be 3 parts) in the
// mount indexed by parameter 1 is less than that of the mount indexed by
// parameter 2. Used in sorting.
func (m mountSort) Less(i, j int) bool {
	return m.parts(i) < m.parts(j)
}

// Swap swaps two items in an array of mounts. Used in sorting
func (m mountSort) Swap(i, j int) {
	m[i], m[j] = m[j], m[i]
}

// parts returns the number of parts in the destination of a mount. Used in sorting.
func (m mountSort) parts(i int) int {
	return strings.Count(filepath.Clean(m[i].Destination), string(os.PathSeparator))
}

// sortMounts sorts an array of mounts in lexicographic order. This ensure that
// when mounting, the mounts don't shadow other mounts. For example, if mounting
// /etc and /etc/resolv.conf, /etc/resolv.conf must not be mounted first.
func sortMounts(m []container.Mount) []container.Mount {
	sort.Sort(mountSort(m))
	return m
}

// registerMountPoints initializes the container mount points with the configured volumes and bind mounts.
// It follows the next sequence to decide what to mount in each final destination:
//
// 1. Select the previously configured mount points for the containers, if any.
// 2. Select the volumes mounted from another containers. Overrides previously configured mount point destination.
// 3. Select the bind mounts set by the client. Overrides previously configured mount point destinations.
// 4. Cleanup old volumes that are about to be reassigned.
//
// Do not lock while creating volumes since this could be calling out to external plugins
// Don't want to block other actions, like `docker ps` because we're waiting on an external plugin
func (daemon *Daemon) registerMountPoints(ctr *container.Container, defaultReadOnlyNonRecursive bool) (retErr error) {
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
				daemon.volumes.Release(ctx, m.Volume.Name(), ctr.ID)
			}
		}
	}()

	dereferenceIfExists := func(destination string) {
		if v, ok := mountPoints[destination]; ok {
			log.G(ctx).Debugf("Duplicate mount point '%s'", destination)
			if v.Volume != nil {
				daemon.volumes.Release(ctx, v.Volume.Name(), ctr.ID)
			}
		}
	}

	// 1. Read already configured mount points.
	maps.Copy(mountPoints, ctr.MountPoints)

	// 2. Read volumes from other containers.
	for _, v := range ctr.HostConfig.VolumesFrom {
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

			if cp.Source == "" {
				v, err := daemon.volumes.Get(ctx, cp.Name, volumeopts.WithGetDriver(cp.Driver), volumeopts.WithGetReference(ctr.ID))
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
	for _, b := range ctr.HostConfig.Binds {
		bind, err := parser.ParseMountRaw(b, ctr.HostConfig.VolumeDriver)
		if err != nil {
			return err
		}
		needsSlavePropagation, err := daemon.validateBindDaemonRoot(bind.Spec)
		if err != nil {
			return err
		}
		if needsSlavePropagation {
			bind.Propagation = mounttypes.PropagationRSlave
		}

		// #10618
		_, tmpfsExists := ctr.HostConfig.Tmpfs[bind.Destination]
		if binds[bind.Destination] || tmpfsExists {
			return duplicateMountPointError(bind.Destination)
		}

		if bind.Type == mounttypes.TypeVolume {
			// create the volume
			v, err := daemon.volumes.Create(ctx, bind.Name, bind.Driver, volumeopts.WithCreateReference(ctr.ID))
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

		if bind.Type == mounttypes.TypeBind && !bind.RW {
			if defaultReadOnlyNonRecursive {
				if bind.Spec.BindOptions == nil {
					bind.Spec.BindOptions = &mounttypes.BindOptions{}
				}
				bind.Spec.BindOptions.ReadOnlyNonRecursive = true
			}
		}

		binds[bind.Destination] = true
		dereferenceIfExists(bind.Destination)
		mountPoints[bind.Destination] = bind
	}

	for _, cfg := range ctr.HostConfig.Mounts {
		mp, err := parser.ParseMountSpec(cfg)
		if err != nil {
			return errdefs.InvalidParameter(err)
		}
		needsSlavePropagation, err := daemon.validateBindDaemonRoot(mp.Spec)
		if err != nil {
			return err
		}
		if needsSlavePropagation {
			mp.Propagation = mounttypes.PropagationRSlave
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
					volumeopts.WithCreateReference(ctr.ID),
					volumeopts.WithCreateOptions(driverOpts),
					volumeopts.WithCreateLabels(cfg.VolumeOptions.Labels),
				)
			} else {
				v, err = daemon.volumes.Create(ctx, mp.Name, mp.Driver, volumeopts.WithCreateReference(ctr.ID))
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

		if mp.Type == mounttypes.TypeBind {
			if cfg.BindOptions == nil || !cfg.BindOptions.CreateMountpoint {
				mp.SkipMountpointCreation = true
			}

			if !mp.RW && defaultReadOnlyNonRecursive {
				if mp.Spec.BindOptions == nil {
					mp.Spec.BindOptions = &mounttypes.BindOptions{}
				}
				mp.Spec.BindOptions.ReadOnlyNonRecursive = true
			}
		}

		if mp.Type == mounttypes.TypeImage {
			img, err := daemon.imageService.GetImage(ctx, mp.Source, imagebackend.GetImageOpts{})
			if err != nil {
				return err
			}

			rwLayerOpts := &layer.CreateRWLayerOpts{
				StorageOpt: ctr.HostConfig.StorageOpt,
			}

			// Include the destination in the layer name to make it unique for each mount point and container.
			// This makes sure that the same image can be mounted multiple times with different destinations.
			// Hex encode the destination to create a safe, unique identifier
			layerName := hex.EncodeToString([]byte(ctr.ID + ",src=" + mp.Source + ",dst=" + mp.Destination))
			layer, err := daemon.imageService.CreateLayerFromImage(img, layerName, rwLayerOpts)
			if err != nil {
				return err
			}
			metadata, err := layer.Metadata()
			if err != nil {
				return err
			}

			path, err := layer.Mount("")
			if err != nil {
				return err
			}

			if metadata["ID"] != "" {
				mp.ID = metadata["ID"]
			}

			mp.Name = mp.Spec.Source
			mp.Spec.Source = img.ID().String()
			mp.Source = path
			mp.Layer = layer
			mp.RW = false
		}

		if mp.Type == mounttypes.TypeAPISocket {
			socket, err := daemon.apiSocket(ctr.ID, cfg.APISocketOptions)
			if err != nil {
				return err
			}
			mp.Source = socket
			log.L.Errorf("daemon.registerMountPoints: mount api socket: %+v", mp)
		}
		binds[mp.Destination] = true
		dereferenceIfExists(mp.Destination)
		mountPoints[mp.Destination] = mp
	}

	ctr.Lock()

	// 4. Cleanup old volumes that are about to be reassigned.
	for _, m := range mountPoints {
		if parser.IsBackwardCompatible(m) {
			if mp, exists := ctr.MountPoints[m.Destination]; exists && mp.Volume != nil {
				daemon.volumes.Release(ctx, mp.Volume.Name(), ctr.ID)
			}
		}
	}
	ctr.MountPoints = mountPoints

	ctr.Unlock()

	return nil
}

// lazyInitializeVolume initializes a mountpoint's volume if needed.
// This happens after a daemon restart.
func (daemon *Daemon) lazyInitializeVolume(containerID string, m *volumemounts.MountPoint) error {
	if m.Driver != "" && m.Volume == nil {
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
	LiveRestoreVolume(ctx context.Context, v *volumetypes.Volume, ref string) error
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

func (v *volumeWrapper) Status() map[string]any {
	return v.v.Status
}

func (v *volumeWrapper) LiveRestoreVolume(ctx context.Context, ref string) error {
	return v.s.LiveRestoreVolume(ctx, v.v, ref)
}
