// +build windows

package daemon

import "github.com/docker/docker/container"

func (daemon *Daemon) setupLinkedContainers(container *container.Container) ([]string, error) {
	return nil, nil
}

// getSize returns real size & virtual size
func (daemon *Daemon) getSize(container *container.Container) (int64, int64) {
	// TODO Windows
	return 0, 0
}

func (daemon *Daemon) setupIpcDirs(container *container.Container) error {
	return nil
}

// TODO Windows: Fix Post-TP5. This is a hack to allow docker cp to work
// against containers which have volumes. You will still be able to cp
// to somewhere on the container drive, but not to any mounted volumes
// inside the container. Without this fix, docker cp is broken to any
// container which has a volume, regardless of where the file is inside the
// container.
func (daemon *Daemon) mountVolumes(container *container.Container) error {
	return nil
}

func detachMounted(path string) error {
	return nil
}

func killProcessDirectly(container *container.Container) error {
	return nil
}

func isLinkable(child *container.Container) bool {
	return false
}

func enableIPOnPredefinedNetwork() bool {
	return true
}

func (daemon *Daemon) isNetworkHotPluggable() bool {
	return false
}
