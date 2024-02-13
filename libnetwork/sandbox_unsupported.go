//go:build !linux

package libnetwork

import "github.com/docker/docker/libnetwork/osl"

func releaseOSSboxResources(*osl.Namespace, *Endpoint) {}

func (sb *Sandbox) updateGateway(*Endpoint) error {
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

func (sb *Sandbox) populateNetworkResources(*Endpoint) error {
	// not implemented on Windows (Sandbox.osSbox is always nil)
	return nil
}
