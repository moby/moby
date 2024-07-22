package libnetwork

import (
	"context"

	"github.com/docker/docker/libnetwork/osl"
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

func (sb *Sandbox) populateNetworkResources(context.Context, *Endpoint) error {
	// not implemented on Windows (Sandbox.osSbox is always nil)
	return nil
}

// IPv6Enabled always returns false on Windows as None of the Windows container
// network drivers currently support IPv6.
func (sb *Sandbox) IPv6Enabled() (enabled, ok bool) {
	return false, true
}
