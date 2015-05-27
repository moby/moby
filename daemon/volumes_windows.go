// +build windows

package daemon

import "github.com/docker/docker/daemon/execdriver"

// Not supported on Windows
func copyOwnership(source, destination string) error {
	return nil
}

func (container *Container) setupMounts() ([]execdriver.Mount, error) {
	return nil, nil
}
