//go:build !freebsd && !linux && !windows && !darwin

package libnetwork

import "github.com/docker/docker/libnetwork/driverapi"

func registerNetworkDrivers(r driverapi.Registerer, driverConfig func(string) map[string]interface{}) error {
	return nil
}
