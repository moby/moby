package libnetwork

import (
	"context"

	"github.com/docker/docker/libnetwork/osl"
)

// Windows-specific container configuration flags.
type containerConfigOS struct {
	dnsNoProxy bool
}

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
