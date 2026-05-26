//go:build !linux && !windows

package cnmallocator

import (
	"github.com/moby/moby/v2/daemon/libnetwork/driverapi"
	"github.com/moby/swarmkit/v2/manager/allocator/networkallocator"
)

var globalDrivers = map[string]func(driverapi.Registerer) error{}

var localDrivers []string

func (*Provider) PredefinedNetworks() []networkallocator.PredefinedNetworkData {
	return nil
}
