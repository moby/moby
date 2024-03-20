//go:build !linux

package libnetwork

func (c *Controller) setupFirewallReloadHandler() {}
func setupUserChain(ipVersion any) error          { return nil }
