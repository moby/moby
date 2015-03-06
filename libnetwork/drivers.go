package libnetwork

import (
	"fmt"

	"github.com/docker/libnetwork/pkg/options"
)

// DriverParams are a generic structure to hold driver specific settings.
type DriverParams options.Generic

// DriverInterface is an interface that every plugin driver needs to implement.
type DriverInterface interface {
	CreateNetwork(string, interface{}) (Network, error)
}

var drivers = map[string]struct {
	creatorFn  DriverInterface
	creatorArg interface{}
}{}

// RegisterNetworkType associates a textual identifier with a way to create a
// new network. It is called by the various network implementations, and used
// upon invokation of the libnetwork.NetNetwork function.
//
// creatorFn must implement DriverInterface
//
// For example:
//
//    type driver struct{}
//
//    func (d *driver) CreateNetwork(name string, config *TestNetworkConfig) (Network, error) {
//    }
//
//    func init() {
//        RegisterNetworkType("test", &driver{}, &TestNetworkConfig{})
//    }
//
func RegisterNetworkType(name string, creatorFn DriverInterface, creatorArg interface{}) error {
	// Store the new driver information to invoke at creation time.
	if _, ok := drivers[name]; ok {
		return fmt.Errorf("a driver for network type %q is already registed", name)
	}

	drivers[name] = struct {
		creatorFn  DriverInterface
		creatorArg interface{}
	}{creatorFn, creatorArg}

	return nil
}

func createNetwork(networkType, name string, generic DriverParams) (Network, error) {
	d, ok := drivers[networkType]
	if !ok {
		return nil, fmt.Errorf("unknown driver %q", networkType)
	}

	config, err := options.GenerateFromModel(options.Generic(generic), d.creatorArg)
	if err != nil {
		return nil, fmt.Errorf("failed to generate driver config: %v", err)
	}

	return d.creatorFn.CreateNetwork(name, config)
}
