//go:build !linux

package libnetwork

// FirewallBackend returns the name of the firewall backend for "docker info".
func (c *Controller) FirewallBackend() string {
	return ""
}

// enabledIptablesVersions is a no-op on non-Linux systems.
func (c *Controller) enabledIptablesVersions() []any {
	return nil
}

func (c *Controller) setupOSLSandbox(_ *Sandbox) error {
	return nil
}
