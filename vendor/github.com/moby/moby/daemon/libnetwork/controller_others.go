//go:build !linux

package libnetwork

import "github.com/moby/moby/api/types/system"

// FirewallBackend returns the name of the firewall backend for "docker info".
func (c *Controller) FirewallBackend() *system.FirewallInfo {
	return nil
}

// enabledIptablesVersions is a no-op on non-Linux systems.
func (c *Controller) enabledIptablesVersions() []any {
	return nil
}

func (c *Controller) setupOSLSandbox(_ *Sandbox) error {
	return nil
}
