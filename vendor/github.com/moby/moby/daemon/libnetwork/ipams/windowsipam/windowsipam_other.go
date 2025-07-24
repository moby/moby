//go:build !windows

package windowsipam

import "github.com/moby/moby/daemon/libnetwork/ipamapi"

// Register is a no-op -- windowsipam is only supported on Windows.
func Register(ipamapi.Registerer) error {
	return nil
}
