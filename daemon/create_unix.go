// +build !windows

package daemon

import (
	"os"
	"path/filepath"

	derr "github.com/docker/docker/errors"
	"github.com/docker/docker/image"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/runconfig"
	"github.com/docker/docker/volume"
	"github.com/opencontainers/runc/libcontainer/label"
)

// createContainerPlatformSpecificSettings performs platform specific container create functionality
func (daemon *Daemon) createContainerPlatformSpecificSettings(container *Container, config *runconfig.Config, hostConfig *runconfig.HostConfig, img *image.Image) error {
	if err := daemon.Mount(container); err != nil {
		return err
	}
	defer daemon.Unmount(container)

	for spec := range config.Volumes {
		name := stringid.GenerateNonCryptoID()
		destination := filepath.Clean(spec)

		// Skip volumes for which we already have something mounted on that
		// destination because of a --volume-from.
		if container.isDestinationMounted(destination) {
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

		v, err := daemon.createVolume(name, volumeDriver, nil)
		if err != nil {
			return err
		}

		if err := label.Relabel(v.Path(), container.MountLabel, true); err != nil {
			return err
		}

		// never attempt to copy existing content in a container FS to a shared volume
		if v.DriverName() == volume.DefaultDriverName {
			if err := container.copyImagePathContent(v, destination); err != nil {
				return err
			}
		}

		container.addMountPointWithVolume(destination, v, true)
	}
	return nil
}
