package libnetwork

import (
	"context"

	"github.com/moby/moby/v2/daemon/libnetwork/osl"
	"github.com/moby/moby/v2/errdefs"
)

func releaseOSSboxResources(*osl.Namespace, *Endpoint) {}

func (sb *Sandbox) updateGateway(_, _ *Endpoint) error {
	// not implemented on Windows (Sandbox.osSbox is always nil)
	return nil
}

func (sb *Sandbox) ExecFunc(func()) error {
	// not implemented on Windows (Sandbox.osSbox is always nil)
	return nil
}

func (sb *Sandbox) releaseOSSbox() error {
	// not implemented on Windows (Sandbox.osSbox is always nil)
	return nil
}

func (sb *Sandbox) restoreOslSandbox() error {
	// not implemented on Windows (Sandbox.osSbox is always nil)
	return nil
}

// NetnsPath is not implemented on Windows (Sandbox.osSbox is always nil)
func (sb *Sandbox) NetnsPath() (path string, ok bool) {
	return "", false
}

func (sb *Sandbox) canPopulateNetworkResources() bool {
	return true
}

func (sb *Sandbox) populateNetworkResourcesOS(ctx context.Context, ep *Endpoint) error {
	n := ep.getNetwork()
	if err := addEpToResolver(ctx, n.Name(), ep.Name(), &sb.config, ep.iface, n.Resolvers()); err != nil {
		return errdefs.System(err)
	}
	return nil
}

// IPv6Enabled always returns false on Windows as None of the Windows container
// network drivers currently support IPv6.
func (sb *Sandbox) IPv6Enabled() (enabled, ok bool) {
	return false, true
}
