package libnetwork

import "github.com/docker/docker/libnetwork/driverapi"

func (n *Network) hasEndpointCapability() bool {
	// TODO(aker): remove after the next MCR LTS has been released
	if n.driverCaps == (driverapi.Capability{}) {
		return n.Type() != "host" && n.Type() != "null"
	}
	return n.driverCaps.EndpointDriver
}
