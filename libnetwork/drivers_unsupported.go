//go:build !freebsd && !linux && !windows

package libnetwork

func registerNetworkDrivers(r driverapi.Registerer, driverConfig func(string) map[string]interface{}) error {
	return nil
}
