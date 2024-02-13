//go:build !linux

package libnetwork

func setupArrangeUserFilterRule(c *Controller) {}
func arrangeUserFilterRule()                   {}
func setupUserChain(ipVersion any) error       { return nil }
