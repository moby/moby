//go:build !windows
// +build !windows

package daemon // import "github.com/docker/docker/daemon"

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	containertypes "github.com/docker/docker/api/types/container"
	mounttypes "github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/container"
	"github.com/docker/docker/oci"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/pkg/stringid"
	volumeopts "github.com/docker/docker/volume/service/opts"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/sirupsen/logrus"
)

// createContainerOSSpecificSettings performs host-OS specific container create functionality
func (daemon *Daemon) createContainerOSSpecificSettings(container *container.Container, config *containertypes.Config, hostConfig *containertypes.HostConfig) error {
	if err := daemon.Mount(container); err != nil {
		return err
	}
	defer daemon.Unmount(container)

	user, err := getUser(container, config.User)
	if err != nil {
		return err
	}
	owner := idtools.Identity{UID: int(user.UID), GID: int(user.GID)}
	owner, err = daemon.idMapping.ToHost(owner)
	if err != nil {
		return err
	}
	if err := container.SetupWorkingDirectory(owner); err != nil {
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
		name := stringid.GenerateRandomID()
		destination := filepath.Clean(spec)

		// Skip volumes for which we already have something mounted on that
		// destination because of a --volume-from.
		if container.HasMountFor(destination) {
			logrus.WithField("container", container.ID).WithField("destination", spec).Debug("mountpoint already exists, skipping anonymous volume")
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

		v, err := daemon.volumes.Create(context.TODO(), name, hostConfig.VolumeDriver, volumeopts.WithCreateReference(container.ID))
		if err != nil {
			return err
		}

		if err := label.Relabel(v.Mountpoint, container.MountLabel, true); err != nil {
			return err
		}

		container.AddMountPointWithVolume(destination, &volumeWrapper{v: v, s: daemon.volumes}, true)
	}
	return daemon.populateVolumes(container)
}

// populateVolumes copies data from the container's rootfs into the volume for non-binds.
// this is only called when the container is created.
func (daemon *Daemon) populateVolumes(c *container.Container) error {
	for _, mnt := range c.MountPoints {
		if mnt.Volume == nil {
			continue
		}

		if mnt.Type != mounttypes.TypeVolume || !mnt.CopyData {
			continue
		}

		logrus.Debugf("copying image data from %s:%s, to %s", c.ID, mnt.Destination, mnt.Name)
		if err := c.CopyImagePathContent(mnt.Volume, mnt.Destination); err != nil {
			return err
		}
	}
	return nil
}
