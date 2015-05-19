package daemon

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/execdriver"
	"github.com/docker/docker/pkg/chrootarchive"
	"github.com/docker/docker/pkg/mount"
	"github.com/docker/docker/pkg/symlink"
)

type volumeMount struct {
	containerPath string
	hostPath      string
	writable      bool
	copyData      bool
	from          string
}

func (container *Container) createVolumes() error {
	mounts := make(map[string]*volumeMount)

	// get the normal volumes
	for path := range container.Config.Volumes {
		path = filepath.Clean(path)
		// skip if there is already a volume for this container path
		if _, exists := container.Volumes[path]; exists {
			continue
		}

		realPath, err := container.GetResourcePath(path)
		if err != nil {
			return err
		}
		if stat, err := os.Stat(realPath); err == nil {
			if !stat.IsDir() {
				return fmt.Errorf("can't mount to container path, file exists - %s", path)
			}
		}

		mnt := &volumeMount{
			containerPath: path,
			writable:      true,
			copyData:      true,
		}
		mounts[mnt.containerPath] = mnt
	}

	// Get all the bind mounts
	// track bind paths separately due to #10618
	bindPaths := make(map[string]struct{})
	for _, spec := range container.hostConfig.Binds {
		mnt, err := parseBindMountSpec(spec)
		if err != nil {
			return err
		}

		// #10618
		if _, exists := bindPaths[mnt.containerPath]; exists {
			return fmt.Errorf("Duplicate volume mount %s", mnt.containerPath)
		}

		bindPaths[mnt.containerPath] = struct{}{}
		mounts[mnt.containerPath] = mnt
	}

	// Get volumes from
	for _, from := range container.hostConfig.VolumesFrom {
		cID, mode, err := parseVolumesFromSpec(from)
		if err != nil {
			return err
		}
		if _, exists := container.AppliedVolumesFrom[cID]; exists {
			// skip since it's already been applied
			continue
		}

		c, err := container.daemon.Get(cID)
		if err != nil {
			return fmt.Errorf("container %s not found, impossible to mount its volumes", cID)
		}

		for _, mnt := range c.volumeMounts() {
			mnt.writable = mnt.writable && (mode == "rw")
			mnt.from = cID
			mounts[mnt.containerPath] = mnt
		}
	}

	for _, mnt := range mounts {
		containerMntPath, err := symlink.FollowSymlinkInScope(filepath.Join(container.basefs, mnt.containerPath), container.basefs)
		if err != nil {
			return err
		}

		// Create the actual volume
		v, err := container.daemon.volumes.FindOrCreateVolume(mnt.hostPath, mnt.writable)
		if err != nil {
			return err
		}

		container.VolumesRW[mnt.containerPath] = mnt.writable
		container.Volumes[mnt.containerPath] = v.Path
		v.AddContainer(container.ID)
		if mnt.from != "" {
			container.AppliedVolumesFrom[mnt.from] = struct{}{}
		}

		if mnt.writable && mnt.copyData {
			// Copy whatever is in the container at the containerPath to the volume
			copyExistingContents(containerMntPath, v.Path)
		}
	}

	return nil
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

func (container *Container) VolumePaths() map[string]struct{} {
	var paths = make(map[string]struct{})
	for _, path := range container.Volumes {
		paths[path] = struct{}{}
	}
	return paths
}

func (container *Container) registerVolumes() {
	for path := range container.VolumePaths() {
		if v := container.daemon.volumes.Get(path); v != nil {
			v.AddContainer(container.ID)
			continue
		}

		// if container was created with an old daemon, this volume may not be registered so we need to make sure it gets registered
		writable := true
		if rw, exists := container.VolumesRW[path]; exists {
			writable = rw
		}
		v, err := container.daemon.volumes.FindOrCreateVolume(path, writable)
		if err != nil {
			logrus.Debugf("error registering volume %s: %v", path, err)
			continue
		}
		v.AddContainer(container.ID)
	}
}

func (container *Container) derefVolumes() {
	for path := range container.VolumePaths() {
		vol := container.daemon.volumes.Get(path)
		if vol == nil {
			logrus.Debugf("Volume %s was not found and could not be dereferenced", path)
			continue
		}
		vol.RemoveContainer(container.ID)
	}
}

func parseBindMountSpec(spec string) (*volumeMount, error) {
	arr := strings.Split(spec, ":")

	mnt := &volumeMount{}
	switch len(arr) {
	case 2:
		mnt.hostPath = arr[0]
		mnt.containerPath = arr[1]
		mnt.writable = true
	case 3:
		mnt.hostPath = arr[0]
		mnt.containerPath = arr[1]
		mnt.writable = validMountMode(arr[2]) && arr[2] == "rw"
	default:
		return nil, fmt.Errorf("Invalid volume specification: %s", spec)
	}

	if !filepath.IsAbs(mnt.hostPath) {
		return nil, fmt.Errorf("cannot bind mount volume: %s volume paths must be absolute.", mnt.hostPath)
	}

	mnt.hostPath = filepath.Clean(mnt.hostPath)
	mnt.containerPath = filepath.Clean(mnt.containerPath)
	return mnt, nil
}

func parseVolumesFromSpec(spec string) (string, string, error) {
	specParts := strings.SplitN(spec, ":", 2)
	if len(specParts) == 0 {
		return "", "", fmt.Errorf("malformed volumes-from specification: %s", spec)
	}

	var (
		id   = specParts[0]
		mode = "rw"
	)
	if len(specParts) == 2 {
		mode = specParts[1]
		if !validMountMode(mode) {
			return "", "", fmt.Errorf("invalid mode for volumes-from: %s", mode)
		}
	}
	return id, mode, nil
}

func validMountMode(mode string) bool {
	validModes := map[string]bool{
		"rw": true,
		"ro": true,
	}

	return validModes[mode]
}

func (container *Container) specialMounts() []execdriver.Mount {
	var mounts []execdriver.Mount
	if container.ResolvConfPath != "" {
		mounts = append(mounts, execdriver.Mount{Source: container.ResolvConfPath, Destination: "/etc/resolv.conf", Writable: !container.hostConfig.ReadonlyRootfs, Private: true})
	}
	if container.HostnamePath != "" {
		mounts = append(mounts, execdriver.Mount{Source: container.HostnamePath, Destination: "/etc/hostname", Writable: !container.hostConfig.ReadonlyRootfs, Private: true})
	}
	if container.HostsPath != "" {
		mounts = append(mounts, execdriver.Mount{Source: container.HostsPath, Destination: "/etc/hosts", Writable: !container.hostConfig.ReadonlyRootfs, Private: true})
	}
	return mounts
}

func (container *Container) volumeMounts() map[string]*volumeMount {
	mounts := make(map[string]*volumeMount)

	for containerPath, path := range container.Volumes {
		v := container.daemon.volumes.Get(path)
		if v == nil {
			// This should never happen
			logrus.Debugf("reference by container %s to non-existent volume path %s", container.ID, path)
			continue
		}
		mounts[containerPath] = &volumeMount{hostPath: path, containerPath: containerPath, writable: container.VolumesRW[containerPath]}
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

func (container *Container) mountVolumes() error {
	for dest, source := range container.Volumes {
		v := container.daemon.volumes.Get(source)
		if v == nil {
			return fmt.Errorf("could not find volume for %s:%s, impossible to mount", source, dest)
		}

		destPath, err := container.GetResourcePath(dest)
		if err != nil {
			return err
		}

		if err := mount.Mount(source, destPath, "bind", "rbind,rw"); err != nil {
			return fmt.Errorf("error while mounting volume %s: %v", source, err)
		}
	}

	for _, mnt := range container.specialMounts() {
		destPath, err := container.GetResourcePath(mnt.Destination)
		if err != nil {
			return err
		}
		if err := mount.Mount(mnt.Source, destPath, "bind", "bind,rw"); err != nil {
			return fmt.Errorf("error while mounting volume %s: %v", mnt.Source, err)
		}
	}
	return nil
}

func (container *Container) unmountVolumes() {
	for dest := range container.Volumes {
		destPath, err := container.GetResourcePath(dest)
		if err != nil {
			logrus.Errorf("error while unmounting volumes %s: %v", destPath, err)
			continue
		}
		if err := mount.ForceUnmount(destPath); err != nil {
			logrus.Errorf("error while unmounting volumes %s: %v", destPath, err)
			continue
		}
	}

	for _, mnt := range container.specialMounts() {
		destPath, err := container.GetResourcePath(mnt.Destination)
		if err != nil {
			logrus.Errorf("error while unmounting volumes %s: %v", destPath, err)
			continue
		}
		if err := mount.ForceUnmount(destPath); err != nil {
			logrus.Errorf("error while unmounting volumes %s: %v", destPath, err)
		}
	}
}
