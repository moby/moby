package netutils

import (
	"net"

	"github.com/docker/docker/libnetwork/types"
)

// FindAvailableNetwork returns a network from the passed list which does not
// overlap with existing interfaces in the system
func FindAvailableNetwork(list []*net.IPNet) (*net.IPNet, error) {
	return nil, types.NotImplementedErrorf("not supported on freebsd")
}
