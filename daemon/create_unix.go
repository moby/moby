// +build !windows

package daemon

import (
	"os"
	"path/filepath"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/container"
	derr "github.com/docker/docker/errors"
	"github.com/docker/docker/image"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/volume"
	containertypes "github.com/docker/engine-api/types/container"
	"github.com/opencontainers/runc/libcontainer/label"
)

// createContainerPlatformSpecificSettings performs platform specific container create functionality
func (daemon *Daemon) createContainerPlatformSpecificSettings(container *container.Container, config *containertypes.Config, hostConfig *containertypes.HostConfig, img *image.Image) error {
	if err := daemon.Mount(container); err != nil {
		return err
	}
	defer daemon.Unmount(container)

	if err := container.SetupWorkingDirectory(); err != nil {
		return err
	}

	for spec := range config.Volumes {
		name := stringid.GenerateNonCryptoID()
		destination := filepath.Clean(spec)

		// Skip volumes for which we already have something mounted on that
		// destination because of a --volume-from.
		if container.IsDestinationMounted(destination) {
			continue
		}
		path, err := container.GetResourcePath(destination)
		if err != nil {
			return err
		}

		stat, err := os.Stat(path)
		if err == nil && !stat.IsDir() {
			return derr.ErrorCodeMountOverFile.WithArgs(path)
		}

		volumeDriver := hostConfig.VolumeDriver
		if destination != "" && img != nil {
			if _, ok := img.ContainerConfig.Volumes[destination]; ok {
				// check for whether bind is not specified and then set to local
				if _, ok := container.MountPoints[destination]; !ok {
					volumeDriver = volume.DefaultDriverName
				}
			}
		}

		v, err := daemon.volumes.CreateWithRef(name, volumeDriver, container.ID, nil)
		if err != nil {
			return err
		}

		if err := label.Relabel(v.Path(), container.MountLabel, true); err != nil {
			return err
		}

		container.AddMountPointWithVolume(destination, v, true)
	}
	return daemon.populateVolumes(container)
}

// populateVolumes copies data from the container's rootfs into the volume for non-binds.
// this is only called when the container is created.
func (daemon *Daemon) populateVolumes(c *container.Container) error {
	for _, mnt := range c.MountPoints {
		// skip binds and volumes referenced by other containers (ie, volumes-from)
		if mnt.Driver == "" || mnt.Volume == nil || len(daemon.volumes.Refs(mnt.Volume)) > 1 {
			continue
		}

		logrus.Debugf("copying image data from %s:%s, to %s", c.ID, mnt.Destination, mnt.Name)
		if err := c.CopyImagePathContent(mnt.Volume, mnt.Destination); err != nil {
			return err
		}
	}
	return nil
}
