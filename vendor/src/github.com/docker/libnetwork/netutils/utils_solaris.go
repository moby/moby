package netutils

// Solaris: TODO

import (
	"net"

	"github.com/docker/libnetwork/ipamutils"
)

// ElectInterfaceAddresses looks for an interface on the OS with the specified name
// and returns its IPv4 and IPv6 addresses in CIDR form. If the interface does not exist,
// it chooses from a predifined list the first IPv4 address which does not conflict
// with other interfaces on the system.
func ElectInterfaceAddresses(name string) (*net.IPNet, []*net.IPNet, error) {
	var (
		v4Net *net.IPNet
		err   error
	)

	v4Net, err = FindAvailableNetwork(ipamutils.PredefinedBroadNetworks)
	if err != nil {
		return nil, nil, err
	}
	return v4Net, nil, nil
}

// FindAvailableNetwork returns a network from the passed list which does not
// overlap with existing interfaces in the system
func FindAvailableNetwork(list []*net.IPNet) (*net.IPNet, error) {
	return list[0], nil
}
