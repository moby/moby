// +build linux
// Network utility functions.

package netutils

import (
	"net"
	"strings"

	"github.com/docker/libnetwork/types"
	"github.com/vishvananda/netlink"
)

var (
	networkGetRoutesFct = netlink.RouteList
)

// CheckRouteOverlaps checks whether the passed network overlaps with any existing routes
func CheckRouteOverlaps(toCheck *net.IPNet) error {
	networks, err := networkGetRoutesFct(nil, netlink.FAMILY_V4)
	if err != nil {
		return err
	}

	for _, network := range networks {
		if network.Dst != nil && NetworkOverlaps(toCheck, network.Dst) {
			return ErrNetworkOverlaps
		}
	}
	return nil
}

// GenerateIfaceName returns an interface name using the passed in
// prefix and the length of random bytes. The api ensures that the
// there are is no interface which exists with that name.
func GenerateIfaceName(prefix string, len int) (string, error) {
	for i := 0; i < 3; i++ {
		name, err := GenerateRandomName(prefix, len)
		if err != nil {
			continue
		}
		if _, err := netlink.LinkByName(name); err != nil {
			if strings.Contains(err.Error(), "not found") {
				return name, nil
			}
			return "", err
		}
	}
	return "", types.InternalErrorf("could not generate interface name")
}
