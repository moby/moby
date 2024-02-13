//go:build !windows

package daemon // import "github.com/docker/docker/daemon"

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/containerd/log"
	containertypes "github.com/docker/docker/api/types/container"
	mounttypes "github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/container"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/internal/compatcontext"
	"github.com/docker/docker/oci"
	volumemounts "github.com/docker/docker/volume/mounts"
	volumeopts "github.com/docker/docker/volume/service/opts"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/pkg/errors"
)

// createContainerOSSpecificSettings performs host-OS specific container create functionality
func (daemon *Daemon) createContainerOSSpecificSettings(ctx context.Context, container *container.Container, config *containertypes.Config, hostConfig *containertypes.HostConfig) error {
	if err := daemon.Mount(container); err != nil {
		return err
	}
	defer daemon.Unmount(container)

	rootIDs := daemon.idMapping.RootPair()
	if err := container.SetupWorkingDirectory(rootIDs); err != nil {
		return err
	}

	// Set the default masked and readonly paths with regard to the host config options if they are not set.
	if hostConfig.MaskedPaths == nil && !hostConfig.Privileged {
		hostConfig.MaskedPaths = oci.DefaultSpec().Linux.MaskedPaths // Set it to the default if nil
		container.HostConfig.MaskedPaths = hostConfig.MaskedPaths
	}
	if hostConfig.ReadonlyPaths == nil && !hostConfig.Privileged {
		hostConfig.ReadonlyPaths = oci.DefaultSpec().Linux.ReadonlyPaths // Set it to the default if nil
		container.HostConfig.ReadonlyPaths = hostConfig.ReadonlyPaths
	}

	for spec := range config.Volumes {
		destination := filepath.Clean(spec)

		// Skip volumes for which we already have something mounted on that
		// destination because of a --volume-from.
		if container.HasMountFor(destination) {
			log.G(ctx).WithField("container", container.ID).WithField("destination", spec).Debug("mountpoint already exists, skipping anonymous volume")
			// Not an error, this could easily have come from the image config.
			continue
		}
		path, err := container.GetResourcePath(destination)
		if err != nil {
			return err
		}

		stat, err := os.Stat(path)
		if err == nil && !stat.IsDir() {
			return fmt.Errorf("cannot mount volume over existing file, file exists %s", path)
		}

		v, err := daemon.volumes.Create(context.TODO(), "", hostConfig.VolumeDriver, volumeopts.WithCreateReference(container.ID))
		if err != nil {
			return err
		}

		if err := label.Relabel(v.Mountpoint, container.MountLabel, true); err != nil {
			return err
		}

		container.AddMountPointWithVolume(destination, &volumeWrapper{v: v, s: daemon.volumes}, true)
	}
	return daemon.populateVolumes(ctx, container)
}

// populateVolumes copies data from the container's rootfs into the volume for non-binds.
// this is only called when the container is created.
func (daemon *Daemon) populateVolumes(ctx context.Context, c *container.Container) error {
	for _, mnt := range c.MountPoints {
		if mnt.Volume == nil {
			continue
		}

		if mnt.Type != mounttypes.TypeVolume || !mnt.CopyData {
			continue
		}

		if err := daemon.populateVolume(ctx, c, mnt); err != nil {
			return err
		}
	}
	return nil
}

func (daemon *Daemon) populateVolume(ctx context.Context, c *container.Container, mnt *volumemounts.MountPoint) error {
	ctrDestPath, err := c.GetResourcePath(mnt.Destination)
	if err != nil {
		return err
	}

	if _, err := os.Stat(ctrDestPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	volumePath, cleanup, err := mnt.Setup(ctx, c.MountLabel, daemon.idMapping.RootPair(), nil)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return nil
		}
		log.G(ctx).WithError(err).Debugf("can't copy data from %s:%s, to %s", c.ID, mnt.Destination, volumePath)
		return errors.Wrapf(err, "failed to populate volume")
	}
	defer mnt.Cleanup(compatcontext.WithoutCancel(ctx))
	defer cleanup(compatcontext.WithoutCancel(ctx))

	log.G(ctx).Debugf("copying image data from %s:%s, to %s", c.ID, mnt.Destination, volumePath)
	if err := c.CopyImagePathContent(volumePath, ctrDestPath); err != nil {
		return err
	}

	return nil
}
