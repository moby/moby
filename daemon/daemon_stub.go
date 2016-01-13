// +build !experimental

package daemon

import "github.com/docker/engine-api/types/container"

func (daemon *Daemon) verifyExperimentalContainerSettings(hostConfig *container.HostConfig, config *container.Config) ([]string, error) {
	return nil, nil
}
