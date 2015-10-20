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
	derr "github.com/docker/docker/errors"
	"github.com/docker/docker/pkg/system"
	"github.com/docker/docker/runconfig"
	"github.com/docker/docker/volume"
	volumedrivers "github.com/docker/docker/volume/drivers"
	"github.com/docker/docker/volume/local"
	"github.com/opencontainers/runc/libcontainer/label"
)

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

// parseBindMount validates the configuration of mount information in runconfig is valid.
func parseBindMount(spec, volumeDriver string) (*mountPoint, error) {
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
		if !volume.ValidMountMode(mode) {
			return nil, derr.ErrorCodeVolumeInvalidMode.WithArgs(mode)
		}
		bind.RW = volume.ReadWrite(mode)
		// Mode field is used by SELinux to decide whether to apply label
		bind.Mode = mode
	default:
		return nil, derr.ErrorCodeVolumeInvalid.WithArgs(spec)
	}

	//validate the volumes destination path
	if !filepath.IsAbs(bind.Destination) {
		return nil, derr.ErrorCodeVolumeAbs.WithArgs(bind.Destination)
	}

	name, source, err := parseVolumeSource(arr[0])
	if err != nil {
		return nil, err
	}

	if len(source) == 0 {
		bind.Driver = volumeDriver
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

// sortMounts sorts an array of mounts in lexicographic order. This ensure that
// when mounting, the mounts don't shadow other mounts. For example, if mounting
// /etc and /etc/resolv.conf, /etc/resolv.conf must not be mounted first.
func sortMounts(m []execdriver.Mount) []execdriver.Mount {
	sort.Sort(mounts(m))
	return m
}

type mounts []execdriver.Mount

// Len returns the number of mounts
func (m mounts) Len() int {
	return len(m)
}

// Less returns true if the number of parts (a/b/c would be 3 parts) in the
// mount indexed by parameter 1 is less than that of the mount indexed by
// parameter 2.
func (m mounts) Less(i, j int) bool {
	return m.parts(i) < m.parts(j)
}

// Swap swaps two items in an array of mounts.
func (m mounts) Swap(i, j int) {
	m[i], m[j] = m[j], m[i]
}

// parts returns the number of parts in the destination of a mount.
func (m mounts) parts(i int) int {
	return len(strings.Split(filepath.Clean(m[i].Destination), string(os.PathSeparator)))
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

// parseVolumesFrom ensure that the supplied volumes-from is valid.
func parseVolumesFrom(spec string) (string, string, error) {
	if len(spec) == 0 {
		return "", "", derr.ErrorCodeVolumeFromBlank.WithArgs(spec)
	}

	specParts := strings.SplitN(spec, ":", 2)
	id := specParts[0]
	mode := "rw"

	if len(specParts) == 2 {
		mode = specParts[1]
		if !volume.ValidMountMode(mode) {
			return "", "", derr.ErrorCodeVolumeMode.WithArgs(mode)
		}
	}
	return id, mode, nil
}

// registerMountPoints initializes the container mount points with the configured volumes and bind mounts.
// It follows the next sequence to decide what to mount in each final destination:
//
// 1. Select the previously configured mount points for the containers, if any.
// 2. Select the volumes mounted from another containers. Overrides previously configured mount point destination.
// 3. Select the bind mounts set by the client. Overrides previously configured mount point destinations.
func (daemon *Daemon) registerMountPoints(container *Container, hostConfig *runconfig.HostConfig) error {
	binds := map[string]bool{}
	mountPoints := map[string]*mountPoint{}

	// 1. Read already configured mount points.
	for name, point := range container.MountPoints {
		mountPoints[name] = point
	}

	// 2. Read volumes from other containers.
	for _, v := range hostConfig.VolumesFrom {
		containerID, mode, err := parseVolumesFrom(v)
		if err != nil {
			return err
		}

		c, err := daemon.Get(containerID)
		if err != nil {
			return err
		}

		for _, m := range c.MountPoints {
			cp := &mountPoint{
				Name:        m.Name,
				Source:      m.Source,
				RW:          m.RW && volume.ReadWrite(mode),
				Driver:      m.Driver,
				Destination: m.Destination,
			}

			if len(cp.Source) == 0 {
				v, err := daemon.createVolume(cp.Name, cp.Driver, nil)
				if err != nil {
					return err
				}
				cp.Volume = v
			}

			mountPoints[cp.Destination] = cp
		}
	}

	// 3. Read bind mounts
	for _, b := range hostConfig.Binds {
		// #10618
		bind, err := parseBindMount(b, hostConfig.VolumeDriver)
		if err != nil {
			return err
		}

		if binds[bind.Destination] {
			return derr.ErrorCodeVolumeDup.WithArgs(bind.Destination)
		}

		if len(bind.Name) > 0 && len(bind.Driver) > 0 {
			// create the volume
			v, err := daemon.createVolume(bind.Name, bind.Driver, nil)
			if err != nil {
				return err
			}
			bind.Volume = v
			bind.Source = v.Path()
			// bind.Name is an already existing volume, we need to use that here
			bind.Driver = v.DriverName()
			// Since this is just a named volume and not a typical bind, set to shared mode `z`
			if bind.Mode == "" {
				bind.Mode = "z"
			}
		}

		shared := label.IsShared(bind.Mode)
		if err := label.Relabel(bind.Source, container.MountLabel, shared); err != nil {
			return err
		}
		binds[bind.Destination] = true
		mountPoints[bind.Destination] = bind
	}

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

	container.Lock()
	container.MountPoints = mountPoints
	container.Volumes = bcVolumes
	container.VolumesRW = bcVolumesRW
	container.Unlock()

	return nil
}

// createVolume creates a volume.
func (daemon *Daemon) createVolume(name, driverName string, opts map[string]string) (volume.Volume, error) {
	v, err := daemon.volumes.Create(name, driverName, opts)
	if err != nil {
		return nil, err
	}
	daemon.volumes.Increment(v)
	return v, nil
}

// parseVolumeSource parses the origin sources that's mounted into the container.
func parseVolumeSource(spec string) (string, string, error) {
	if !filepath.IsAbs(spec) {
		return spec, "", nil
	}

	return "", spec, nil
}

// BackwardsCompatible decides whether this mount point can be
// used in old versions of Docker or not.
// Only bind mounts and local volumes can be used in old versions of Docker.
func (m *mountPoint) BackwardsCompatible() bool {
	return len(m.Source) > 0 || m.Driver == volume.DefaultDriverName
}
