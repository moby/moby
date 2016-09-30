// +build !windows

package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/docker/docker/container"
	"github.com/docker/docker/volume"
	"github.com/docker/docker/volume/drivers"
	"github.com/docker/docker/volume/local"
	"github.com/pkg/errors"
)

// setupMounts iterates through each of the mount points for a container and
// calls Setup() on each. It also looks to see if is a network mount such as
// /etc/resolv.conf, and if it is not, appends it to the array of mounts.
func (daemon *Daemon) setupMounts(c *container.Container) ([]container.Mount, error) {
	var mounts []container.Mount
	// TODO: tmpfs mounts should be part of Mountpoints
	tmpfsMounts := make(map[string]bool)
	for _, m := range c.TmpfsMounts() {
		tmpfsMounts[m.Destination] = true
	}
	for _, m := range c.MountPoints {
		if tmpfsMounts[m.Destination] {
			continue
		}
		if err := daemon.lazyInitializeVolume(c.ID, m); err != nil {
			return nil, err
		}
		rootUID, rootGID := daemon.GetRemappedUIDGID()
		path, err := m.Setup(c.MountLabel, rootUID, rootGID)
		if err != nil {
			return nil, err
		}
		if !c.TrySetNetworkMount(m.Destination, path) {
			mnt := container.Mount{
				Source:      path,
				Destination: m.Destination,
				Writable:    m.RW,
				Propagation: string(m.Propagation),
			}
			if m.Volume != nil {
				attributes := map[string]string{
					"driver":      m.Volume.DriverName(),
					"container":   c.ID,
					"destination": m.Destination,
					"read/write":  strconv.FormatBool(m.RW),
					"propagation": string(m.Propagation),
				}
				daemon.LogVolumeEvent(m.Volume.Name(), "mount", attributes)
			}
			mounts = append(mounts, mnt)
		}
	}

	mounts = sortMounts(mounts)
	netMounts := c.NetworkMounts()
	// if we are going to mount any of the network files from container
	// metadata, the ownership must be set properly for potential container
	// remapped root (user namespaces)
	rootUID, rootGID := daemon.GetRemappedUIDGID()
	for _, mount := range netMounts {
		if err := os.Chown(mount.Source, rootUID, rootGID); err != nil {
			return nil, err
		}
	}
	return append(mounts, netMounts...), nil
}

// sortMounts sorts an array of mounts in lexicographic order. This ensure that
// when mounting, the mounts don't shadow other mounts. For example, if mounting
// /etc and /etc/resolv.conf, /etc/resolv.conf must not be mounted first.
func sortMounts(m []container.Mount) []container.Mount {
	sort.Sort(mounts(m))
	return m
}

// setBindModeIfNull is platform specific processing to ensure the
// shared mode is set to 'z' if it is null. This is called in the case
// of processing a named volume and not a typical bind.
func setBindModeIfNull(bind *volume.MountPoint) {
	if bind.Mode == "" {
		bind.Mode = "z"
	}
}

// migrateVolume links the contents of a volume created pre Docker 1.7
// into the location expected by the local driver.
// It creates a symlink from DOCKER_ROOT/vfs/dir/VOLUME_ID to DOCKER_ROOT/volumes/VOLUME_ID/_container_data.
// It preserves the volume json configuration generated pre Docker 1.7 to be able to
// downgrade from Docker 1.7 to Docker 1.6 without losing volume compatibility.
func migrateVolume(id, vfs string) error {
	l, err := volumedrivers.GetDriver(volume.DefaultDriverName)
	if err != nil {
		return err
	}

	newDataPath := l.(*local.Root).DataPath(id)
	fi, err := os.Stat(newDataPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	if fi != nil && fi.IsDir() {
		return nil
	}

	return os.Symlink(vfs, newDataPath)
}

// verifyVolumesInfo ports volumes configured for the containers pre docker 1.7.
// It reads the container configuration and creates valid mount points for the old volumes.
func (daemon *Daemon) verifyVolumesInfo(container *container.Container) error {
	// Inspect old structures only when we're upgrading from old versions
	// to versions >= 1.7 and the MountPoints has not been populated with volumes data.
	type volumes struct {
		Volumes   map[string]string
		VolumesRW map[string]bool
	}
	cfgPath, err := container.ConfigPath()
	if err != nil {
		return err
	}
	f, err := os.Open(cfgPath)
	if err != nil {
		return errors.Wrap(err, "could not open container config")
	}
	var cv volumes
	if err := json.NewDecoder(f).Decode(&cv); err != nil {
		return errors.Wrap(err, "could not decode container config")
	}

	if len(container.MountPoints) == 0 && len(cv.Volumes) > 0 {
		for destination, hostPath := range cv.Volumes {
			vfsPath := filepath.Join(daemon.root, "vfs", "dir")
			rw := cv.VolumesRW != nil && cv.VolumesRW[destination]

			if strings.HasPrefix(hostPath, vfsPath) {
				id := filepath.Base(hostPath)
				v, err := daemon.volumes.CreateWithRef(id, volume.DefaultDriverName, container.ID, nil, nil)
				if err != nil {
					return err
				}
				if err := migrateVolume(id, hostPath); err != nil {
					return err
				}
				container.AddMountPointWithVolume(destination, v, true)
			} else { // Bind mount
				m := volume.MountPoint{Source: hostPath, Destination: destination, RW: rw}
				container.MountPoints[destination] = &m
			}
		}
		return container.ToDisk()
	}
	return nil
}
