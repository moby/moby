// +build windows

package daemon

import (
	"github.com/docker/docker/daemon/execdriver"
	"github.com/docker/docker/runconfig"
)

// Not supported on Windows
func copyOwnership(source, destination string) error {
	return nil
}

func (container *Container) setupMounts() ([]execdriver.Mount, error) {
	return nil, nil
}

// verifyVolumesInfo ports volumes configured for the containers pre docker 1.7.
// As the Windows daemon was not supported before 1.7, this is a no-op
func (daemon *Daemon) verifyVolumesInfo(container *Container) error {
	return nil
}

// TODO Windows: This can be further factored out. Called from daemon\daemon.go
func (daemon *Daemon) registerMountPoints(container *Container, hostConfig *runconfig.HostConfig) error {
	return nil
}
