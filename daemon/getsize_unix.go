// +build linux freebsd solaris

package daemon

import (
	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/container"
)

// getSize returns the real size & virtual size of the container.
func (daemon *Daemon) getSize(container *container.Container) (int64, int64) {
	var (
		sizeRw, sizeRootfs int64
		err                error
	)

	if err := daemon.Mount(container); err != nil {
		logrus.Errorf("Failed to compute size of container rootfs %s: %s", container.ID, err)
		return sizeRw, sizeRootfs
	}
	defer daemon.Unmount(container)

	sizeRw, err = container.RWLayer.Size()
	if err != nil {
		logrus.Errorf("Driver %s couldn't return diff size of container %s: %s",
			daemon.GraphDriverName(), container.ID, err)
		// FIXME: GetSize should return an error. Not changing it now in case
		// there is a side-effect.
		sizeRw = -1
	}

	if parent := container.RWLayer.Parent(); parent != nil {
		sizeRootfs, err = parent.Size()
		if err != nil {
			sizeRootfs = -1
		} else if sizeRw != -1 {
			sizeRootfs += sizeRw
		}
	}
	return sizeRw, sizeRootfs
}
