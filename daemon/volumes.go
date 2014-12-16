package daemon

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/execdriver"
	"github.com/docker/docker/pkg/chrootarchive"
	"github.com/docker/docker/pkg/symlink"
	"github.com/docker/libcontainer/label"
)

type VolumeMount struct {
	ToPath   string
	FromPath string
	Writable bool
	copyData bool
}

func (container *Container) prepareVolumes() error {
	if container.Volumes == nil || len(container.Volumes) == 0 {
		container.Volumes = make(map[string]string)
		container.VolumesRW = make(map[string]bool)
	}

	return container.createVolumes()
}

// sortedVolumeMounts returns the list of container volume mount points sorted in lexicographic order
func (container *Container) sortedVolumeMounts() []string {
	var mountPaths []string
	for path := range container.Volumes {
		mountPaths = append(mountPaths, path)
	}

	sort.Strings(mountPaths)
	return mountPaths
}

func (container *Container) createVolumes() error {
	mounts, err := container.parseVolumeConfig()
	if err != nil {
		return err
	}

	for _, mnt := range mounts {
		// Check if this already exists and skip
		// Don't skip if it is a bind-mount or a volumes-from
		if hostPath, exists := container.Volumes[mnt.ToPath]; exists && mnt.FromPath == "" {
			continue
		} else if v := container.daemon.volumes.Get(hostPath); v != nil && v.Path != mnt.FromPath {
			// If the mnt.FromPath does not match the registered volume in container.Volumes, the volume mount is going to be overwritten
			// For this, we need to deref the volume record and try to delete it.  No need to check for errors here since this is a simple cleanup, maybe the volume was is being used by another container at this point.
			v.RemoveContainer(container.ID)
			container.daemon.volumes.Delete(v.Path)
		}

		// This is the full path to container fs + mntToPath
		containerMntPath, err := symlink.FollowSymlinkInScope(filepath.Join(container.basefs, mnt.ToPath), container.basefs)
		if err != nil {
			return err
		}

		// Create the actual volume
		v, err := container.daemon.volumes.FindOrCreateVolume(mnt.FromPath, mnt.Writable)
		if err != nil {
			return err
		}

		container.VolumesRW[mnt.ToPath] = mnt.Writable
		container.Volumes[mnt.ToPath] = v.Path
		v.AddContainer(container.ID)

		if mnt.Writable && mnt.copyData {
			// Copy whatever is in the container at the mntToPath to the volume
			copyExistingContents(containerMntPath, v.Path)
		}
	}

	return nil
}

func (container *Container) VolumePaths() map[string]struct{} {
	var paths = make(map[string]struct{})
	for _, path := range container.Volumes {
		paths[path] = struct{}{}
	}
	return paths
}

func (container *Container) registerVolumes() {
	for _, mnt := range container.VolumeMounts() {
		if v := container.daemon.volumes.Get(mnt.FromPath); v != nil {
			v.AddContainer(container.ID)
		}
	}
}

func (container *Container) derefVolumes() {
	for path := range container.VolumePaths() {
		vol := container.daemon.volumes.Get(path)
		if vol == nil {
			log.Debugf("Volume %s was not found and could not be dereferenced", path)
			continue
		}
		vol.RemoveContainer(container.ID)
	}
}

func (container *Container) parseVolumeConfig() ([]*VolumeMount, error) {
	var (
		mounts []*VolumeMount
		paths  = make(map[string]struct{})
	)

	// Get volumes-from
	for _, spec := range container.hostConfig.VolumesFrom {
		volumesFrom, err := parseVolumesFromSpec(container.daemon, spec)
		if err != nil {
			return nil, err
		}
		for _, mnt := range volumesFrom {
			paths[mnt.ToPath] = struct{}{}
			mounts = append(mounts, mnt)
		}
	}

	// Get all the bind mounts
	for _, spec := range container.hostConfig.Binds {
		var err error
		mnt := &VolumeMount{}
		mnt.FromPath, mnt.ToPath, mnt.Writable, err = parseBindMountSpec(spec)
		if err != nil {
			return nil, err
		}
		if _, exists := paths[mnt.ToPath]; exists {
			// skip since volumes-from have priority
			continue
		}
		paths[mnt.ToPath] = struct{}{}
		mounts = append(mounts, mnt)
	}

	// Get the rest of the volumes
	for path := range container.Config.Volumes {
		// skip if this is already added from binds or volumes-from
		if _, exists := paths[path]; exists {
			continue
		}
		paths[path] = struct{}{}
		mounts = append(mounts, &VolumeMount{
			ToPath:   filepath.Clean(path),
			Writable: true,
			copyData: true,
		})
	}

	return mounts, nil
}

func parseBindMountSpec(spec string) (string, string, bool, error) {
	var (
		path, toPath string
		writable     bool
		arr          = strings.Split(spec, ":")
	)

	switch len(arr) {
	case 2:
		path = arr[0]
		toPath = arr[1]
		writable = true
	case 3:
		path = arr[0]
		toPath = arr[1]
		writable = validMountMode(arr[2]) && arr[2] == "rw"
	default:
		return "", "", false, fmt.Errorf("Invalid volume specification: %s", spec)
	}

	if !filepath.IsAbs(path) {
		return "", "", false, fmt.Errorf("cannot bind mount volume: %s volume paths must be absolute.", path)
	}

	path = filepath.Clean(path)
	toPath = filepath.Clean(toPath)
	return path, toPath, writable, nil
}

func validMountMode(mode string) bool {
	validModes := map[string]bool{
		"rw": true,
		"ro": true,
	}

	return validModes[mode]
}

func (container *Container) setupMounts() error {
	mounts := []execdriver.Mount{
		{Source: container.ResolvConfPath, Destination: "/etc/resolv.conf", Writable: true, Private: true},
	}

	if container.HostnamePath != "" {
		mounts = append(mounts, execdriver.Mount{Source: container.HostnamePath, Destination: "/etc/hostname", Writable: true, Private: true})
	}

	if container.HostsPath != "" {
		mounts = append(mounts, execdriver.Mount{Source: container.HostsPath, Destination: "/etc/hosts", Writable: true, Private: true})
	}

	for _, m := range mounts {
		if err := label.SetFileLabel(m.Source, container.MountLabel); err != nil {
			return err
		}
	}

	// Mount user specified volumes
	// Note, these are not private because you may want propagation of (un)mounts from host
	// volumes. For instance if you use -v /usr:/usr and the host later mounts /usr/share you
	// want this new mount in the container
	// These mounts must be ordered based on the length of the path that it is being mounted to (lexicographic)
	for _, path := range container.sortedVolumeMounts() {
		mounts = append(mounts, execdriver.Mount{
			Source:      container.Volumes[path],
			Destination: path,
			Writable:    container.VolumesRW[path],
		})
	}

	container.command.Mounts = mounts
	return nil
}

func parseVolumesFromSpec(daemon *Daemon, spec string) (map[string]*VolumeMount, error) {
	specParts := strings.SplitN(spec, ":", 2)
	if len(specParts) == 0 {
		return nil, fmt.Errorf("Malformed volumes-from specification: %s", spec)
	}

	c := daemon.Get(specParts[0])
	if c == nil {
		return nil, fmt.Errorf("Container %s not found. Impossible to mount its volumes", specParts[0])
	}

	mounts := c.VolumeMounts()

	if len(specParts) == 2 {
		mode := specParts[1]
		if !validMountMode(mode) {
			return nil, fmt.Errorf("Invalid mode for volumes-from: %s", mode)
		}

		// Set the mode for the inheritted volume
		for _, mnt := range mounts {
			// Ensure that if the inherited volume is not writable, that we don't make
			// it writable here
			mnt.Writable = mnt.Writable && (mode == "rw")
		}
	}

	return mounts, nil
}

func (container *Container) VolumeMounts() map[string]*VolumeMount {
	mounts := make(map[string]*VolumeMount)

	for toPath, path := range container.Volumes {
		v := container.daemon.volumes.Get(path)
		if v == nil {
			// This should never happen
			log.Debugf("reference by container %s to non-existent volume path %s", container.ID, path)
			continue
		}
		mounts[toPath] = &VolumeMount{FromPath: path, ToPath: toPath, Writable: container.VolumesRW[toPath]}
	}

	return mounts
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

// copyOwnership copies the permissions and uid:gid of the source file
// into the destination file
func copyOwnership(source, destination string) error {
	var stat syscall.Stat_t

	if err := syscall.Stat(source, &stat); err != nil {
		return err
	}

	if err := os.Chown(destination, int(stat.Uid), int(stat.Gid)); err != nil {
		return err
	}

	return os.Chmod(destination, os.FileMode(stat.Mode))
}
