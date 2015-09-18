package daemon

import (
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/runconfig"
)

// ContainerInspect returns low-level information about a
// container. Returns an error if the container cannot be found, or if
// there is an error getting the data.
func (daemon *Daemon) ContainerInspect(name string, decorator func(*Container, *runconfig.HostConfig, map[string]string) error) error {
	container, err := daemon.Get(name)
	if err != nil {
		return err
	}

	container.Lock()
	defer container.Unlock()

	hostConfig := daemon.inspectHostConfig(container)
	graphDriverData, err := daemon.driver.GetMetadata(container.ID)
	if err != nil {
		return err
	}

	return decorator(container, hostConfig, graphDriverData)
}

func (daemon *Daemon) inspectHostConfig(container *Container) *runconfig.HostConfig {
	// make a copy to play with
	hostConfig := *container.hostConfig

	if children, err := daemon.children(container.Name); err == nil {
		for linkAlias, child := range children {
			hostConfig.Links = append(hostConfig.Links, fmt.Sprintf("%s:%s", child.Name, linkAlias))
		}
	}
	// we need this trick to preserve empty log driver, so
	// container will use daemon defaults even if daemon change them
	if hostConfig.LogConfig.Type == "" {
		hostConfig.LogConfig.Type = daemon.defaultLogConfig.Type
	}

	if len(hostConfig.LogConfig.Config) == 0 {
		hostConfig.LogConfig.Config = daemon.defaultLogConfig.Config
	}

	return &hostConfig
}

// ContainerExecInspect returns low-level information about the exec
// command. An error is returned if the exec cannot be found.
func (daemon *Daemon) ContainerExecInspect(id string) (*ExecConfig, error) {
	eConfig, err := daemon.getExecConfig(id)
	if err != nil {
		return nil, err
	}
	return eConfig, nil
}

// VolumeInspect looks up a volume by name. An error is returned if
// the volume cannot be found.
func (daemon *Daemon) VolumeInspect(name string) (*types.Volume, error) {
	v, err := daemon.volumes.Get(name)
	if err != nil {
		return nil, err
	}
	return volumeToAPIType(v), nil
}
