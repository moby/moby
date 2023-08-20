//go:build !linux

package libnetwork

// enabledIptablesVersions is a no-op on non-Linux systems.
func (c *Controller) enabledIptablesVersions() []any {
	return nil
}

func (c *Controller) setupOSLSandbox(_ *Sandbox) error {
	return nil
}
