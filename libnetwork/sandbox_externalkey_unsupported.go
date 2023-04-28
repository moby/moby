//go:build !linux && !freebsd
// +build !linux,!freebsd

package libnetwork

// no-op on non linux systems
func (c *Controller) startExternalKeyListener() error {
	return nil
}

func (c *Controller) stopExternalKeyListener() {}
