//go:build !linux

package libnetwork

import "errors"

func ValidateFirewallBackend(val string) error {
	if val != "" {
		return errors.New("firewall-backend can only be configured on Linux")
	}
	return nil
}

func (c *Controller) selectFirewallBackend() error {
	return nil
}

func (c *Controller) setupUserChains() {}
