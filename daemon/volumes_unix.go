// +build !windows

package daemon

import (
	"io/ioutil"
	"os"
	"sort"

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
func (daemon *Daemon) setupMounts(container *Container) ([]execdriver.Mount, error) {
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

// setBindModeIfNull is platform specific processing to ensure the
// shared mode is set to 'z' if it is null. This is called in the case
// of processing a named volume and not a typical bind.
func setBindModeIfNull(bind *volume.MountPoint) *volume.MountPoint {
	if bind.Mode == "" {
		bind.Mode = "z"
	}
	return bind
}
