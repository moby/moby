// +build !windows

package daemon

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/execdriver"
	"github.com/docker/docker/pkg/chrootarchive"
	"github.com/docker/docker/pkg/system"
	"github.com/docker/docker/volume"
	volumedrivers "github.com/docker/docker/volume/drivers"
	"github.com/docker/docker/volume/local"
)

// copyExistingContents copies from the source to the destination and
// ensures the ownership is appropriately set.
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

// copyOwnership copies the permissions and uid:gid of the source file
// to the destination file
func copyOwnership(source, destination string) error {
	stat, err := system.Stat(source)
	if err != nil {
		return err
	}

	if err := os.Chown(destination, int(stat.UID()), int(stat.GID())); err != nil {
		return err
	}

	return os.Chmod(destination, os.FileMode(stat.Mode()))
}

// setupMounts iterates through each of the mount points for a container and
// calls Setup() on each. It also looks to see if is a network mount such as
// /etc/resolv.conf, and if it is not, appends it to the array of mounts.
func (container *Container) setupMounts() ([]execdriver.Mount, error) {
	var mounts []execdriver.Mount
	for _, m := range container.MountPoints {
		path, err := m.Setup()
		if err != nil {
			return nil, err
		}
		if !container.trySetNetworkMount(m.Destination, path) {
			mounts = append(mounts, execdriver.Mount{
				Source:      path,
				Destination: m.Destination,
				Writable:    m.RW,
			})
		}
	}

	mounts = sortMounts(mounts)
	netMounts := container.networkMounts()
	// if we are going to mount any of the network files from container
	// metadata, the ownership must be set properly for potential container
	// remapped root (user namespaces)
	rootUID, rootGID := container.daemon.GetRemappedUIDGID()
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
func sortMounts(m []execdriver.Mount) []execdriver.Mount {
	sort.Sort(mounts(m))
	return m
}

// migrateVolume links the contents of a volume created pre Docker 1.7
// into the location expected by the local driver.
// It creates a symlink from DOCKER_ROOT/vfs/dir/VOLUME_ID to DOCKER_ROOT/volumes/VOLUME_ID/_container_data.
// It preserves the volume json configuration generated pre Docker 1.7 to be able to
// downgrade from Docker 1.7 to Docker 1.6 without losing volume compatibility.
func migrateVolume(id, vfs string) error {
	l, err := volumedrivers.Lookup(volume.DefaultDriverName)
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

// validVolumeLayout checks whether the volume directory layout
// is valid to work with Docker post 1.7 or not.
func validVolumeLayout(files []os.FileInfo) bool {
	if len(files) == 1 && files[0].Name() == local.VolumeDataPathName && files[0].IsDir() {
		return true
	}

	if len(files) != 2 {
		return false
	}

	for _, f := range files {
		if f.Name() == "config.json" ||
			(f.Name() == local.VolumeDataPathName && f.Mode()&os.ModeSymlink == os.ModeSymlink) {
			// Old volume configuration, we ignore it
			continue
		}
		return false
	}

	return true
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
				id, source := volume.ParseVolumeSource(hostPath)
				container.addBindMountPoint(id, source, destination, rw)
			}
		}
	} else if len(container.MountPoints) > 0 {
		// Volumes created with a Docker version >= 1.7. We verify integrity in case of data created
		// with Docker 1.7 RC versions that put the information in
		// DOCKER_ROOT/volumes/VOLUME_ID rather than DOCKER_ROOT/volumes/VOLUME_ID/_container_data.
		l, err := volumedrivers.Lookup(volume.DefaultDriverName)
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

		return container.toDiskLocking()
	}

	return nil
}

// setBindModeIfNull is platform specific processing to ensure the
// shared mode is set to 'z' if it is null. This is called in the case
// of processing a named volume and not a typical bind.
func setBindModeIfNull(bind *volume.MountPoint) *volume.MountPoint {
	if bind.Mode == "" {
		bind.Mode = "z"
	}
	return bind
}

// configureBackCompatStructures is platform specific processing for
// registering mount points to populate old structures.
func configureBackCompatStructures(daemon *Daemon, container *Container, mountPoints map[string]*volume.MountPoint) (map[string]string, map[string]bool) {
	// Keep backwards compatible structures
	bcVolumes := map[string]string{}
	bcVolumesRW := map[string]bool{}
	for _, m := range mountPoints {
		if m.BackwardsCompatible() {
			bcVolumes[m.Destination] = m.Path()
			bcVolumesRW[m.Destination] = m.RW

			// This mountpoint is replacing an existing one, so the count needs to be decremented
			if mp, exists := container.MountPoints[m.Destination]; exists && mp.Volume != nil {
				daemon.volumes.Decrement(mp.Volume)
			}
		}
	}
	return bcVolumes, bcVolumesRW
}

// setBackCompatStructures is a platform specific helper function to set
// backwards compatible structures in the container when registering volumes.
func setBackCompatStructures(container *Container, bcVolumes map[string]string, bcVolumesRW map[string]bool) {
	container.Volumes = bcVolumes
	container.VolumesRW = bcVolumesRW
}
