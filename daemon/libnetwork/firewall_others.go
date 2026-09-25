//go:build !linux

package libnetwork

func (c *Controller) selectFirewallBackend() error {
	return nil
}

func (c *Controller) setupPlatformFirewall() {}
