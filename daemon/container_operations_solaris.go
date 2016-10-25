// +build solaris

package daemon

import "github.com/docker/docker/container"

func (daemon *Daemon) setupLinkedContainers(container *container.Container) ([]string, error) {
	return nil, nil
}

// getSize returns real size & virtual size
func (daemon *Daemon) getSize(container *container.Container) (int64, int64) {
	return 0, 0
}

func (daemon *Daemon) setupIpcDirs(container *container.Container) error {
	return nil
}

func (daemon *Daemon) mountVolumes(container *container.Container) error {
	return nil
}

func killProcessDirectly(container *container.Container) error {
	return nil
}

func detachMounted(path string) error {
	return nil
}

func isLinkable(child *container.Container) bool {
	return false
}

func (daemon *Daemon) isNetworkHotPluggable() bool {
	return false
}
