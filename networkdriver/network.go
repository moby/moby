package networkdriver

import (
	"encoding/binary"
	"errors"
	"github.com/dotcloud/docker/pkg/netlink"
	"net"
)

var (
	ErrNetworkOverlapsWithNameservers = errors.New("requested network overlaps with nameserver")
	ErrNetworkOverlaps                = errors.New("requested network overlaps with existing network")
)

func CheckNameserverOverlaps(nameservers []string, toCheck *net.IPNet) error {
	if len(nameservers) > 0 {
		for _, ns := range nameservers {
			_, nsNetwork, err := net.ParseCIDR(ns)
			if err != nil {
				return err
			}
			if NetworkOverlaps(toCheck, nsNetwork) {
				return ErrNetworkOverlapsWithNameservers
			}
		}
	}
	return nil
}

func CheckRouteOverlaps(toCheck *net.IPNet) error {
	networks, err := netlink.NetworkGetRoutes()
	if err != nil {
		return err
	}

	for _, network := range networks {
		if network.IPNet != nil && NetworkOverlaps(toCheck, network.IPNet) {
			return ErrNetworkOverlaps
		}
	}
	return nil
}

// Detects overlap between one IPNet and another
func NetworkOverlaps(netX *net.IPNet, netY *net.IPNet) bool {
	if firstIP, _ := NetworkRange(netX); netY.Contains(firstIP) {
		return true
	}
	if firstIP, _ := NetworkRange(netY); netX.Contains(firstIP) {
		return true
	}
	return false
}

// Calculates the first and last IP addresses in an IPNet
func NetworkRange(network *net.IPNet) (net.IP, net.IP) {
	var (
		netIP   = network.IP.To4()
		firstIP = netIP.Mask(network.Mask)
		lastIP  = net.IPv4(0, 0, 0, 0).To4()
	)

	for i := 0; i < len(lastIP); i++ {
		lastIP[i] = netIP[i] | ^network.Mask[i]
	}
	return firstIP, lastIP
}

// Given a netmask, calculates the number of available hosts
func NetworkSize(mask net.IPMask) int32 {
	m := net.IPv4Mask(0, 0, 0, 0)
	for i := 0; i < net.IPv4len; i++ {
		m[i] = ^mask[i]
	}
	return int32(binary.BigEndian.Uint32(m)) + 1
}
