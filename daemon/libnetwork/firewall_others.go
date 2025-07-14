//go:build !linux

package libnetwork

func (c *Controller) selectFirewallBackend() {}

func (c *Controller) setupUserChains() {}
