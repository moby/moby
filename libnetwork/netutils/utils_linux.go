//go:build linux
// +build linux

// Network utility functions.

package netutils

import (
	"net"
	"os"

	"github.com/docker/docker/libnetwork/ns"
	"github.com/docker/docker/libnetwork/resolvconf"
	"github.com/docker/docker/libnetwork/types"
	"github.com/pkg/errors"
	"github.com/vishvananda/netlink"
)

var networkGetRoutesFct func(netlink.Link, int) ([]netlink.Route, error)

// CheckRouteOverlaps checks whether the passed network overlaps with any existing routes
func CheckRouteOverlaps(toCheck *net.IPNet) error {
	networkGetRoutesFct := networkGetRoutesFct
	if networkGetRoutesFct == nil {
		networkGetRoutesFct = ns.NlHandle().RouteList
	}
	networks, err := networkGetRoutesFct(nil, netlink.FAMILY_V4)
	if err != nil {
		return err
	}
	for _, network := range networks {
		if network.Dst != nil && network.Scope == netlink.SCOPE_LINK && NetworkOverlaps(toCheck, network.Dst) {
			return ErrNetworkOverlaps
		}
	}
	return nil
}

// GenerateIfaceName returns an interface name using the passed in
// prefix and the length of random bytes. The api ensures that the
// there are is no interface which exists with that name.
func GenerateIfaceName(nlh *netlink.Handle, prefix string, len int) (string, error) {
	linkByName := netlink.LinkByName
	if nlh != nil {
		linkByName = nlh.LinkByName
	}
	for i := 0; i < 3; i++ {
		name, err := GenerateRandomName(prefix, len)
		if err != nil {
			return "", err
		}
		_, err = linkByName(name)
		if err != nil {
			if errors.As(err, &netlink.LinkNotFoundError{}) {
				return name, nil
			}
			return "", err
		}
	}
	return "", types.InternalErrorf("could not generate interface name")
}

// FindAvailableNetwork returns a network from the passed list which does not
// overlap with existing interfaces in the system
func FindAvailableNetwork(list []*net.IPNet) (*net.IPNet, error) {
	// We don't check for an error here, because we don't really care if we
	// can't read /etc/resolv.conf. So instead we skip the append if resolvConf
	// is nil. It either doesn't exist, or we can't read it for some reason.
	var nameservers []string
	if rc, err := os.ReadFile(resolvconf.Path()); err == nil {
		nameservers = resolvconf.GetNameserversAsCIDR(rc)
	}
	for _, nw := range list {
		if err := CheckNameserverOverlaps(nameservers, nw); err == nil {
			if err := CheckRouteOverlaps(nw); err == nil {
				return nw, nil
			}
		}
	}
	return nil, errors.New("no available network")
}
