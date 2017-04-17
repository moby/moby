// +build !windows

package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Sirupsen/logrus"
	containertypes "github.com/docker/docker/api/types/container"
	mounttypes "github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/container"
	"github.com/docker/docker/pkg/stringid"
	"github.com/opencontainers/runc/libcontainer/label"
)

// createContainerPlatformSpecificSettings performs platform specific container create functionality
func (daemon *Daemon) createContainerPlatformSpecificSettings(container *container.Container, config *containertypes.Config, hostConfig *containertypes.HostConfig) error {
	if err := daemon.Mount(container); err != nil {
		return err
	}
	defer daemon.Unmount(container)

	rootUID, rootGID := daemon.GetRemappedUIDGID()
	if err := container.SetupWorkingDirectory(rootUID, rootGID); err != nil {
		return err
	}

	var defaultDriver = hostConfig.VolumeDriver
	if driver, ok := config.Labels["com.docker.swarm.volume.default.driver"]; ok {
		defaultDriver = driver
	}
	var defaultOpts map[string]string
	if opts, ok := config.Labels["com.docker.swarm.volume.default.opts"]; ok {
		defaultOpts = make(map[string]string)
		rawOpts := strings.Split(opts, ",")
		for id := range rawOpts {
			pair := strings.SplitN(rawOpts[id], "=", 2)
			if len(pair) != 2 {
				return fmt.Errorf("Unrecognised default opts: %s", rawOpts[id])
			}
			defaultOpts[pair[0]] = pair[1]
		}
	}

	for spec := range config.Volumes {
		name := stringid.GenerateNonCryptoID()

		if defaultName, ok := config.Labels["com.docker.swarm.volume.default.name"]; ok {
			name = defaultName
		} else if serviceName, ok := config.Labels["com.docker.swarm.task.name"]; ok {
			name = fmt.Sprintf("%s.%x", serviceName, spec)
		}
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
			return fmt.Errorf("cannot mount volume over existing file, file exists %s", path)
		}

		v, err := daemon.volumes.CreateWithRef(name, defaultDriver, container.ID, defaultOpts, nil)
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
