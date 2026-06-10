//go:build !linux && !freebsd && !windows

package daemon

import (
	"context"

	"github.com/moby/moby/v2/daemon/config"
	"github.com/moby/moby/v2/daemon/container"
	"github.com/moby/moby/v2/daemon/libnetwork"
	"github.com/moby/moby/v2/daemon/network"
)

func buildSandboxPlatformOptions(*container.Container, *config.Config) ([]libnetwork.SandboxOption, error) {
	return nil, nil
}

func (daemon *Daemon) initializeNetworkingPaths(*container.Container, *container.Container) error {
	return nil
}

func enableIPOnPredefinedNetwork() bool {
	return false
}

// serviceDiscoveryOnDefaultNetwork indicates if service discovery is supported on the default network.
func serviceDiscoveryOnDefaultNetwork() bool {
	return false
}

func (daemon *Daemon) addLegacyLinks(context.Context, *config.Config, *container.Container, *network.EndpointSettings, *libnetwork.Sandbox) error {
	return nil
}

func (daemon *Daemon) setupLinkedContainers(*container.Container) ([]string, error) {
	return nil, nil
}

func killProcessDirectly(*container.Container) error {
	return nil
}
