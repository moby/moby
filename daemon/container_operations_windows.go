// +build windows

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
	return fmt.Errorf("Windows does not support connecting a running container to a network")
}

// DisconnectFromNetwork disconnects container from a network.
func (daemon *Daemon) DisconnectFromNetwork(container *container.Container, n libnetwork.Network, force bool) error {
	return fmt.Errorf("Windows does not support disconnecting a running container from a network")
}

// getSize returns real size & virtual size
func (daemon *Daemon) getSize(container *container.Container) (int64, int64) {
	// TODO Windows
	return 0, 0
}

// setNetworkNamespaceKey is a no-op on Windows.
func (daemon *Daemon) setNetworkNamespaceKey(containerID string, pid int) error {
	return nil
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
