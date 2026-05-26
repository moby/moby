//go:build !linux && !windows

package libnetwork

import (
	"context"

	"github.com/moby/moby/v2/daemon/libnetwork/osl"
	"github.com/moby/moby/v2/daemon/libnetwork/types"
	"github.com/moby/moby/v2/errdefs"
)

func releaseOSSboxResources(*osl.Namespace, *Endpoint) {}

func (sb *Sandbox) updateGateway(_, _ *Endpoint) error { return nil }

// Statistics is a no-op on platforms without a real OS sandbox implementation.
func (sb *Sandbox) Statistics() (map[string]*types.InterfaceStatistics, error) {
	return map[string]*types.InterfaceStatistics{}, nil
}

func (sb *Sandbox) ExecFunc(func()) error { return nil }

func (sb *Sandbox) releaseOSSbox() error { return nil }

func (sb *Sandbox) restoreOslSandbox(_ context.Context) error { return nil }

func (sb *Sandbox) NetnsPath() (path string, ok bool) { return "", false }

func (sb *Sandbox) canPopulateNetworkResources() bool { return true }

func (sb *Sandbox) populateNetworkResourcesOS(ctx context.Context, ep *Endpoint) error {
	n := ep.getNetwork()
	if err := addEpToResolver(ctx, n.Name(), ep.Name(), &sb.config, ep.iface, n.Resolvers()); err != nil {
		return errdefs.System(err)
	}
	return nil
}

func (sb *Sandbox) IPv6Enabled() (enabled, ok bool) { return false, true }

func addEpToResolver(
	ctx context.Context,
	netName, epName string,
	config *containerConfig,
	epIface *EndpointInterface,
	resolvers []*Resolver,
) error {
	return nil
}
