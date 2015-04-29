// +build !windows

package daemon

import (
	"os"

	"github.com/docker/docker/daemon/execdriver"
	"github.com/docker/docker/pkg/system"
)

// copyOwnership copies the permissions and uid:gid of the source file
// into the destination file
func copyOwnership(source, destination string) error {
	stat, err := system.Stat(source)
	if err != nil {
		return err
	}

	if err := os.Chown(destination, int(stat.Uid()), int(stat.Gid())); err != nil {
		return err
	}

	return os.Chmod(destination, os.FileMode(stat.Mode()))
}

func (container *Container) prepareVolumes() error {
	if container.Volumes == nil || len(container.Volumes) == 0 {
		container.Volumes = make(map[string]string)
		container.VolumesRW = make(map[string]bool)
	}

	if len(container.hostConfig.VolumesFrom) > 0 && container.AppliedVolumesFrom == nil {
		container.AppliedVolumesFrom = make(map[string]struct{})
	}
	return container.createVolumes()
}

func (container *Container) setupMounts() error {
	mounts := []execdriver.Mount{}

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

	mounts = append(mounts, container.specialMounts()...)

	container.command.Mounts = mounts
	return nil
}
