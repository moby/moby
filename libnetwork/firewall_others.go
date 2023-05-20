//go:build !linux

package libnetwork

func setupArrangeUserFilterRule(c *Controller) {}
func arrangeUserFilterRule()                   {}
