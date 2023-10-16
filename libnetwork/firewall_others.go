//go:build !linux

package libnetwork

func (c *Controller) setupArrangeUserFilterRule() {}
func setupUserChain(ipVersion any) error          { return nil }
