package daemon

import "github.com/docker/docker/api/types/mount"

func (daemon *Daemon) validateBindDaemonRoot(m mount.Mount) (bool, error) {
	return false, nil
}
