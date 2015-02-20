package libnetwork

import "fmt"

type DriverParams map[string]interface{}
type DriverConstructor func(DriverParams) (Network, error)

var drivers = map[string]DriverConstructor{}

// RegisterNetworkType associates a textual identifier with a way to create a
// new network. It is called by the various network implementations, and used
// upon invokation of the libnetwork.NetNetwork function.
func RegisterNetworkType(name string, ctor DriverConstructor) error {
	if _, ok := drivers[name]; ok {
		return fmt.Errorf("a driver for network type %q is already registed", name)
	}
	drivers[name] = ctor
	return nil
}
