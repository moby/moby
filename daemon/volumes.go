package daemon

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/chrootarchive"
	"github.com/docker/docker/runconfig"
	"github.com/docker/docker/volume"
	"github.com/docker/docker/volume/local"
)

func parseBindMount(spec, containerVolumeDriver string) (*mountPoint, error) {
	bind := &mountPoint{
		RW: true,
	}
	arr := strings.Split(spec, ":")

	switch len(arr) {
	case 2:
		bind.Destination = arr[1]
	case 3:
		bind.Destination = arr[1]
		mode := arr[2]
		if !validMountMode(mode) {
			return nil, fmt.Errorf("invalid mode for volumes-from: %s", mode)
		}
		bind.RW = rwModes[mode]
		// Relabel will apply a SELinux label, if necessary
		bind.Relabel = mode
	default:
		return nil, fmt.Errorf("Invalid volume specification: %s", spec)
	}

	name, source, err := parseVolumeSource(arr[0])
	if err != nil {
		return nil, err
	}

	if len(source) == 0 {
		bind.Driver = containerVolumeDriver
		if len(bind.Driver) == 0 {
			bind.Driver = volume.DefaultDriverName
		}
	} else {
		bind.Source = filepath.Clean(source)
	}

	bind.Name = name
	bind.Destination = filepath.Clean(bind.Destination)
	return bind, nil
}

func parseVolumesFromContainer(spec string) (string, string, error) {
	if len(spec) == 0 {
		return "", "", fmt.Errorf("malformed volumes-from specification: %s", spec)
	}

	specParts := strings.SplitN(spec, ":", 2)
	id := specParts[0]
	mode := "rw"

	if len(specParts) == 2 {
		mode = specParts[1]
		if !validMountMode(mode) {
			return "", "", fmt.Errorf("invalid mode for volumes-from: %s", mode)
		}
	}
	return id, mode, nil
}

// read-write modes
var rwModes = map[string]bool{
	"rw":   true,
	"rw,Z": true,
	"rw,z": true,
	"z,rw": true,
	"Z,rw": true,
	"Z":    true,
	"z":    true,
}

// read-only modes
var roModes = map[string]bool{
	"ro":   true,
	"ro,Z": true,
	"ro,z": true,
	"z,ro": true,
	"Z,ro": true,
}

func validMountMode(mode string) bool {
	return roModes[mode] || rwModes[mode]
}

func copyExistingContents(source, destination string) error {
	volList, err := ioutil.ReadDir(source)
	if err != nil {
		return err
	}
	if len(volList) > 0 {
		srcList, err := ioutil.ReadDir(destination)
		if err != nil {
			return err
		}
		if len(srcList) == 0 {
			// If the source volume is empty copy files from the root into the volume
			if err := chrootarchive.CopyWithTar(source, destination); err != nil {
				return err
			}
		}
	}
	return copyOwnership(source, destination)
}

// registerMountPoints initializes the container mount points with the configured volumes and bind mounts.
// It follows the next sequence to decide what to mount in each final destination:
//
// 1. Select the previously configured mount points for the containers, if any.
// 2. Select the volumes mounted from another containers. Overrides previously configured mount point destinations.
// 3. Select the volumes configured in a json file. Overrides previously configured mount point destinations.
// 4. Select the bind mounts set by the client. Overrides previously configured mount point destinations.
func (daemon *Daemon) registerMountPoints(container *Container, hostConfig *runconfig.HostConfig) error {
	binds := map[string]bool{}
	mountPoints := map[string]*mountPoint{}

	// 1. Read already configured mount points.
	for name, point := range container.MountPoints {
		mountPoints[name] = point
	}

	// 2. Read volumes from other containers.
	//    Safe volumes from files for later to not change the configuration order.
	var volumeFiles []string
	for _, v := range hostConfig.VolumesFrom {
		if strings.HasPrefix(v, "@") {
			volumeFiles = append(volumeFiles, v)
		} else {
			containerID, mode, err := parseVolumesFromContainer(v)
			if err != nil {
				return err
			}

			c, err := daemon.Get(containerID)
			if err != nil {
				return err
			}

			for _, m := range c.MountPoints {
				cp := m
				cp.RW = m.RW && mode != "ro"

				if !cp.isBindMount() {
					v, err := createVolume(cp.Name, cp.Driver)
					if err != nil {
						return err
					}
					cp.Volume = v
				}

				mountPoints[cp.Destination] = cp
			}
		}
	}

	// 3. Read volumes from json configuration
	for _, f := range volumeFiles {
		mounts, err := parseVolumesFromFile(f, container.MountLabel, container.Config.VolumeDriver)
		if err != nil {
			return err
		}

		for _, bind := range mounts {
			if binds[bind.Destination] {
				return fmt.Errorf("Duplicate bind mount %s", bind.Destination)
			}

			if !bind.isBindMount() {
				if err := bind.createVolume(); err != nil {
					return err
				}
			}

			if err := bind.applyLabel(container.MountLabel); err != nil {
				return err
			}

			binds[bind.Destination] = true
			mountPoints[bind.Destination] = bind
		}
	}

	// 4. Read bind mounts
	for _, b := range hostConfig.Binds {
		// #10618
		bind, err := parseBindMount(b, container.Config.VolumeDriver)
		if err != nil {
			return err
		}

		if binds[bind.Destination] {
			return fmt.Errorf("Duplicate bind mount %s", bind.Destination)
		}

		if !bind.isBindMount() {
			if err := bind.createVolume(); err != nil {
				return err
			}
		}

		if err := bind.applyLabel(container.MountLabel); err != nil {
			return err
		}

		binds[bind.Destination] = true
		mountPoints[bind.Destination] = bind
	}

	// Keep backwards compatible structures
	bcVolumes := map[string]string{}
	bcVolumesRW := map[string]bool{}
	for _, m := range mountPoints {
		if m.backwardsCompatible() {
			bcVolumes[m.Destination] = m.Path()
			bcVolumesRW[m.Destination] = m.RW
		}
	}

	container.Lock()
	container.MountPoints = mountPoints
	container.Volumes = bcVolumes
	container.VolumesRW = bcVolumesRW
	container.Unlock()

	return nil
}

// verifyVolumesInfo ports volumes configured for the containers pre docker 1.7.
// It reads the container configuration and creates valid mount points for the old volumes.
func (daemon *Daemon) verifyVolumesInfo(container *Container) error {
	// Inspect old structures only when we're upgrading from old versions
	// to versions >= 1.7 and the MountPoints has not been populated with volumes data.
	if len(container.MountPoints) == 0 && len(container.Volumes) > 0 {
		for destination, hostPath := range container.Volumes {
			vfsPath := filepath.Join(daemon.root, "vfs", "dir")
			rw := container.VolumesRW != nil && container.VolumesRW[destination]

			if strings.HasPrefix(hostPath, vfsPath) {
				id := filepath.Base(hostPath)
				if err := migrateVolume(id, hostPath); err != nil {
					return err
				}
				container.addLocalMountPoint(id, destination, rw)
			} else { // Bind mount
				id, source, err := parseVolumeSource(hostPath)
				// We should not find an error here coming
				// from the old configuration, but who knows.
				if err != nil {
					return err
				}
				container.addBindMountPoint(id, source, destination, rw)
			}
		}
	} else if len(container.MountPoints) > 0 {
		// Volumes created with a Docker version >= 1.7. We verify integrity in case of data created
		// with Docker 1.7 RC versions that put the information in
		// DOCKER_ROOT/volumes/VOLUME_ID rather than DOCKER_ROOT/volumes/VOLUME_ID/_container_data.
		l, err := getVolumeDriver(volume.DefaultDriverName)
		if err != nil {
			return err
		}

		for _, m := range container.MountPoints {
			if m.Driver != volume.DefaultDriverName {
				continue
			}
			dataPath := l.(*local.Root).DataPath(m.Name)
			volumePath := filepath.Dir(dataPath)

			d, err := ioutil.ReadDir(volumePath)
			if err != nil {
				// If the volume directory doesn't exist yet it will be recreated,
				// so we only return the error when there is a different issue.
				if !os.IsNotExist(err) {
					return err
				}
				// Do not check when the volume directory does not exist.
				continue
			}
			if validVolumeLayout(d) {
				continue
			}

			if err := os.Mkdir(dataPath, 0755); err != nil {
				return err
			}

			// Move data inside the data directory
			for _, f := range d {
				oldp := filepath.Join(volumePath, f.Name())
				newp := filepath.Join(dataPath, f.Name())
				if err := os.Rename(oldp, newp); err != nil {
					logrus.Errorf("Unable to move %s to %s\n", oldp, newp)
				}
			}
		}

		return container.ToDisk()
	}

	return nil
}

func createVolume(name, driverName string) (volume.Volume, error) {
	vd, err := getVolumeDriver(driverName)
	if err != nil {
		return nil, err
	}
	return vd.Create(name)
}

func removeVolume(v volume.Volume) error {
	vd, err := getVolumeDriver(v.DriverName())
	if err != nil {
		return nil
	}
	return vd.Remove(v)
}

func parseVolumesFromFile(fileID, containerMountLabel, containerVolumeDriver string) ([]*mountPoint, error) {
	filePath := fileID[1:]
	var mounts []mountPointExported

	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	if err := json.NewDecoder(f).Decode(&mounts); err != nil {
		return nil, err
	}

	mountPoints := make([]*mountPoint, len(mounts))

	for i, m := range mounts {
		if len(m.Destination) == 0 {
			return nil, fmt.Errorf("invalid empty mount point destination")
		}
		if len(m.Source) == 0 && len(m.Name) == 0 {
			return nil, fmt.Errorf("invalid empty mount point source and name. Either Name or Source must be provided")
		}

		bind := &mountPoint{}

		if len(m.Source) == 0 {
			bind.Driver = containerVolumeDriver
			if len(bind.Driver) == 0 {
				bind.Driver = volume.DefaultDriverName
			}
		} else {
			bind.Source = filepath.Clean(m.Source)
		}

		if len(m.Mode) == 0 {
			m.Mode = "rw"
		}

		if !validMountMode(m.Mode) {
			return nil, fmt.Errorf("invalid mode for mount point: %s", m.Mode)
		}

		bind.RW = rwModes[m.Mode]
		bind.Relabel = m.Mode
		bind.Name = m.Name
		bind.Destination = filepath.Clean(m.Destination)

		mountPoints[i] = bind
	}

	return mountPoints, nil
}
