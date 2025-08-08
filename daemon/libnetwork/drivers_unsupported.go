//go:build !freebsd && !linux && !windows

package libnetwork

import "github.com/moby/moby/v2/daemon/libnetwork/driverapi"

func registerNetworkDrivers(r driverapi.Registerer, driverConfig func(string) map[string]any) error {
	return nil
}
