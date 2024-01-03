package netutils

import (
	"net"
)

// FindAvailableNetwork returns a network from the passed list which does not
// overlap with existing interfaces in the system
//
// TODO : Use appropriate windows APIs to identify non-overlapping subnets
func FindAvailableNetwork(list []*net.IPNet) (*net.IPNet, error) {
	return nil, nil
}
