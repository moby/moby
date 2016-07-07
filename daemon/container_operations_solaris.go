// +build solaris

package daemon

import (
	"fmt"

	"github.com/docker/docker/container"
	networktypes "github.com/docker/engine-api/types/network"
	"github.com/docker/libnetwork"
)

func (daemon *Daemon) setupLinkedContainers(container *container.Container) ([]string, error) {
	return nil, nil
}

// ConnectToNetwork connects a container to a network
func (daemon *Daemon) ConnectToNetwork(container *container.Container, idOrName string, endpointConfig *networktypes.EndpointSettings) error {
	return fmt.Errorf("Solaris does not support connecting a running container to a network")
}

// getSize returns real size & virtual size
func (daemon *Daemon) getSize(container *container.Container) (int64, int64) {
	return 0, 0
}

// DisconnectFromNetwork disconnects a container from the network
func (daemon *Daemon) DisconnectFromNetwork(container *container.Container, n libnetwork.Network, force bool) error {
	return fmt.Errorf("Solaris does not support disconnecting a running container from a network")
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
