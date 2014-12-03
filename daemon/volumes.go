package daemon

import (
	"fmt"
	"io"
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
	"github.com/docker/docker/volumes"
	"github.com/docker/libcontainer/label"
)

type Mount struct {
	MountToPath string
	container   *Container
	volume      *volumes.Volume
	Writable    bool
	copyData    bool
}

func (mnt *Mount) Export(resource string) (io.ReadCloser, error) {
	var name string
	if resource == mnt.MountToPath[1:] {
		name = filepath.Base(resource)
	}
	path, err := filepath.Rel(mnt.MountToPath[1:], resource)
	if err != nil {
		return nil, err
	}
	return mnt.volume.Export(path, name)
}

func (container *Container) prepareVolumes() error {
	if container.Volumes == nil || len(container.Volumes) == 0 {
		container.Volumes = make(map[string]string)
		container.VolumesRW = make(map[string]bool)
		if err := container.applyVolumesFrom(); err != nil {
			return err
		}
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
	mounts, err := container.parseVolumeMountConfig()
	if err != nil {
		return err
	}

	for _, mnt := range mounts {
		if err := mnt.initialize(); err != nil {
			return err
		}
	}

	return nil
}

func (m *Mount) initialize() error {
	// No need to initialize anything since it's already been initialized
	if _, exists := m.container.Volumes[m.MountToPath]; exists {
		return nil
	}

	// This is the full path to container fs + mntToPath
	containerMntPath, err := symlink.FollowSymlinkInScope(filepath.Join(m.container.basefs, m.MountToPath), m.container.basefs)
	if err != nil {
		return err
	}
	m.container.VolumesRW[m.MountToPath] = m.Writable
	m.container.Volumes[m.MountToPath] = m.volume.Path
	m.volume.AddContainer(m.container.ID)
	if m.Writable && m.copyData {
		// Copy whatever is in the container at the mntToPath to the volume
		copyExistingContents(containerMntPath, m.volume.Path)
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
		mnt.volume.AddContainer(container.ID)
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

func (container *Container) parseVolumeMountConfig() (map[string]*Mount, error) {
	var mounts = make(map[string]*Mount)
	// Get all the bind mounts
	for _, spec := range container.hostConfig.Binds {
		path, mountToPath, writable, err := parseBindMountSpec(spec)
		if err != nil {
			return nil, err
		}
		// Check if a volume already exists for this and use it
		vol, err := container.daemon.volumes.FindOrCreateVolume(path, writable)
		if err != nil {
			return nil, err
		}
		mounts[mountToPath] = &Mount{
			container:   container,
			volume:      vol,
			MountToPath: mountToPath,
			Writable:    writable,
		}
	}

	// Get the rest of the volumes
	for path := range container.Config.Volumes {
		// Check if this is already added as a bind-mount
		path = filepath.Clean(path)
		if _, exists := mounts[path]; exists {
			continue
		}

		// Check if this has already been created
		if _, exists := container.Volumes[path]; exists {
			continue
		}

		vol, err := container.daemon.volumes.FindOrCreateVolume("", true)
		if err != nil {
			return nil, err
		}
		mounts[path] = &Mount{
			container:   container,
			MountToPath: path,
			volume:      vol,
			Writable:    true,
			copyData:    true,
		}
	}

	return mounts, nil
}

func parseBindMountSpec(spec string) (string, string, bool, error) {
	var (
		path, mountToPath string
		writable          bool
		arr               = strings.Split(spec, ":")
	)

	switch len(arr) {
	case 2:
		path = arr[0]
		mountToPath = arr[1]
		writable = true
	case 3:
		path = arr[0]
		mountToPath = arr[1]
		writable = validMountMode(arr[2]) && arr[2] == "rw"
	default:
		return "", "", false, fmt.Errorf("Invalid volume specification: %s", spec)
	}

	if !filepath.IsAbs(path) {
		return "", "", false, fmt.Errorf("cannot bind mount volume: %s volume paths must be absolute.", path)
	}

	path = filepath.Clean(path)
	mountToPath = filepath.Clean(mountToPath)
	return path, mountToPath, writable, nil
}

func (container *Container) applyVolumesFrom() error {
	volumesFrom := container.hostConfig.VolumesFrom

	mountGroups := make([]map[string]*Mount, 0, len(volumesFrom))

	for _, spec := range volumesFrom {
		mountGroup, err := parseVolumesFromSpec(container.daemon, spec)
		if err != nil {
			return err
		}
		mountGroups = append(mountGroups, mountGroup)
	}

	for _, mounts := range mountGroups {
		for _, mnt := range mounts {
			mnt.container = container
			if err := mnt.initialize(); err != nil {
				return err
			}
		}
	}
	return nil
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

func parseVolumesFromSpec(daemon *Daemon, spec string) (map[string]*Mount, error) {
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

func (container *Container) VolumeMounts() map[string]*Mount {
	mounts := make(map[string]*Mount)

	for mountToPath, path := range container.Volumes {
		if v := container.daemon.volumes.Get(path); v != nil {
			mounts[mountToPath] = &Mount{volume: v, container: container, MountToPath: mountToPath, Writable: container.VolumesRW[mountToPath]}
		}
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
